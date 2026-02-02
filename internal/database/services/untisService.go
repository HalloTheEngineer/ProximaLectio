package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"proximaLectio/internal/database/models/untis"
	api "proximaLectio/internal/untis"
	"proximaLectio/internal/utils"
	"strconv"
	"strings"
	"time"
)

const (
	SchoolSearchURL = "https://schoolsearch.webuntis.com/schoolquery2"

	AssetFolder = "assets"
	ThemeFolder = AssetFolder + string(os.PathSeparator) + "themes"
	FontFolder  = AssetFolder + string(os.PathSeparator) + "fonts"
)

type NotificationHandler func(ctx context.Context, userID string, target untis.NotificationTarget, subject string, date string, changeType string, oldVal, newVal string)
type UntisService struct {
	db                  *sql.DB
	onStatusChangeHooks []NotificationHandler
}

func NewUntisService(db *sql.DB) *UntisService {
	return &UntisService{db: db}
}

func (s *UntisService) RegisterNotificationHook(handler NotificationHandler) {
	s.onStatusChangeHooks = append(s.onStatusChangeHooks, handler)
}

func (s *UntisService) notifyHooks(ctx context.Context, userID string, target untis.NotificationTarget, subject, date, cType, old, new string) {
	for _, hook := range s.onStatusChangeHooks {
		hook(ctx, userID, target, subject, date, cType, old, new)
	}
}

// --- GUILD SETTINGS ---

func (s *UntisService) AllowChannel(ctx context.Context, guildID, channelID string) error {
	query := `INSERT INTO allowed_notification_channels (guild_id, channel_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, guildID, channelID)
	return err
}

func (s *UntisService) RevokeChannel(ctx context.Context, guildID, channelID string) error {
	query := `DELETE FROM allowed_notification_channels WHERE guild_id = $1 AND channel_id = $2`
	_, err := s.db.ExecContext(ctx, query, guildID, channelID)
	return err
}

func (s *UntisService) IsChannelAllowed(ctx context.Context, guildID, channelID string) (bool, error) {
	var allowed bool
	query := `SELECT EXISTS(SELECT 1 FROM allowed_notification_channels WHERE guild_id = $1 AND channel_id = $2)`
	err := s.db.QueryRowContext(ctx, query, guildID, channelID).Scan(&allowed)
	return allowed, err
}

// --- SCHOOL MANAGEMENT ---

func (s *UntisService) UpsertSchool(ctx context.Context, tenantID, schoolID int, loginName, displayName, server, address string) error {
	query := `
       INSERT INTO schools (tenant_id, school_id, display_name, login_name, server, address, last_updated)
       VALUES ($1, $2, $3, $4, $5, $6, NOW())
       ON CONFLICT (tenant_id) DO UPDATE SET
          display_name = EXCLUDED.display_name,
          login_name = EXCLUDED.login_name,
          server = EXCLUDED.server,
          address = EXCLUDED.address,
          last_updated = NOW()`
	_, err := s.db.ExecContext(ctx, query, tenantID, schoolID, displayName, loginName, server, address)
	return err
}

func (s *UntisService) GetSchool(ctx context.Context, tenantID string) (*untis.School, error) {
	var school untis.School
	query := `SELECT tenant_id, school_id, display_name, login_name, server, address, last_updated 
              FROM schools WHERE tenant_id = $1`
	err := s.db.QueryRowContext(ctx, query, tenantID).Scan(
		&school.TenantId, &school.SchoolId, &school.DisplayName, &school.LoginName, &school.Server, &school.Address, &school.LastUpdated,
	)
	if err != nil {
		return nil, err
	}
	return &school, nil
}

func (s *UntisService) SearchSchools(ctx context.Context, query string, tenantID string) ([]untis.School, error) {
	params := map[string]string{}
	if query != "" {
		params["search"] = query
	}
	if tenantID != "" {
		params["tenantid"] = tenantID
	}

	payload := map[string]interface{}{
		"id":      "1",
		"jsonrpc": "2.0",
		"method":  "searchSchool",
		"params":  []interface{}{params},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", SchoolSearchURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result struct {
			Schools []untis.School `json:"schools"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	return rpcResp.Result.Schools, nil
}

// --- USER MANAGEMENT ---

