package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"proximaLectio/internal/cache"
	"proximaLectio/internal/constants"
	"proximaLectio/internal/crypto"
	"proximaLectio/internal/database/models/untis"
	api "proximaLectio/internal/untis"
	"time"
)

const (
	SchoolSearchURL = "https://schoolsearch.webuntis.com/schoolquery2"
	AssetFolder     = "assets"
	ThemeFolder     = AssetFolder + string(os.PathSeparator) + "themes"
	FontFolder      = AssetFolder + string(os.PathSeparator) + "fonts"
)

type NotificationHandler func(ctx context.Context, userID string, target untis.NotificationTarget, subject string, date string, changeType string, oldVal, newVal string)

type UntisService struct {
	db        *sql.DB
	encryptor *crypto.Encryptor

	User    *UserService
	School  *SchoolService
	Guild   *GuildService
	SyncSvc *SyncService
	Render  *RenderService
	Theme   *ThemeService
	Cleanup *CleanupService

	cache               *cache.Service
	onStatusChangeHooks []NotificationHandler
}

func NewUntisService(db *sql.DB, encryptor *crypto.Encryptor) *UntisService {
	userSvc := NewUserService(db, encryptor)
	schoolSvc := NewSchoolService(db)
	guildSvc := NewGuildService(db)
	syncSvc := NewSyncService(db, encryptor, userSvc, schoolSvc)
	themeSvc := NewThemeService(db)
	renderSvc := NewRenderService(db, themeSvc)
	cleanupSvc := NewCleanupService(db)

	cacheSvc := cache.NewService(
		constants.CacheTTLUser,
		constants.CacheTTLSchool,
		constants.CacheTTLTheme,
		constants.CacheTTLSubjects,
		constants.CacheTTLSchoolSearch,
	)

	return &UntisService{
		db:        db,
		encryptor: encryptor,
		User:      userSvc,
		School:    schoolSvc,
		Guild:     guildSvc,
		SyncSvc:   syncSvc,
		Render:    renderSvc,
		Theme:     themeSvc,
		Cleanup:   cleanupSvc,
		cache:     cacheSvc,
	}
}

func (s *UntisService) RegisterNotificationHook(handler NotificationHandler) {
	s.onStatusChangeHooks = append(s.onStatusChangeHooks, handler)
	s.User.RegisterNotificationHook(handler)
	s.SyncSvc.RegisterNotificationHook(handler)
}

func (s *UntisService) AllowChannel(ctx context.Context, guildID, channelID string) error {
	return s.Guild.AllowChannel(ctx, guildID, channelID)
}

func (s *UntisService) RevokeChannel(ctx context.Context, guildID, channelID string) error {
	return s.Guild.RevokeChannel(ctx, guildID, channelID)
}

func (s *UntisService) IsChannelAllowed(ctx context.Context, guildID, channelID string) (bool, error) {
	return s.Guild.IsChannelAllowed(ctx, guildID, channelID)
}

func (s *UntisService) GetGuildMembers(ctx context.Context, guildID string) ([]GuildMember, error) {
	return s.Guild.GetGuildMembers(ctx, guildID)
}

func (s *UntisService) GetGuildMemberByDiscordID(ctx context.Context, guildID, discordID string) (*GuildMember, error) {
	return s.Guild.GetGuildMemberByDiscordID(ctx, guildID, discordID)
}

func (s *UntisService) UpsertSchool(ctx context.Context, tenantID, schoolID int, loginName, displayName, server, address string) error {
	err := s.School.UpsertSchool(ctx, tenantID, schoolID, loginName, displayName, server, address)
	if err == nil {
		s.cache.School.Delete(fmt.Sprintf("%d", tenantID))
	}
	return err
}

func (s *UntisService) GetSchool(ctx context.Context, tenantID string) (*untis.School, error) {
	key := "school:" + tenantID
	if val, ok := s.cache.School.Get(key); ok {
		if school, ok := val.(*untis.School); ok {
			return school, nil
		}
	}

	school, err := s.School.GetSchool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	s.cache.School.Set(key, school)
	return school, nil
}

func (s *UntisService) SearchSchools(ctx context.Context, query string, tenantID string) ([]untis.School, error) {
	cacheKey := "search:" + hashQuery(query, tenantID)
	if val, ok := s.cache.SchoolSearch.Get(cacheKey); ok {
		if schools, ok := val.([]untis.School); ok {
			return schools, nil
		}
	}

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

	s.cache.SchoolSearch.Set(cacheKey, rpcResp.Result.Schools)
	return rpcResp.Result.Schools, nil
}

