package untis

import "time"

type AbsenceRecord struct {
	UntisID   int
	StartDate time.Time
	EndDate   time.Time
	Reason    string
	IsExcused bool
}