func (s *UntisService) GetUser(ctx context.Context, id string) (*untis.User, error) {
	var u untis.User
	var target, address sql.NullString
	query := `SELECT id, username, display_name, email, untis_school_tenant_id, untis_user, untis_password, untis_person_id, theme_id, 
                     notifications_enabled, notification_target, notification_address 
              FROM users WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPassword, &u.UntisPersonID, &u.ThemeID,
		&u.NotificationsEnabled, &target, &address,
	)
	if err != nil {
		return nil, err
	}
	u.NotificationTarget = target.String
	u.NotificationAddress = address.String
	return &u, nil
}

func (s *UntisService) GetAllUsers(ctx context.Context) ([]*untis.User, error) {
	query := `SELECT id, username, display_name, email, untis_school_tenant_id, untis_user, untis_password, untis_person_id, theme_id, 
                     notifications_enabled, notification_target, notification_address FROM users`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*untis.User
	for rows.Next() {
		var u untis.User
		var target, address sql.NullString
		err := rows.Scan(
			&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPassword, &u.UntisPersonID, &u.ThemeID,
			&u.NotificationsEnabled, &target, &address,
		)
		if err != nil {
			continue
		}
		u.NotificationTarget = target.String
		u.NotificationAddress = address.String
		users = append(users, &u)
	}
	return users, nil
}

func (s *UntisService) LoginUser(ctx context.Context, school *untis.School, username, password, discordID, discordUsername string) (*untis.User, error) {
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
	_ = s.UpsertSchool(ctx, tenantID, school.SchoolId, school.LoginName, school.DisplayName, school.Server, school.Address)

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
		tenantID, username, password, appData.User.Person.ID,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPersonID)

	return &u, err
}

func (s *UntisService) LogoutUser(ctx context.Context, id string) bool {
	res, _ := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	aff, _ := res.RowsAffected()
	return aff > 0
}

func (s *UntisService) UserExists(ctx context.Context, id string) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (s *UntisService) SetNotificationConfig(ctx context.Context, id string, enabled bool, target string, address string) error {
	query := `UPDATE users SET notifications_enabled = $1, notification_target = $2, notification_address = $3 WHERE id = $4`
	_, err := s.db.ExecContext(ctx, query, enabled, target, address, id)
	return err
}

func (s *UntisService) ToggleNotifications(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE users SET notifications_enabled = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, enabled, id)
	return err
}

func (s *UntisService) CheckUserHomeworkAlerts(ctx context.Context, discordUserID string) error {
	user, err := s.GetUser(ctx, discordUserID)
	if err != nil {
		return err
	}
	school, err := s.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
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

// -- SYNC ---

func (s *UntisService) Sync(ctx context.Context, id string) bool {
	start := utils.FloorToDay(time.Now().AddDate(0, 0, -2))
	end := utils.EndOfDay(time.Now().AddDate(0, 0, 7))

	err := s.SyncUserTimetable(ctx, id, start, end)
	if IsAuthError(err) {
		slog.Warn("Auth failure for user, skipping further sync tasks", "user", id)
		return false
	}

	_ = s.SyncUserAbsences(ctx, id)
	_ = s.SyncUserExams(ctx, id)
	return true
}

// --- SYNC: ABSENCES ---

func (s *UntisService) SyncUserAbsences(ctx context.Context, discordUserID string) error {
	user, err := s.GetUser(ctx, discordUserID)
	if err != nil {
		return err
	}

	school, err := s.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	if err != nil {
		return err
	}

	client, _ := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err := client.Authenticate(ctx); err != nil {
		if IsAuthError(err) {
			s.notifyHooks(ctx, user.ID, untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}, "System", "", "AUTH_FAILURE", "", "")
		}
		return err
	}

	years, _ := client.GetSchoolYears(ctx)
	now := time.Now()
	syncStart, syncEnd := now.AddDate(0, 0, -30), now.AddDate(0, 0, 1)

	for _, y := range years {
		sDate, _ := time.Parse("2006-01-02", y.DateRange.Start)
		eDate, _ := time.Parse("2006-01-02", y.DateRange.End)
		if now.After(sDate) && now.Before(eDate.AddDate(0, 0, 1)) {
			syncStart, syncEnd = sDate, eDate
			break
		}
	}

	absences, err := client.GetAbsences(ctx, syncStart, syncEnd)
	if err != nil {
		return err
	}

	var existingCount int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM absences WHERE user_id = $1`, discordUserID).Scan(&existingCount)
	isInitialSync := existingCount == 0

	for _, a := range absences {
		startDate := parseUntisDateTime(a.StartDate, 0)
		endDate := parseUntisDateTime(a.EndDate, 0)
		target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}

		if user.NotificationsEnabled {
			var oldReason, oldStatus string
			err := s.db.QueryRowContext(ctx, `SELECT reason, status FROM absences WHERE user_id = $1 AND untis_id = $2`, discordUserID, a.ID).Scan(&oldReason, &oldStatus)

			if err != nil && !isInitialSync {
				s.notifyHooks(ctx, discordUserID, target, "Absence", startDate.Format("02.01.2006"), "ABSENCE_NEW", "", a.Reason)
			} else if err == nil {
				if oldStatus != a.ExcuseStatus {
					s.notifyHooks(ctx, discordUserID, target, "Absence", startDate.Format("02.01.2006"), "ABSENCE_EXCUSED", oldStatus, a.ExcuseStatus)
				}
				if oldReason != a.Reason {
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
		_, _ = s.db.ExecContext(ctx, query, discordUserID, a.ID, startDate, endDate, a.StartTime, a.EndTime, a.Reason, a.ExcuseStatus, a.IsExcused)
	}
	return nil
}

func (s *UntisService) SearchAbsencesForAutocomplete(ctx context.Context, userID string, query string) ([]untis.AbsenceRecord, error) {
	sqlQuery := `
		SELECT untis_id, start_date, end_date, reason, is_excused 
		FROM absences 
		WHERE user_id = $1 AND (reason ILIKE $2 OR $2 = '')
		ORDER BY start_date DESC 
		LIMIT 20`

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

// --- SYNC: EXAMS ---

func (s *UntisService) SyncUserExams(ctx context.Context, discordUserID string) error {
	user, _ := s.GetUser(ctx, discordUserID)
	school, _ := s.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	client, _ := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))

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
		entryDate, _ := time.Parse("2006-01-02", day.Date)
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

			sH, sM := s.parseTime(slot.Duration.Start)
			eH, eM := s.parseTime(slot.Duration.End)
			startTime := fmt.Sprintf("%02d:%02d:00", sH, sM)
			endTime := fmt.Sprintf("%02d:%02d:00", eH, eM)

			if user.NotificationsEnabled {
				var exists bool
				_ = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exams WHERE user_id = $1 AND untis_id = $2)`, discordUserID, untisID).Scan(&exists)
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
			_, _ = s.db.ExecContext(ctx, query, discordUserID, untisID, entryDate, startTime, endTime, subject, "Exam")
		}
	}
	return nil
}

// --- SYNC: TIMETABLE ---

func (s *UntisService) SyncUserTimetable(ctx context.Context, id string, start, end time.Time) error {
	user, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}

	timetable, err := s.GetTimetable(ctx, id, start, end)
	if err != nil {
		return err
	}

	for _, day := range timetable.Days {
		entryDate, _ := time.Parse("2006-01-02", day.Date)
		for _, slot := range day.GridEntries {
			sH, sM := s.parseTime(slot.Duration.Start)
			startTime := fmt.Sprintf("%02d:%02d:00", sH, sM)
			endTime := fmt.Sprintf("%02d:%02d:00", sH, sM)

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

			if user.NotificationsEnabled {
				var oldStatus, oldTeacher, oldRoom string
				checkQ := `SELECT status, teacher, room FROM timetable_entries WHERE user_id = $1 AND entry_date = $2 AND start_time = $3 AND subject = $4`
				if err := s.db.QueryRowContext(ctx, checkQ, id, entryDate, startTime, subject).Scan(&oldStatus, &oldTeacher, &oldRoom); err == nil {
					target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}
					if oldStatus != slot.Status {
						s.notifyHooks(ctx, id, target, subject, day.Date, "STATUS", oldStatus, slot.Status)
					}
					if teacher != "" && oldTeacher != "" && oldTeacher != teacher {
						s.notifyHooks(ctx, id, target, subject, day.Date, "TEACHER", oldTeacher, teacher)
					}
					if room != "" && oldRoom != "" && oldRoom != room {
						s.notifyHooks(ctx, id, target, subject, day.Date, "ROOM", oldRoom, room)
					}
				}
			}

			upsert := `
				INSERT INTO timetable_entries (user_id, entry_date, start_time, end_time, subject, teacher, room, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (user_id, entry_date, start_time, subject) DO UPDATE SET
					teacher = EXCLUDED.teacher, room = EXCLUDED.room, status = EXCLUDED.status, last_synced = NOW()`
			_, _ = s.db.ExecContext(ctx, upsert, id, entryDate, startTime, endTime, subject, teacher, room, slot.Status)
		}
	}
	return nil
}

func (s *UntisService) GetUserAbsences(ctx context.Context, userID string, filter int) ([]untis.AbsenceRecord, error) {
	query := `SELECT untis_id, start_date, end_date, reason, status, is_excused FROM absences WHERE user_id = $1`
	if filter == 1 {
		query += " AND is_excused = FALSE"
	} else if filter == 2 {
		query += " AND is_excused = TRUE"
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY start_date DESC LIMIT 25", userID)
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

// --- USER EXAMS ---

func (s *UntisService) GetUpcomingExams(ctx context.Context, userID string) ([]untis.ExamRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT untis_id, exam_date, start_time, end_time, subject, name FROM exams WHERE user_id = $1 AND exam_date >= CURRENT_DATE ORDER BY exam_date ASC LIMIT 15`, userID)
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

func (s *UntisService) GetTimetable(ctx context.Context, userID string, start, end time.Time) (*api.TimetableEntry, error) {
	user, _ := s.GetUser(ctx, userID)
	school, _ := s.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
	client, _ := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err := client.Authenticate(ctx); err != nil {
		if IsAuthError(err) {
			s.notifyHooks(ctx, user.ID, untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}, "System", "", "AUTH_FAILURE", "", "")
		}
		return nil, err
	}
	return client.GetMyTimetable(ctx, start, end)
}