func (s *UntisService) GetUser(ctx context.Context, id string) (*untis.User, error) {
	key := "user:" + id
	if val, ok := s.cache.User.Get(key); ok {
		if user, ok := val.(*untis.User); ok {
			return user, nil
		}
	}

	user, err := s.User.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cache.User.Set(key, user)
	return user, nil
}

func (s *UntisService) GetAllUsers(ctx context.Context) ([]*untis.User, error) {
	return s.User.GetAllUsers(ctx)
}

func (s *UntisService) LoginUser(ctx context.Context, school *untis.School, username, password, discordID, discordUsername string) (*untis.User, error) {
	user, err := s.SyncSvc.LoginUser(ctx, school, username, password, discordID, discordUsername)
	if err == nil {
		s.cache.User.Delete("user:" + discordID)
	}
	return user, err
}

func (s *UntisService) LogoutUser(ctx context.Context, id string) bool {
	result := s.User.DeleteUser(ctx, id)
	if result {
		s.cache.User.Delete("user:" + id)
		s.cache.Subjects.Delete("subjects:" + id)
	}
	return result
}

func (s *UntisService) UserExists(ctx context.Context, id string) bool {
	key := "user_exists:" + id
	if val, ok := s.cache.User.Get(key); ok {
		if exists, ok := val.(bool); ok {
			return exists
		}
	}

	exists := s.User.UserExists(ctx, id)
	s.cache.User.Set(key, exists)
	return exists
}

func (s *UntisService) SetNotificationConfig(ctx context.Context, id string, enabled bool, target string, address string) error {
	err := s.User.SetNotificationConfig(ctx, id, enabled, target, address)
	if err == nil {
		s.cache.User.Delete("user:" + id)
	}
	return err
}

func (s *UntisService) ToggleNotifications(ctx context.Context, id string, enabled bool) error {
	err := s.User.ToggleNotifications(ctx, id, enabled)
	if err == nil {
		s.cache.User.Delete("user:" + id)
	}
	return err
}

func (s *UntisService) CheckUserHomeworkAlerts(ctx context.Context, discordUserID string) error {
	return s.SyncSvc.CheckUserHomeworkAlerts(ctx, discordUserID)
}

func (s *UntisService) Sync(ctx context.Context, id string) bool {
	result := s.SyncSvc.Sync(ctx, id)
	s.cache.Subjects.Delete("subjects:" + id)
	return result
}

func (s *UntisService) SyncUserAbsences(ctx context.Context, discordUserID string) error {
	return s.SyncSvc.SyncUserAbsences(ctx, discordUserID)
}

func (s *UntisService) SyncUserExams(ctx context.Context, discordUserID string) error {
	return s.SyncSvc.SyncUserExams(ctx, discordUserID)
}

func (s *UntisService) SyncUserTimetable(ctx context.Context, id string, start, end time.Time) error {
	err := s.SyncSvc.SyncUserTimetable(ctx, id, start, end)
	if err == nil {
		s.cache.Subjects.Delete("subjects:" + id)
	}
	return err
}

func (s *UntisService) SyncUserHomeworks(ctx context.Context, discordUserID string) error {
	return s.SyncSvc.SyncUserHomeworks(ctx, discordUserID)
}

func (s *UntisService) GetTimetable(ctx context.Context, userID string, start, end time.Time) (*api.TimetableEntry, error) {
	return s.SyncSvc.GetTimetable(ctx, userID, start, end)
}

func (s *UntisService) GetUserAbsences(ctx context.Context, userID string, filter int) ([]untis.AbsenceRecord, error) {
	return s.SyncSvc.GetUserAbsences(ctx, userID, filter)
}

func (s *UntisService) SearchAbsencesForAutocomplete(ctx context.Context, userID string, query string) ([]untis.AbsenceRecord, error) {
	return s.SyncSvc.SearchAbsencesForAutocomplete(ctx, userID, query)
}

func (s *UntisService) GetUpcomingExams(ctx context.Context, userID string) ([]untis.ExamRecord, error) {
	return s.SyncSvc.GetUpcomingExams(ctx, userID)
}

func (s *UntisService) GetUserHomeworks(ctx context.Context, userID string, filter int) ([]untis.HomeworkRecord, error) {
	return s.SyncSvc.GetUserHomeworks(ctx, userID, filter)
}

