package untis

type NotificationType string

const (
	NotificationTypeDM      NotificationType = "DM"
	NotificationTypeChannel NotificationType = "CHANNEL"
	NotificationTypeWebhook NotificationType = "WEBHOOK"
)

type NotificationTarget struct {
	Type    string // DM, CHANNEL, WEBHOOK
	Address string // Channel ID or URL
}
