package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"proximaLectio/internal/constants"
	"proximaLectio/internal/crypto"
	"proximaLectio/internal/database/models/untis"
	api "proximaLectio/internal/untis"
	"proximaLectio/internal/utils"
	"strconv"
	"strings"
	"time"
)

type SyncService struct {
	db        *sql.DB
	encryptor *crypto.Encryptor
	userSvc   *UserService
	schoolSvc *SchoolService
	hooks     []NotificationHandler
}

func NewSyncService(db *sql.DB, encryptor *crypto.Encryptor, userSvc *UserService, schoolSvc *SchoolService) *SyncService {
	return &SyncService{
		db:        db,
		encryptor: encryptor,
		userSvc:   userSvc,
		schoolSvc: schoolSvc,
	}
}

func (s *SyncService) RegisterNotificationHook(handler NotificationHandler) {
	s.hooks = append(s.hooks, handler)
}

func (s *SyncService) notifyHooks(ctx context.Context, userID string, target untis.NotificationTarget, subject, date, cType, old, new string) {
	for _, hook := range s.hooks {
		hook(ctx, userID, target, subject, date, cType, old, new)
	}
}

func (s *SyncService) Sync(ctx context.Context, id string) bool {
	start := utils.FloorToDay(time.Now().AddDate(0, 0, -2))
	end := utils.EndOfDay(time.Now().AddDate(0, 0, 7))

	err := s.SyncUserTimetable(ctx, id, start, end)
	if IsAuthError(err) {
		slog.Warn("Auth failure for user, skipping further sync tasks", "user", id)
		return false
	}

	_ = s.SyncUserAbsences(ctx, id)
	_ = s.SyncUserExams(ctx, id)
	_ = s.SyncUserHomeworks(ctx, id)
	return true
}

func (s *SyncService) SyncUserTimetable(ctx context.Context, id string, start, end time.Time) error {
	user, err := s.userSvc.GetUser(ctx, id)
	if err != nil {
		return err
	}

	timetable, err := s.GetTimetable(ctx, id, start, end)
	if err != nil {
		return err
	}

	for _, day := range timetable.Days {
		entryDate, err := time.Parse("2006-01-02", day.Date)
		if err != nil {
			slog.Warn("Failed to parse timetable date", "date", day.Date, "error", err)
			continue
		}
		for _, slot := range day.GridEntries {
			sH, sM := parseTime(slot.Duration.Start)
			eH, eM := parseTime(slot.Duration.End)
			startTime := fmt.Sprintf("%02d:%02d:00", sH, sM)
			endTime := fmt.Sprintf("%02d:%02d:00", eH, eM)

			var subject, teacher, room string
			allPositions := [][]api.Position{slot.Position1, slot.Position2, slot.Position3, slot.Position4}
			for _, list := range allPositions {
				for _, pos := range list {
					if pos.Current != nil {
						switch pos.Current.Type {
						case "SUBJECT":
							subject = pos.Current.ShortName
						case "TEACHER":
							teacher = pos.Current.ShortName
						case "ROOM":
							room = pos.Current.ShortName
						}
					}
				}
			}

			status := slot.Status
			if status == "CHANGED" {
				status = "SUBSTITUTION"
			}

			if user.NotificationsEnabled {
				var oldStatus, oldTeacher, oldRoom string
				checkQ := `SELECT status, teacher, room FROM timetable_entries WHERE user_id = $1 AND entry_date = $2 AND start_time = $3 AND subject = $4`
				err := s.db.QueryRowContext(ctx, checkQ, id, entryDate, startTime, subject).Scan(&oldStatus, &oldTeacher, &oldRoom)
				target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}

				if err == nil {
					if oldStatus != status {
						s.notifyHooks(ctx, id, target, subject, day.Date, "STATUS", oldStatus, status)
					}
					if teacher != "" && oldTeacher != "" && oldTeacher != teacher {
						s.notifyHooks(ctx, id, target, subject, day.Date, "TEACHER", oldTeacher, teacher)
					}
					if room != "" && oldRoom != "" && oldRoom != room {
						s.notifyHooks(ctx, id, target, subject, day.Date, "ROOM", oldRoom, room)
					}
				} else if errors.Is(err, sql.ErrNoRows) && status == "CANCELLED" {
					s.notifyHooks(ctx, id, target, subject, day.Date, "STATUS", "REGULAR", status)
				}
			}

			upsert := `
			INSERT INTO timetable_entries (user_id, entry_date, start_time, end_time, subject, teacher, room, status, substitution_text)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id, entry_date, start_time, subject) DO UPDATE SET
				teacher = EXCLUDED.teacher, room = EXCLUDED.room, status = EXCLUDED.status, 
				substitution_text = EXCLUDED.substitution_text, last_synced = NOW()`
			if _, err := s.db.ExecContext(ctx, upsert, id, entryDate, startTime, endTime, subject, teacher, room, status, slot.SubstitutionText); err != nil {
				slog.Error("Failed to upsert timetable entry", "userID", id, "subject", subject, "date", entryDate, "error", err)
			}
		}
	}
	return nil
}

