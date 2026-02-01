package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"proximaLectio/internal/database/models/untis"
	api "proximaLectio/internal/untis"
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

func (s *UntisService) GetSchool(ctx context.Context, tenantId string) (*untis.School, error) {
	var school untis.School
	query := `SELECT tenant_id, school_id, display_name, login_name, server, address, last_updated 
              FROM schools WHERE tenant_id = $1`
	err := s.db.QueryRowContext(ctx, query, tenantId).Scan(
		&school.TenantId,
		&school.SchoolId,
		&school.DisplayName,
		&school.LoginName,
		&school.Server,
		&school.Address,
		&school.LastUpdated,
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

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", SchoolSearchURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
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

func (s *UntisService) UserExists(ctx context.Context, id string) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (s *UntisService) GetUser(ctx context.Context, id string) (*untis.User, error) {
	var u untis.User
	var target sql.NullString
	var address sql.NullString

	query := `SELECT id, username, display_name, email, untis_school_tenant_id, untis_user, untis_person_id, theme_id, 
	                 notifications_enabled, notification_target, notification_address 
              FROM users WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPersonID, &u.ThemeID,
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
	query := `SELECT id, username, display_name, email, untis_school_tenant_id, untis_user, untis_person_id, theme_id, 
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
			&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPersonID, &u.ThemeID,
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

func (s *UntisService) SetNotificationConfig(ctx context.Context, id string, enabled bool, target string, address string) error {
	query := `UPDATE users SET 
                notifications_enabled = $1, 
                notification_target = $2, 
                notification_address = $3 
              WHERE id = $4`
	_, err := s.db.ExecContext(ctx, query, enabled, target, address, id)
	return err
}

func (s *UntisService) ToggleNotifications(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE users SET notifications_enabled = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, enabled, id)
	return err
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
		discordID,
		discordUsername,
		appData.User.Person.DisplayName,
		appData.User.Email,
		tenantID,
		username,
		password,
		appData.User.Person.ID,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &u.UntisPersonID)

	return &u, err
}

func (s *UntisService) LogoutUser(ctx context.Context, id string) bool {
	query := `DELETE FROM users WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return false
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false
	}

	return rowsAffected > 0
}

func (s *UntisService) SyncUserAbsences(ctx context.Context, discordUserID string) error {
	var user, pass, schoolName, serverHost string
	query := `
       SELECT u.untis_user, u.untis_password, s.login_name, s.server 
       FROM users u 
       JOIN schools s ON u.untis_school_tenant_id = s.tenant_id 
       WHERE u.id = $1`

	if err := s.db.QueryRowContext(ctx, query, discordUserID).Scan(&user, &pass, &schoolName, &serverHost); err != nil {
		return err
	}

	client, err := api.NewClient(schoolName, user, pass, fmt.Sprintf("https://%s/WebUntis", serverHost))
	if err != nil {
		return err
	}

	if err := client.Authenticate(ctx); err != nil {
		return err
	}

	years, err := client.GetSchoolYears(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	var syncStart, syncEnd time.Time
	found := false

	for _, y := range years {
		sDate, _ := time.Parse("2006-01-02", y.DateRange.Start)
		eDate, _ := time.Parse("2006-01-02", y.DateRange.End)
		if now.After(sDate) && now.Before(eDate.AddDate(0, 0, 1)) {
			syncStart, syncEnd = sDate, eDate
			found = true
			break
		}
	}

	if !found {
		syncStart = time.Date(now.Year()-1, 8, 1, 0, 0, 0, 0, time.Local)
		syncEnd = time.Date(now.Year()+1, 7, 31, 0, 0, 0, 0, time.Local)
	}

	absences, err := client.GetAbsences(ctx, syncStart, syncEnd)
	if err != nil {
		return err
	}

	dbUser, _ := s.GetUser(ctx, discordUserID)

	var existingCount int
	countQuery := `SELECT COUNT(*) FROM absences WHERE user_id = $1`
	_ = s.db.QueryRowContext(ctx, countQuery, discordUserID).Scan(&existingCount)
	isInitialSync := existingCount == 0

	for _, a := range absences {
		startDate := parseUntisDateTime(a.StartDate, 0)
		endDate := parseUntisDateTime(a.EndDate, 0)

		if dbUser != nil && dbUser.NotificationsEnabled {
			var oldReason string
			var oldExcused bool
			checkQuery := `SELECT reason, is_excused FROM absences WHERE user_id = $1 AND untis_id = $2`
			err := s.db.QueryRowContext(ctx, checkQuery, discordUserID, a.ID).Scan(&oldReason, &oldExcused)

			target := untis.NotificationTarget{Type: dbUser.NotificationTarget, Address: dbUser.NotificationAddress}
			dateStr := startDate.Format("02.01.2006")

			if err != nil {
				if !isInitialSync {
					for _, hook := range s.onStatusChangeHooks {
						hook(ctx, discordUserID, target, "Absence", dateStr, "ABSENCE_NEW", "", a.Reason)
					}
				}
			} else {
				if oldExcused != a.IsExcused {
					newVal := "unentschuldigt"
					if a.IsExcused {
						newVal = "entschuldigt"
					}
					for _, hook := range s.onStatusChangeHooks {
						hook(ctx, discordUserID, target, "Absence", dateStr, "ABSENCE_EXCUSED", "", newVal)
					}
				}
				if oldReason != a.Reason {
					for _, hook := range s.onStatusChangeHooks {
						hook(ctx, discordUserID, target, "Absence", dateStr, "ABSENCE_REASON", oldReason, a.Reason)
					}
				}
			}
		}

		upsert := `
			INSERT INTO absences (user_id, untis_id, start_date, end_date, start_time, end_time, reason, is_excused)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (user_id, untis_id) DO UPDATE SET
				reason = EXCLUDED.reason,
				is_excused = EXCLUDED.is_excused,
				start_time = EXCLUDED.start_time,
				end_time = EXCLUDED.end_time`

		_, _ = s.db.ExecContext(ctx, upsert,
			discordUserID, a.ID, startDate, endDate, a.StartTime, a.EndTime, a.Reason, a.IsExcused,
		)
	}

	return nil
}

func (s *UntisService) GetUserAbsences(ctx context.Context, userID string, filter int) ([]untis.AbsenceRecord, error) {
	sqlQuery := `SELECT untis_id, start_date, end_date, reason, is_excused 
	             FROM absences WHERE user_id = $1`

	if filter == 1 { // unexcused
		sqlQuery += " AND is_excused = FALSE"
	} else if filter == 2 { // excused
		sqlQuery += " AND is_excused = TRUE"
	}

	sqlQuery += " ORDER BY start_date DESC LIMIT 25"

	rows, err := s.db.QueryContext(ctx, sqlQuery, userID)
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
			eH, eM := s.parseTime(slot.Duration.End)

			startTime := fmt.Sprintf("%02d:%02d:00", sH, sM)
			endTime := fmt.Sprintf("%02d:%02d:00", eH, eM)

			var subject, teacher, room string
			allPositions := [][]api.Position{
				slot.Position1, slot.Position2, slot.Position3,
				slot.Position4, slot.Position5, slot.Position6,
				slot.Position7,
			}

			for _, posList := range allPositions {
				for _, pos := range posList {
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
				checkQuery := `SELECT status, teacher, room FROM timetable_entries 
				               WHERE user_id = $1 AND entry_date = $2 AND start_time = $3 AND subject = $4`

				err := s.db.QueryRowContext(ctx, checkQuery, id, entryDate, startTime, subject).Scan(&oldStatus, &oldTeacher, &oldRoom)

				if err == nil {
					target := untis.NotificationTarget{Type: user.NotificationTarget, Address: user.NotificationAddress}

					if oldStatus != slot.Status {
						for _, hook := range s.onStatusChangeHooks {
							hook(ctx, id, target, subject, day.Date, "STATUS", oldStatus, slot.Status)
						}
					}
					if teacher != "" && oldTeacher != "" && oldTeacher != teacher {
						for _, hook := range s.onStatusChangeHooks {
							hook(ctx, id, target, subject, day.Date, "TEACHER", oldTeacher, teacher)
						}
					}
					if room != "" && oldRoom != "" && oldRoom != room {
						for _, hook := range s.onStatusChangeHooks {
							hook(ctx, id, target, subject, day.Date, "ROOM", oldRoom, room)
						}
					}
				}
			}

			query := `
             INSERT INTO timetable_entries (user_id, entry_date, start_time, end_time, subject, teacher, room, status)
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
             ON CONFLICT (user_id, entry_date, start_time, subject) DO UPDATE SET
                end_time = EXCLUDED.end_time,
                teacher = EXCLUDED.teacher,
                room = EXCLUDED.room,
                status = EXCLUDED.status,
                last_synced = CURRENT_TIMESTAMP`

			_, err = s.db.ExecContext(ctx, query, id, entryDate, startTime, endTime, subject, teacher, room, slot.Status)
			if err != nil {
				slog.Error("Failed to upsert timetable entry", "error", err, "user", id, "subject", subject)
				continue
			}
		}
	}
	return nil
}

func (s *UntisService) GetTimetable(ctx context.Context, discordUserID string, start, end time.Time) (*api.TimetableEntry, error) {
	var user, pass, schoolName, serverHost string
	query := `
       SELECT u.untis_user, u.untis_password, s.login_name, s.server 
       FROM users u 
       JOIN schools s ON u.untis_school_tenant_id = s.tenant_id 
       WHERE u.id = $1`

	if err := s.db.QueryRowContext(ctx, query, discordUserID).Scan(&user, &pass, &schoolName, &serverHost); err != nil {
		return nil, err
	}

	client, err := api.NewClient(schoolName, user, pass, fmt.Sprintf("https://%s/WebUntis", serverHost))
	if err != nil {
		return nil, err
	}

	if err := client.Authenticate(ctx); err != nil {
		return nil, err
	}

	return client.GetMyTimetable(ctx, start, end)
}

func (s *UntisService) GetNextRoomForSubject(ctx context.Context, userID string, subjectQuery string) (*untis.RoomResult, error) {
	var query string
	var args []interface{}
	args = append(args, userID)

	if subjectQuery == "" {
		query = `
          SELECT subject, room, teacher, entry_date, start_time, 
                 (CURRENT_TIME BETWEEN start_time AND end_time) as is_now
          FROM timetable_entries
          WHERE user_id = $1 
            AND (
               entry_date > CURRENT_DATE 
               OR (entry_date = CURRENT_DATE AND end_time >= CURRENT_TIME)
            )
          ORDER BY entry_date, start_time
          LIMIT 1`
	} else {
		query = `
          SELECT subject, room, teacher, entry_date, start_time,
                 (CURRENT_TIME BETWEEN start_time AND end_time) as is_now
          FROM timetable_entries
          WHERE user_id = $1 
            AND (subject ILIKE $2 OR subject ILIKE $3)
            AND (
               entry_date > CURRENT_DATE 
               OR (entry_date = CURRENT_DATE AND end_time >= CURRENT_TIME)
            )
          ORDER BY entry_date, start_time
          LIMIT 1`
		args = append(args, subjectQuery, "%"+subjectQuery+"%")
	}

	var subject, room, teacher string
	var entryDate time.Time
	var startTimeStr string
	var isNow bool

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&subject, &room, &teacher, &entryDate, &startTimeStr, &isNow,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	h, m := s.parseTime(startTimeStr)

	fullTime := time.Date(
		entryDate.Year(), entryDate.Month(), entryDate.Day(),
		h, m, 0, 0, time.Local,
	)

	now := time.Now()
	isToday := entryDate.Year() == now.Year() && entryDate.YearDay() == now.YearDay()

	return &untis.RoomResult{
		Subject:   subject,
		Room:      room,
		Teacher:   teacher,
		StartTime: fullTime,
		IsToday:   isToday,
		IsNow:     isNow,
	}, nil
}

func (s *UntisService) GetUniqueSubjects(ctx context.Context, userID string, filter string) ([]string, error) {
	query := `
       SELECT DISTINCT subject 
       FROM timetable_entries 
       WHERE user_id = $1 AND subject ILIKE $2 
       ORDER BY subject 
       LIMIT 25`

	rows, err := s.db.QueryContext(ctx, query, userID, "%"+filter+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []string
	for rows.Next() {
		var sub string
		if err := rows.Scan(&sub); err == nil {
			subjects = append(subjects, sub)
		}
	}
	return subjects, nil
}

func (s *UntisService) GenerateScheduleImage(timetable *api.TimetableEntry, daysCount int, themeID string) (io.Reader, error) {
	config := DefaultRenderConfig()
	config.DaysCount = daysCount

	if themeID != "" && themeID != "default" {
		userTheme, err := LoadTheme(themeID)
		if err == nil {
			config.Theme = userTheme
		}
	}

	items := s.mapTimetableToItems(timetable, config.Theme)

	maxHour := config.PivotHour + config.HoursCount
	for _, item := range items {
		if item.EndH > maxHour {
			maxHour = item.EndH
		}
	}
	config.HoursCount = maxHour - config.PivotHour + 1

	renderer := NewCanvasRenderer(config, items)
	return renderer.Draw()
}

func (s *UntisService) GetThemes(filter string) (l []*string, err error) {
	entries, err := os.ReadDir(ThemeFolder)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".json") ||
			!strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(filter)) {
			continue
		}
		str := strings.ToUpper(strings.TrimSuffix(entry.Name(), ".json"))
		l = append(l, &str)
	}
	return l, nil
}

func (s *UntisService) SetTheme(ctx context.Context, id string, themeId string) error {
	query := `UPDATE users SET theme_id = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, themeId, id)
	return err
}

