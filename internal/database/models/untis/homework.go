package untis

import "time"

type HomeworkRecord struct {
	UntisID   int       `json:"untis_id"`
	Subject   string    `json:"subject"`
	Text      string    `json:"text"`
	DueDate   time.Time `json:"due_date"`
	Completed bool      `json:"completed"`
}