func (s *UntisService) GetUserStats(ctx context.Context, userID string) (*untis.UserStats, error) {
	stats := &untis.UserStats{}
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'CANCELLED'), COUNT(*) FILTER (WHERE status = 'SUBSTITUTION') FROM timetable_entries WHERE user_id = $1`, userID).Scan(&stats.TotalLessons, &stats.CancelledCount, &stats.SubstitutionCount)
	_ = s.db.QueryRowContext(ctx, `SELECT room FROM timetable_entries WHERE user_id = $1 AND room != '' GROUP BY room ORDER BY COUNT(*) DESC LIMIT 1`, userID).Scan(&stats.MostVisitedRoom)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE is_excused = FALSE) FROM absences WHERE user_id = $1`, userID).Scan(&stats.TotalAbsences, &stats.UnexcusedAbsences)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exams WHERE user_id = $1 AND exam_date >= CURRENT_DATE`, userID).Scan(&stats.UpcomingExams)
	return stats, nil
}

func (s *UntisService) GetGuildMemberStatusesAt(ctx context.Context, guildID string, targetTime time.Time) ([]untis.UserScheduleStatus, error) {
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

func (s *UntisService) GetNextRoomForSubject(ctx context.Context, userID string, subjectQuery string) (*untis.RoomResult, error) {
	query := `SELECT subject, room, teacher, entry_date, start_time, (CURRENT_TIME BETWEEN start_time AND end_time) as is_now
	          FROM timetable_entries WHERE user_id = $1 AND (subject ILIKE $2 OR subject ILIKE $3)
	          AND (entry_date > CURRENT_DATE OR (entry_date = CURRENT_DATE AND end_time >= CURRENT_TIME))
	          ORDER BY entry_date, start_time LIMIT 1`
	var res untis.RoomResult
	var ts string
	if err := s.db.QueryRowContext(ctx, query, userID, subjectQuery, "%"+subjectQuery+"%").Scan(&res.Subject, &res.Room, &res.Teacher, &res.StartTime, &ts, &res.IsNow); err != nil {
		return nil, nil
	}
	h, m := s.parseTime(ts)
	res.StartTime = time.Date(res.StartTime.Year(), res.StartTime.Month(), res.StartTime.Day(), h, m, 0, 0, time.Local)
	res.IsToday = res.StartTime.YearDay() == time.Now().YearDay()
	return &res, nil
}

func (s *UntisService) GetUniqueSubjects(ctx context.Context, userID string, filter string) ([]string, error) {
	rows, _ := s.db.QueryContext(ctx, `SELECT DISTINCT subject FROM timetable_entries WHERE user_id = $1 AND subject ILIKE $2 ORDER BY subject LIMIT 25`, userID, "%"+filter+"%")
	defer rows.Close()
	var subs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

// --- GENERATION ---

func (s *UntisService) GenerateScheduleImage(timetable *api.TimetableEntry, daysCount int, themeID string) (io.Reader, error) {
	config := DefaultRenderConfig()
	config.DaysCount = daysCount
	if themeID != "" && themeID != "default" {
		if t, err := LoadTheme(themeID); err == nil {
			config.Theme = t
		}
	}
	items := s.mapTimetableToItems(timetable, config.Theme)
	renderer := NewCanvasRenderer(config, items)
	return renderer.Draw()
}

func (s *UntisService) GenerateExcusePDF(ctx context.Context, userID string, untisID int, guardian string) (io.Reader, error) {
	user, err := s.GetUser(ctx, userID)
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
	school, err := s.GetSchool(ctx, strconv.Itoa(int(user.UntisSchoolID)))
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

// --- THEMING ---

func (s *UntisService) GetThemes(filter string) ([]*string, error) {
	entries, _ := os.ReadDir(ThemeFolder)
	var l []*string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && strings.Contains(strings.ToLower(e.Name()), strings.ToLower(filter)) {
			str := strings.ToUpper(strings.TrimSuffix(e.Name(), ".json"))
			l = append(l, &str)
		}
	}
	return l, nil
}

func (s *UntisService) GetTheme(themeID string) (*Theme, error) {
	id := strings.ToLower(strings.TrimSpace(themeID))

	if id == "" || id == "default" {
		def := DefaultRenderConfig().Theme
		return &def, nil
	}

	theme, err := LoadTheme(id)
	if err != nil {
		return nil, fmt.Errorf("theme '%s' not found: %w", themeID, err)
	}

	return &theme, nil
}

func (s *UntisService) SetTheme(ctx context.Context, id string, themeID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET theme_id = $1 WHERE id = $2`, themeID, id)
	return err
}

