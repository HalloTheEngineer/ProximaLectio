package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron"
)

func StartSyncWorker(ctx context.Context, s *UntisService, syncSpec string, homeworkSpec string) {
	scheduler := gocron.NewScheduler(time.Local)

	_, err := scheduler.Cron(syncSpec).Do(func() {
		slog.Info("Starting scheduled general sync cycle")
		runGeneralSync(ctx, s)
	})
	if err != nil {
		slog.Error("Failed to schedule general sync job", "error", err)
	}

	_, err = scheduler.Cron(homeworkSpec).Do(func() {
		slog.Info("Starting scheduled daily homework alert check")
		runHomeworkCheck(ctx, s)
	})
	if err != nil {
		slog.Error("Failed to schedule homework alert job", "error", err)
	}

	scheduler.StartAsync()
	slog.Info("Gocron sync worker started", "sync_schedule", syncSpec, "homework_schedule", homeworkSpec)

	go func() {
		<-ctx.Done()
		slog.Info("Stopping sync worker...")
		scheduler.Stop()
	}()
}

func runGeneralSync(ctx context.Context, s *UntisService) {
	users, err := s.GetAllUsers(ctx)
	if err != nil {
		slog.Error("Could not fetch users for sync", "error", err)
		return
	}

	for _, user := range users {
		userCtx, cancel := context.WithTimeout(ctx, 60*time.Second)

		if !s.Sync(userCtx, user.ID) {
			cancel()
			continue
		}

		cancel()
		time.Sleep(500 * time.Millisecond)
	}
}

func runHomeworkCheck(ctx context.Context, s *UntisService) {
	users, err := s.GetAllUsers(ctx)
	if err != nil {
		return
	}

	for _, user := range users {
		userCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		if err := s.CheckUserHomeworkAlerts(userCtx, user.ID); err != nil {
			slog.Warn("Daily homework check failed", "user", user.ID, "error", err)
		}

		cancel()
		time.Sleep(500 * time.Millisecond)
	}
}
