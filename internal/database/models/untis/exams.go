package untis

import "time"

type ExamRecord struct {
	UntisID   int
	Date      time.Time
	StartTime string
	EndTime   string
	Subject   string
	Name      string
}
