package untis

type TimetableEntry struct {
	Days []struct {
		Date        string       `json:"date"`
		GridEntries []LessonSlot `json:"gridEntries"`
	} `json:"days"`
}

type Position struct {
	Current *struct {
		Type      string `json:"type"`
		ShortName string `json:"shortName"`
	} `json:"current"`
}

type LessonSlot struct {
	Duration struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"duration"`
	Status    string     `json:"status"`
	Position1 []Position `json:"position1"`
	Position2 []Position `json:"position2"`
	Position3 []Position `json:"position3"`
	Position4 []Position `json:"position4"`
	Position5 []Position `json:"position5"`
	Position6 []Position `json:"position6"`
	Position7 []Position `json:"position7"`
}

type UserScheduleStatus struct {
	UserID   string
	Username string
	Subject  string // Empty if free
	Room     string
	Status   string // REGULAR, CANCELLED, etc.
	IsFree   bool
}
