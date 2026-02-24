package untis

import "time"

type User struct {
	ID                   string     `json:"id"`
	Username             string     `json:"username"`
	DisplayName          string     `json:"display_name"`
	Email                string     `json:"email"`
	UntisSchoolID        int64      `json:"untis_school_id"`
	UntisUser            string     `json:"untis_user"`
	UntisPassword        string     `json:"untis_password"`
	UntisPersonID        int64      `json:"untis_person_id"`
	ThemeID              string     `json:"theme_id"`
	NotificationsEnabled bool       `json:"notifications_enabled"`
	NotificationTarget   string     `json:"notification_target"`  // "DM", "CHANNEL", "WEBHOOK"
	NotificationAddress  string     `json:"notification_address"` // ID or URL
	AbsencesSyncedAt     *time.Time `json:"absences_synced_at"`
}