func (s *SyncService) SyncUserAbsences(ctx context.Context, discordUserID string) error {
	user, err := s.userSvc.GetUser(ctx, discordUserID)
	if err != nil {
		return err
	}

	school, err := s.schoolSvc.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err != nil {
		return err
	}

	client, err := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err != nil {
		return fmt.Errorf("failed to create untis client: %w", err)
	}
	if err := client.Authenticate(ctx); err != nil {
		if IsAuthError(err) {
			s.notifyHooks(ctx, user.ID, untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}, "System", "", "AUTH_FAILURE", "", "")
		}
		return err
	}

	years, err := client.GetSchoolYears(ctx)
	if err != nil {
		slog.Warn("Failed to get school years", "userID", discordUserID, "error", err)
	}
	now := time.Now()
	syncStart, syncEnd := now.AddDate(0, 0, -30), now.AddDate(0, 0, 1)

	for _, y := range years {
		sDate, err := time.Parse("2006-01-02", y.DateRange.Start)
		if err != nil {
			continue
		}
		eDate, err := time.Parse("2006-01-02", y.DateRange.End)
		if err != nil {
			continue
		}
		if now.After(sDate) && now.Before(eDate.AddDate(0, 0, 1)) {
			syncStart, syncEnd = sDate, eDate
			break
		}
	}

	absences, err := client.GetAbsences(ctx, syncStart, syncEnd)
	if err != nil {
		return err
	}

	isInitialSync := user.AbsencesSyncedAt == nil

	for _, a := range absences {
		startDate := parseUntisDateTime(a.StartDate, 0)
		endDate := parseUntisDateTime(a.EndDate, 0)
		target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}

		newStatus := strings.ToUpper(strings.TrimSpace(a.ExcuseStatus))
		newReason := strings.TrimSpace(a.Reason)

		if user.NotificationsEnabled {
			var oldReason, oldStatus string
			err := s.db.QueryRowContext(ctx, `SELECT reason, status FROM absences WHERE user_id = $1 AND untis_id = $2`, discordUserID, a.ID).Scan(&oldReason, &oldStatus)

			oldStatusNormalized := strings.ToUpper(strings.TrimSpace(oldStatus))
			oldReasonNormalized := strings.TrimSpace(oldReason)

			if err != nil && !isInitialSync {
				s.notifyHooks(ctx, discordUserID, target, "Absence", startDate.Format("02.01.2006"), "ABSENCE_NEW", "", a.Reason)
			} else if err == nil {
				if oldStatusNormalized != newStatus {
					s.notifyHooks(ctx, discordUserID, target, "Absence", startDate.Format("02.01.2006"), "ABSENCE_EXCUSED", oldStatus, a.ExcuseStatus)
				}
				if oldReasonNormalized != newReason {
					s.notifyHooks(ctx, discordUserID, target, "Absence", startDate.Format("02.01.2006"), "ABSENCE_REASON", oldReason, a.Reason)
				}
			}
		}

		query := `
			INSERT INTO absences (user_id, untis_id, start_date, end_date, start_time, end_time, reason, status, is_excused)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id, untis_id) DO UPDATE SET
				reason = EXCLUDED.reason, status = EXCLUDED.status, is_excused = EXCLUDED.is_excused,
				start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time`
		if _, err := s.db.ExecContext(ctx, query, discordUserID, a.ID, startDate, endDate, a.StartTime, a.EndTime, a.Reason, a.ExcuseStatus, a.IsExcused); err != nil {
			slog.Error("Failed to upsert absence", "userID", discordUserID, "absenceID", a.ID, "error", err)
		}
	}

	if err := s.userSvc.MarkAbsencesSynced(ctx, discordUserID); err != nil {
		slog.Warn("Failed to mark absences as synced", "userID", discordUserID, "error", err)
	}

	return nil
}

