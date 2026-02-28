package services

import (
	"context"
	"database/sql"
	"time"
)

type CleanupService struct {
	db            *sql.DB
	retentionDays int
}

func NewCleanupService(db *sql.DB) *CleanupService {
	return &CleanupService{db: db, retentionDays: 30}
}

func (s *CleanupService) SetRetentionDays(days int) {
	s.retentionDays = days
}

func (s *CleanupService) RunCleanup(ctx context.Context) (*CleanupResult, error) {
	result := &CleanupResult{}
	cutoffDate := time.Now().AddDate(0, 0, -s.retentionDays)

	r, err := s.db.ExecContext(ctx, `DELETE FROM timetable_entries WHERE entry_date < $1`, cutoffDate)
	if err == nil {
		if rows, _ := r.RowsAffected(); rows > 0 {
			result.TimetableEntriesDeleted = rows
		}
	}

	r, err = s.db.ExecContext(ctx, `DELETE FROM exams WHERE exam_date < $1`, cutoffDate)
	if err == nil {
		if rows, _ := r.RowsAffected(); rows > 0 {
			result.ExamsDeleted = rows
		}
	}

	r, err = s.db.ExecContext(ctx, `DELETE FROM homeworks WHERE due_date < $1`, cutoffDate)
	if err == nil {
		if rows, _ := r.RowsAffected(); rows > 0 {
			result.HomeworksDeleted = rows
		}
	}

	// Note: Absences are NOT cleaned up - they are kept indefinitely
	// Old absences are filtered during sync instead

	result.RanAt = time.Now()
	return result, nil
}

type CleanupResult struct {
	TimetableEntriesDeleted int64
	ExamsDeleted            int64
	HomeworksDeleted        int64
	AbsencesDeleted         int64
	RanAt                   time.Time
}
