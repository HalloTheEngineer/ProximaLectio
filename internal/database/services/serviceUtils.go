package services

import (
	"fmt"
	"strings"
	"time"
)

func parseTime(ts string) (int, int) {
	formats := []string{"2006-01-02T15:04", time.RFC3339, "15:04:05", "15:04"}
	for _, f := range formats {
		if t, err := time.Parse(f, ts); err == nil {
			return t.Hour(), t.Minute()
		}
	}
	return 0, 0
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