func (s *UntisService) GetTheme(themeID string) (*Theme, error) {
	if themeID == "default" {
		def := DefaultRenderConfig().Theme
		return &def, nil
	}
	theme, err := LoadTheme(themeID)
	if err != nil {
		return nil, errors.New("could not find theme")
	}
	return &theme, nil
}

func (s *UntisService) mapTimetableToItems(timetable *api.TimetableEntry, theme Theme) []RenderItem {
	var items []RenderItem

	for i, day := range timetable.Days {
		for _, slot := range day.GridEntries {
			sH, sM := s.parseTime(slot.Duration.Start)
			eH, eM := s.parseTime(slot.Duration.End)

			item := RenderItem{
				DayIndex: i, StartH: sH, StartM: sM, EndH: eH, EndM: eM,
				Color: theme.RegularBg, TextColor: theme.RegularText,
				Status: slot.Status,
			}

			switch slot.Status {
			case "SUBSTITUTION":
				item.Color, item.TextColor = theme.SubstitutionBg, theme.SubstitutionText
			case "CANCELLED":
				item.Color, item.TextColor = theme.CancelledBg, theme.CancelledText
			case "ROOM_CHANGE":
				item.Color, item.TextColor = theme.RoomChangeBg, theme.RoomChangeText
			case "EXAM":
				item.Color, item.TextColor = theme.ExamBg, theme.ExamText
			case "OFFICE_HOUR":
				item.Color, item.TextColor = theme.OfficeHourBg, theme.OfficeHourText
			case "ADDITIONAL":
				item.Color, item.TextColor = theme.AdditionalBg, theme.AdditionalText
			default:
				item.Color, item.TextColor = theme.RegularBg, theme.RegularText
			}

			allPositions := [][]api.Position{
				slot.Position1, slot.Position2, slot.Position3,
				slot.Position4, slot.Position5, slot.Position6,
				slot.Position7,
			}

			for _, posList := range allPositions {
				for _, pos := range posList {
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
			}

			items = append(items, item)
		}
	}
	return items
}

func (s *UntisService) parseTime(ts string) (int, int) {
	formats := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		time.RFC3339,
		"15:04:05",
		"15:04",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, ts); err == nil {
			return t.Hour(), t.Minute()
		}
	}

	clean := ts
	if strings.Contains(ts, "T") {
		parts := strings.Split(ts, "T")
		clean = parts[len(parts)-1]
	}
	clean = strings.TrimSuffix(clean, "Z")

	p := strings.Split(clean, ":")
	if len(p) >= 2 {
		h, _ := strconv.Atoi(p[0])
		m, _ := strconv.Atoi(p[1])
		return h, m
	}

	return 0, 0
}
func parseUntisDateTime(dateInt, timeInt int) time.Time {
	dStr := strconv.Itoa(dateInt)
	tStr := fmt.Sprintf("%04d", timeInt) // 940 -> "0940"

	t, _ := time.ParseInLocation("20060102 1504", dStr+" "+tStr, time.Local)
	return t
}