// --- HELPERS ---

func (s *UntisService) mapTimetableToItems(timetable *api.TimetableEntry, theme Theme) []RenderItem {
	var items []RenderItem
	for i, day := range timetable.Days {
		for _, slot := range day.GridEntries {
			sH, sM := s.parseTime(slot.Duration.Start)
			eH, eM := s.parseTime(slot.Duration.End)
			item := RenderItem{DayIndex: i, StartH: sH, StartM: sM, EndH: eH, EndM: eM, Color: theme.RegularBg, TextColor: theme.RegularText, Status: slot.Status}
			switch slot.Status {
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

func (s *UntisService) parseTime(ts string) (int, int) {
	formats := []string{"2006-01-02T15:04", time.RFC3339, "15:04:05", "15:04"}
	for _, f := range formats {
		if t, err := time.Parse(f, ts); err == nil {
			return t.Hour(), t.Minute()
		}
	}
	return 0, 0
}

func (s *UntisService) getClientForUser(ctx context.Context, userID string) (*api.Client, error) {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	school, err := s.GetSchool(ctx, fmt.Sprintf("%d", user.UntisSchoolID))
	if err != nil {
		return nil, err
	}

	client, err := api.NewClient(school.LoginName, user.UntisUser, user.UntisPassword, fmt.Sprintf("https://%s/WebUntis", school.Server))
	if err != nil {
		return nil, err
	}

	if err := client.Authenticate(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

func parseUntisDateTime(dateInt, timeInt int) time.Time {
	t, _ := time.ParseInLocation("20060102 1504", fmt.Sprintf("%d %04d", dateInt, timeInt), time.Local)
	return t
}

func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "auth") ||
		strings.Contains(msg, "login") ||
		strings.Contains(msg, "password") ||
		strings.Contains(msg, "credentials") ||
		strings.Contains(msg, "401")
}