func (s *SyncService) SyncUserExams(ctx context.Context, discordUserID string) error {
	user, err := s.userSvc.GetUser(ctx, discordUserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	school, err := s.schoolSvc.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err != nil {
		return fmt.Errorf("failed to get school: %w", err)
	}
	client, err := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err != nil {
		return fmt.Errorf("failed to create untis client: %w", err)
	}

	if err := client.Authenticate(ctx); err != nil {
		if IsAuthError(err) {
			s.notifyHooks(ctx, user.ID, untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}, "System", "", "AUTH_FAILURE", "", "")
		}
		return err
	}

	timetable, err := client.GetMyTimetable(ctx, time.Now().AddDate(0, 0, -7), time.Now().AddDate(0, 0, 90))
	if err != nil {
		return err
	}

	for _, day := range timetable.Days {
		entryDate, err := time.Parse("2006-01-02", day.Date)
		if err != nil {
			continue
		}
		for _, slot := range day.GridEntries {
			if slot.Status != "EXAM" {
				continue
			}

			var subject string
			for _, pos := range append(slot.Position1, slot.Position2...) {
				if pos.Current != nil && pos.Current.Type == "SUBJECT" {
					subject = pos.Current.ShortName
				}
			}

			untisID := 0
			if len(slot.IDs) > 0 {
				untisID = slot.IDs[0]
			}

			sH, sM := parseTime(slot.Duration.Start)
			eH, eM := parseTime(slot.Duration.End)
			startTime := fmt.Sprintf("%02d:%02d:00", sH, sM)
			endTime := fmt.Sprintf("%02d:%02d:00", eH, eM)

			if user.NotificationsEnabled {
				var exists bool
				if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exams WHERE user_id = $1 AND untis_id = $2)`, discordUserID, untisID).Scan(&exists); err != nil {
					slog.Warn("Failed to check exam existence", "userID", discordUserID, "examID", untisID, "error", err)
				}
				if !exists {
					target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}
					s.notifyHooks(ctx, discordUserID, target, subject, entryDate.Format("02.01.2006"), "EXAM_NEW", "", "Exam")
				}
			}

			query := `
				INSERT INTO exams (user_id, untis_id, exam_date, start_time, end_time, subject, name, last_updated)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
				ON CONFLICT (user_id, untis_id) DO UPDATE SET
					exam_date = EXCLUDED.exam_date, start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time,
					subject = EXCLUDED.subject, name = EXCLUDED.name`
			if _, err := s.db.ExecContext(ctx, query, discordUserID, untisID, entryDate, startTime, endTime, subject, "Exam"); err != nil {
				slog.Error("Failed to upsert exam", "userID", discordUserID, "examID", untisID, "error", err)
			}
		}
	}
	return nil
}

func (s *SyncService) SyncUserHomeworks(ctx context.Context, discordUserID string) error {
	user, err := s.userSvc.GetUser(ctx, discordUserID)
	if err != nil {
		return err
	}

	school, err := s.schoolSvc.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err != nil {
		return err
	}

	client, err := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err != nil {
		return fmt.Errorf("failed to create untis client: %w", err)
	}
	if err := client.Authenticate(ctx); err != nil {
		if IsAuthError(err) {
			s.notifyHooks(ctx, user.ID, untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}, "System", "", "AUTH_FAILURE", "", "")
		}
		return err
	}

	now := time.Now()
	resp, err := client.GetHomeworks(ctx, now, now.AddDate(0, 0, 14))
	if err != nil {
		return err
	}

	subjects := make(map[int]string)
	for _, l := range resp.Data.Lessons {
		subjects[l.ID] = l.Subject
	}

	for _, hw := range resp.Data.Homeworks {
		subject := subjects[hw.LessonID]

		if user.NotificationsEnabled {
			var exists bool
			if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM homeworks WHERE user_id = $1 AND untis_id = $2)`, discordUserID, hw.ID).Scan(&exists); err != nil {
				slog.Warn("Failed to check homework existence", "userID", discordUserID, "homeworkID", hw.ID, "error", err)
			}
			if !exists {
				target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}
				dueDate := parseUntisDateTime(hw.DueDate, 0)
				s.notifyHooks(ctx, discordUserID, target, subject, dueDate.Format("02.01.2006"), "HOMEWORK_NEW", "", hw.Text)
			}
		}

		dueDate := parseUntisDateTime(hw.DueDate, 0)
		query := `
			INSERT INTO homeworks (user_id, untis_id, subject, text, due_date, completed)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, untis_id) DO UPDATE SET
				subject = EXCLUDED.subject, text = EXCLUDED.text, due_date = EXCLUDED.due_date, completed = EXCLUDED.completed`
		if _, err := s.db.ExecContext(ctx, query, discordUserID, hw.ID, subject, hw.Text, dueDate, hw.Completed); err != nil {
			slog.Error("Failed to upsert homework", "userID", discordUserID, "homeworkID", hw.ID, "error", err)
		}
	}
	return nil
}

