package services

import (
	"context"
	"log/slog"
	"time"
)

func StartSyncWorker(ctx context.Context, s *UntisService, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		slog.Info("(✓) Starting periodic timetable sync worker", "interval", interval)

		for {
			select {
			case <-ctx.Done():
				slog.Info("(✓) Stopping sync worker...")
				return
			case <-ticker.C:
				runSyncCycle(ctx, s)
			}
		}
	}()
}

func runSyncCycle(ctx context.Context, s *UntisService) {
	slog.Info("(✓) Starting global sync cycle...")
	startCycle := time.Now()

	users, err := s.GetAllUsers(ctx)
	if err != nil {
		slog.Error("Failed to fetch users for sync cycle", "error", err)
		return
	}

	for _, user := range users {
		userCtx, cancel := context.WithTimeout(ctx, 45*time.Second)

		slog.Debug("Syncing user", "username", user.Username, "id", user.ID)

		start := time.Now().AddDate(0, 0, -2)
		end := time.Now().AddDate(0, 0, 7)

		err := s.SyncUserTimetable(userCtx, user.ID, start, end)
		if err != nil {
			slog.Warn("Failed to sync user timetable", "user_id", user.ID, "error", err)
		}

		if err := s.SyncUserAbsences(userCtx, user.ID); err != nil {
			slog.Warn("Absence sync failed", "user", user.ID, "error", err)
		}

		cancel()

		time.Sleep(1 * time.Second)
	}

	slog.Info("(✓) Global sync cycle completed", "duration", time.Since(startCycle), "user_count", len(users))
}
