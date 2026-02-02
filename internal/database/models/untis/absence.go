package untis

import "time"

type AbsenceRecord struct {
	UntisID   int
	StartDate time.Time
	EndDate   time.Time
	Reason    string
	Status    string
	IsExcused bool
}