func (s *SyncService) GetTimetable(ctx context.Context, userID string, start, end time.Time) (*api.TimetableEntry, error) {
	user, err := s.userSvc.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	school, err := s.schoolSvc.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err != nil {
		return nil, fmt.Errorf("failed to get school: %w", err)
	}
	client, err := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err != nil {
		return nil, fmt.Errorf("failed to create untis client: %w", err)
	}
	if err := client.Authenticate(ctx); err != nil {
		if IsAuthError(err) {
			s.notifyHooks(ctx, user.ID, untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}, "System", "", "AUTH_FAILURE", "", "")
		}
		return nil, err
	}
	return client.GetMyTimetable(ctx, start, end)
}

func (s *SyncService) GetUserAbsences(ctx context.Context, userID string, filter int) ([]untis.AbsenceRecord, error) {
	query := `SELECT untis_id, start_date, end_date, reason, status, is_excused FROM absences WHERE user_id = $1`
	if filter == 1 {
		query += " AND is_excused = FALSE"
	} else if filter == 2 {
		query += " AND is_excused = TRUE"
	}
	rows, err := s.db.QueryContext(ctx, query+fmt.Sprintf(" ORDER BY start_date DESC LIMIT %d", constants.MaxAbsencesDisplay), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []untis.AbsenceRecord
	for rows.Next() {
		var r untis.AbsenceRecord
		if err := rows.Scan(&r.UntisID, &r.StartDate, &r.EndDate, &r.Reason, &r.Status, &r.IsExcused); err == nil {
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *SyncService) SearchAbsencesForAutocomplete(ctx context.Context, userID string, query string) ([]untis.AbsenceRecord, error) {
	sqlQuery := `
		SELECT untis_id, start_date, end_date, reason, is_excused 
		FROM absences 
		WHERE user_id = $1 AND (reason ILIKE $2 OR $2 = '')
		ORDER BY start_date DESC 
		LIMIT ` + strconv.Itoa(constants.MaxAutocompleteResults)

	rows, err := s.db.QueryContext(ctx, sqlQuery, userID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []untis.AbsenceRecord
	for rows.Next() {
		var r untis.AbsenceRecord
		if err := rows.Scan(&r.UntisID, &r.StartDate, &r.EndDate, &r.Reason, &r.IsExcused); err == nil {
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *SyncService) GetUpcomingExams(ctx context.Context, userID string) ([]untis.ExamRecord, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT untis_id, exam_date, start_time, end_time, subject, name FROM exams WHERE user_id = $1 AND exam_date >= CURRENT_DATE ORDER BY exam_date ASC LIMIT %d`, constants.MaxExamsDisplay), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []untis.ExamRecord
	for rows.Next() {
		var r untis.ExamRecord
		var st, et string
		if err := rows.Scan(&r.UntisID, &r.Date, &st, &et, &r.Subject, &r.Name); err == nil {
			r.StartTime, r.EndTime = st[:5], et[:5]
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *SyncService) GetUserHomeworks(ctx context.Context, userID string, filter int) ([]untis.HomeworkRecord, error) {
	query := `SELECT untis_id, subject, text, due_date, completed FROM homeworks WHERE user_id = $1`
	if filter == 1 {
		query += " AND completed = FALSE"
	} else if filter == 2 {
		query += " AND completed = TRUE"
	}
	rows, err := s.db.QueryContext(ctx, query+fmt.Sprintf(" ORDER BY due_date ASC LIMIT %d", constants.MaxAbsencesDisplay), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []untis.HomeworkRecord
	for rows.Next() {
		var r untis.HomeworkRecord
		if err := rows.Scan(&r.UntisID, &r.Subject, &r.Text, &r.DueDate, &r.Completed); err == nil {
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *SyncService) GetUserStats(ctx context.Context, userID string) (*untis.UserStats, error) {
	stats := &untis.UserStats{}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'CANCELLED'), COUNT(*) FILTER (WHERE status = 'SUBSTITUTION') FROM timetable_entries WHERE user_id = $1`, userID).Scan(&stats.TotalLessons, &stats.CancelledCount, &stats.SubstitutionCount); err != nil {
		slog.Warn("Failed to get timetable stats", "userID", userID, "error", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT room FROM timetable_entries WHERE user_id = $1 AND room != '' GROUP BY room ORDER BY COUNT(*) DESC LIMIT 1`, userID).Scan(&stats.MostVisitedRoom); err != nil && err != sql.ErrNoRows {
		slog.Warn("Failed to get most visited room", "userID", userID, "error", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE is_excused = FALSE) FROM absences WHERE user_id = $1`, userID).Scan(&stats.TotalAbsences, &stats.UnexcusedAbsences); err != nil {
		slog.Warn("Failed to get absence stats", "userID", userID, "error", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exams WHERE user_id = $1 AND exam_date >= CURRENT_DATE`, userID).Scan(&stats.UpcomingExams); err != nil {
		slog.Warn("Failed to get upcoming exams count", "userID", userID, "error", err)
	}
	return stats, nil
}

func (s *SyncService) GetGuildMemberStatusesAt(ctx context.Context, guildID string, targetTime time.Time) ([]untis.UserScheduleStatus, error) {
	query := `
		SELECT u.id, u.username, COALESCE(te.subject, '') as subject, COALESCE(te.room, '') as room, COALESCE(te.status, '') as status
		FROM users u JOIN guild_members gm ON u.id = gm.user_id
		LEFT JOIN timetable_entries te ON u.id = te.user_id AND te.entry_date = $2 AND $3::TIME BETWEEN te.start_time AND te.end_time
		WHERE gm.guild_id = $1 ORDER BY u.username ASC`
	rows, err := s.db.QueryContext(ctx, query, guildID, targetTime.Format("2006-01-02"), targetTime.Format("15:04:05"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []untis.UserScheduleStatus
	for rows.Next() {
		var st untis.UserScheduleStatus
		if err := rows.Scan(&st.UserID, &st.Username, &st.Subject, &st.Room, &st.Status); err == nil {
			st.IsFree = st.Subject == "" || st.Status == "CANCELLED"
			results = append(results, st)
		}
	}
	return results, nil
}

func (s *SyncService) GetNextRoomForSubject(ctx context.Context, userID string, subjectQuery string) (*untis.RoomResult, error) {
	query := `SELECT subject, room, teacher, entry_date, start_time, (CURRENT_TIME BETWEEN start_time AND end_time) as is_now
	          FROM timetable_entries WHERE user_id = $1 AND (subject ILIKE $2 OR subject ILIKE $3)
	          AND (entry_date > CURRENT_DATE OR (entry_date = CURRENT_DATE AND end_time >= CURRENT_TIME))
	          ORDER BY entry_date, start_time LIMIT 1`
	var res untis.RoomResult
	var ts string
	if err := s.db.QueryRowContext(ctx, query, userID, subjectQuery, "%"+subjectQuery+"%").Scan(&res.Subject, &res.Room, &res.Teacher, &res.StartTime, &ts, &res.IsNow); err != nil {
		return nil, nil
	}
	h, m := parseTime(ts)
	res.StartTime = time.Date(res.StartTime.Year(), res.StartTime.Month(), res.StartTime.Day(), h, m, 0, 0, time.Local)
	res.IsToday = res.StartTime.YearDay() == time.Now().YearDay()
	return &res, nil
}

func (s *SyncService) GetUniqueSubjects(ctx context.Context, userID string, filter string) ([]string, error) {
	rows, _ := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT DISTINCT subject FROM timetable_entries WHERE user_id = $1 AND subject ILIKE $2 ORDER BY subject LIMIT %d`, constants.MaxAutocompleteResults), userID, "%"+filter+"%")
	defer rows.Close()
	var subs []string
	for rows.Next() {
		var sub string
		if err := rows.Scan(&sub); err == nil {
			subs = append(subs, sub)
		}
	}
	return subs, nil
}

func (s *SyncService) CheckUserHomeworkAlerts(ctx context.Context, discordUserID string) error {
	user, err := s.userSvc.GetUser(ctx, discordUserID)
	if err != nil {
		return err
	}
	school, err := s.schoolSvc.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err != nil {
		return err
	}

	client, _ := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err := client.Authenticate(ctx); err != nil {
		return err
	}

	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	tomorrowInt, _ := strconv.Atoi(tomorrow.Format("20060102"))

	resp, err := client.GetHomeworks(ctx, now, now.AddDate(0, 0, 14))
	if err != nil {
		return err
	}

	subjects := make(map[int]string)
	for _, l := range resp.Data.Lessons {
		subjects[l.ID] = l.Subject
	}

	for _, hw := range resp.Data.Homeworks {
		if hw.DueDate == tomorrowInt && !hw.Completed {
			subject := subjects[hw.LessonID]
			target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}

			s.notifyHooks(ctx, discordUserID, target, subject, tomorrow.Format("02.01.2006"), "HOMEWORK_DUE", "", hw.Text)
		}
	}

	return nil
}

func (s *SyncService) LoginUser(ctx context.Context, school *untis.School, username, password, discordID, discordUsername string) (*untis.User, error) {
	baseURL := fmt.Sprintf("https://%s/WebUntis", school.Server)
	client, err := api.NewClient(school.LoginName, username, password, baseURL)
	if err != nil {
		return nil, err
	}

	if err := client.Authenticate(ctx); err != nil {
		return nil, fmt.Errorf("untis authentication failed: %w", err)
	}

	appData, err := client.GetAppData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app data: %w", err)
	}

	tenantID, _ := strconv.Atoi(school.TenantId)
	if err := s.schoolSvc.UpsertSchool(ctx, tenantID, school.SchoolId, school.LoginName, school.DisplayName, school.Server, school.Address); err != nil {
		slog.Warn("Failed to upsert school during login", "error", err)
	}

	encryptedPassword, err := s.encryptor.Encrypt(password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	query := `
		INSERT INTO users (id, username, display_name, email, untis_school_tenant_id, untis_user, untis_password, untis_person_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			email = EXCLUDED.email,
			untis_school_tenant_id = EXCLUDED.untis_school_tenant_id,
			untis_user = EXCLUDED.untis_user,
			untis_password = EXCLUDED.untis_password,
			untis_person_id = EXCLUDED.untis_person_id
		RETURNING id, username, display_name, email, untis_school_tenant_id, untis_user, untis_person_id`

	var u untis.User
	err = s.db.QueryRowContext(ctx, query,
		discordID, discordUsername, appData.User.Person.DisplayName, appData.User.Email,
		tenantID, username, encryptedPassword, appData.User.Person.ID,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPersonID)

	u.UntisPassword = password
	return &u, err
}

func (s *SyncService) GenerateExcusePDF(ctx context.Context, userID string, untisID int, guardian string) (io.Reader, error) {
	user, err := s.userSvc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var start, end time.Time
	var startTimeInt, endTimeInt int
	var reason string
	query := `SELECT start_date, end_date, start_time, end_time, reason FROM absences WHERE user_id = $1 AND untis_id = $2`
	err = s.db.QueryRowContext(ctx, query, userID, untisID).Scan(&start, &end, &startTimeInt, &endTimeInt, &reason)
	if err != nil {
		return nil, fmt.Errorf("absence not found in database: %w", err)
	}

	dateRange := start.Format("02.01.2006")
	if !start.Equal(end) {
		dateRange = fmt.Sprintf("%s bis %s", dateRange, end.Format("02.01.2006"))
	}

	startTimeStr := fmt.Sprintf("%02d:%02d", startTimeInt/100, startTimeInt%100)
	endTimeStr := fmt.Sprintf("%02d:%02d", endTimeInt/100, endTimeInt%100)

	city := "N/A"
	school, err := s.schoolSvc.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err == nil && school != nil && school.Address != "" {
		city = school.Address
	}

	data := ExcuseData{
		StudentName:    formatUntisName(user.DisplayName),
		StudentID:      user.UntisPersonID,
		DateRange:      dateRange,
		StartTime:      startTimeStr,
		EndTime:        endTimeStr,
		Reason:         reason,
		City:           city,
		SubmissionDate: time.Now().Format("02.01.2006"),
		ReferenceID:    fmt.Sprintf("%d-ABS-%d", user.UntisPersonID, untisID),
		Guardian:       guardian,
	}

	renderer := NewPDFRenderer()
	return renderer.RenderExcuse(data)
}

func (s *SyncService) mapTimetableToItems(timetable *api.TimetableEntry, theme Theme) []RenderItem {
	var items []RenderItem
	for i, day := range timetable.Days {
		for _, slot := range day.GridEntries {
			sH, sM := parseTime(slot.Duration.Start)
			eH, eM := parseTime(slot.Duration.End)

			status := slot.Status
			if status == "CHANGED" {
				status = "SUBSTITUTION"
			}

			item := RenderItem{
				DayIndex:         i,
				StartH:           sH,
				StartM:           sM,
				EndH:             eH,
				EndM:             eM,
				Color:            theme.RegularBg,
				TextColor:        theme.RegularText,
				Status:           status,
				SubstitutionText: slot.SubstitutionText,
			}

			switch status {
			case "SUBSTITUTION":
				item.Color, item.TextColor = theme.SubstitutionBg, theme.SubstitutionText
			case "CANCELLED":
				item.Color, item.TextColor = theme.CancelledBg, theme.CancelledText
			case "EXAM":
				item.Color, item.TextColor = theme.ExamBg, theme.ExamText
			}

			for _, pos := range append(slot.Position1, slot.Position2...) {
				if pos.Current != nil {
					switch pos.Current.Type {
					case "SUBJECT":
						item.Title = pos.Current.ShortName
					case "ROOM":
						item.Footer = pos.Current.ShortName
					case "TEACHER":
						item.Subtitle = pos.Current.ShortName
					}
				}
			}
			items = append(items, item)
		}
	}
	return items
}