func (s *UntisService) GetUserStats(ctx context.Context, userID string) (*untis.UserStats, error) {
	return s.SyncSvc.GetUserStats(ctx, userID)
}

func (s *UntisService) GetGuildMemberStatusesAt(ctx context.Context, guildID string, targetTime time.Time) ([]untis.UserScheduleStatus, error) {
	return s.SyncSvc.GetGuildMemberStatusesAt(ctx, guildID, targetTime)
}

func (s *UntisService) GetNextRoomForSubject(ctx context.Context, userID string, subjectQuery string) (*untis.RoomResult, error) {
	return s.SyncSvc.GetNextRoomForSubject(ctx, userID, subjectQuery)
}

func (s *UntisService) GetUniqueSubjects(ctx context.Context, userID string, filter string) ([]string, error) {
	key := "subjects:" + userID + ":" + filter
	if val, ok := s.cache.Subjects.Get(key); ok {
		if subjects, ok := val.([]string); ok {
			return subjects, nil
		}
	}

	subjects, err := s.SyncSvc.GetUniqueSubjects(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	s.cache.Subjects.Set(key, subjects)
	return subjects, nil
}

func (s *UntisService) GenerateScheduleImage(timetable *api.TimetableEntry, daysCount int, themeID string) (io.Reader, error) {
	return s.Render.GenerateScheduleImage(timetable, daysCount, themeID)
}

func (s *UntisService) GenerateExcusePDF(ctx context.Context, userID string, untisID int, guardian string) (io.Reader, error) {
	return s.SyncSvc.GenerateExcusePDF(ctx, userID, untisID, guardian)
}

func (s *UntisService) GetThemes(filter string) ([]*string, error) {
	return s.Theme.GetThemes(filter)
}

func (s *UntisService) GetTheme(themeID string) (*Theme, error) {
	key := "theme:" + themeID
	if val, ok := s.cache.Theme.Get(key); ok {
		if theme, ok := val.(*Theme); ok {
			return theme, nil
		}
	}

	theme, err := s.Theme.GetTheme(themeID)
	if err != nil {
		return nil, err
	}

	s.cache.Theme.Set(key, theme)
	return theme, nil
}

func (s *UntisService) SetTheme(ctx context.Context, id string, themeID string) error {
	err := s.Theme.SetTheme(ctx, id, themeID)
	if err == nil {
		s.cache.User.Delete("user:" + id)
	}
	return err
}

func (s *UntisService) RunCleanup(ctx context.Context) (*CleanupResult, error) {
	return s.Cleanup.RunCleanup(ctx)
}

func (s *UntisService) SetRetentionDays(days int) {
	s.Cleanup.SetRetentionDays(days)
}

func (s *UntisService) CacheStats() map[string]interface{} {
	userHits, userMisses := s.cache.User.Stats()
	schoolHits, schoolMisses := s.cache.School.Stats()
	themeHits, themeMisses := s.cache.Theme.Stats()
	subjectsHits, subjectsMisses := s.cache.Subjects.Stats()
	searchHits, searchMisses := s.cache.SchoolSearch.Stats()

	return map[string]interface{}{
		"user": map[string]interface{}{
			"hits":   userHits,
			"misses": userMisses,
			"size":   s.cache.User.Size(),
		},
		"school": map[string]interface{}{
			"hits":   schoolHits,
			"misses": schoolMisses,
			"size":   s.cache.School.Size(),
		},
		"theme": map[string]interface{}{
			"hits":   themeHits,
			"misses": themeMisses,
			"size":   s.cache.Theme.Size(),
		},
		"subjects": map[string]interface{}{
			"hits":   subjectsHits,
			"misses": subjectsMisses,
			"size":   s.cache.Subjects.Size(),
		},
		"search": map[string]interface{}{
			"hits":   searchHits,
			"misses": searchMisses,
			"size":   s.cache.SchoolSearch.Size(),
		},
	}
}

func (s *UntisService) mapTimetableToItems(timetable *api.TimetableEntry, theme Theme) []RenderItem {
	return s.Render.mapTimetableToItems(timetable, theme)
}

func (s *UntisService) parseTime(ts string) (int, int) {
	return parseTime(ts)
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

func hashQuery(query, tenantID string) string {
	h := sha256.New()
	h.Write([]byte(query + ":" + tenantID))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
