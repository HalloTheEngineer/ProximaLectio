package utils

import "time"

func GetWeekRange(t time.Time) (time.Time, time.Time) {
	weekday := int(t.Weekday())

	daysToSubtract := weekday - 1
	if weekday == 0 {
		daysToSubtract = 6
	}

	monday := FloorToDay(t.AddDate(0, 0, -daysToSubtract))
	sunday := EndOfDay(monday.AddDate(0, 0, 6))

	return monday, sunday
}

func FloorToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}
