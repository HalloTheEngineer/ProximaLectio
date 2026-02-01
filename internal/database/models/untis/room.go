package untis

import "time"

type RoomResult struct {
	Subject   string
	Room      string
	Teacher   string
	StartTime time.Time
	IsToday   bool
	IsNow     bool
}
