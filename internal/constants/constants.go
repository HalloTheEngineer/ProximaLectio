package constants

import "time"

const (
	TimeoutShort  = 3 * time.Second
	TimeoutMedium = 8 * time.Second
	TimeoutLong   = 15 * time.Second
	TimeoutSync   = 60 * time.Second
	TimeoutRender = 80 * time.Second

	SyncIntervalMs = 500 * time.Millisecond

	DBMaxOpenConns    = 25
	DBMaxIdleConns    = 5
	DBConnMaxLifetime = 5 * time.Minute

	MaxAutocompleteResults = 20
	MaxSchoolResults       = 15
	MaxAbsencesDisplay     = 25
	MaxExamsDisplay        = 15

	ScheduleSyncCron  = "0 6-18 * * *"
	HomeworkAlertCron = "0 19 * * *"
	CleanupCron       = "0 3 * * *"

	DataRetentionDays = 30

	RateLimitRPS   = 2
	RateLimitBurst = 5

	CacheTTLUser         = 5 * time.Minute
	CacheTTLSchool       = 30 * time.Minute
	CacheTTLTheme        = 1 * time.Hour
	CacheTTLSubjects     = 2 * time.Minute
	CacheTTLSchoolSearch = 10 * time.Minute
)

const (
	ColorSuccess = 0x2ECC71
	ColorWarning = 0xF1C40F
	ColorError   = 0xED4245
	ColorInfo    = 0x5865F2
	ColorPrimary = 9036596
)
