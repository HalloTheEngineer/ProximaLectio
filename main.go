package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"proximaLectio/internal/config"
	"proximaLectio/internal/constants"
	"proximaLectio/internal/database"
	"proximaLectio/internal/database/services"
	"proximaLectio/internal/discord"
	"proximaLectio/internal/health"
	"runtime/debug"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()

	if cfg.Verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	slog.Info("Starting ProximaLectio...")

	db := database.Connect(cfg)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthChecker := health.NewChecker(db.RawDB(), cfg.HealthPort)
	healthChecker.Start()

	slog.Info("Starting sync worker...")
	func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC in StartSyncWorker", "recover", r, "stack", string(debug.Stack()))
			}
		}()
		services.StartSyncWorker(ctx, db.Untis, constants.ScheduleSyncCron, constants.HomeworkAlertCron, constants.CleanupCron)
	}()
	slog.Info("(✓) Sync worker started")

	slog.Info("Launching Discord client...")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Discord client panicked", "recover", r, "stack", string(debug.Stack()))
			}
		}()

		client := discord.Launch(ctx, db, cfg)

		<-ctx.Done()
		if client != nil {
			slog.Info("(✓) Shutting down Discord client...")
			client.Close(context.Background())
		}
	}()

	slog.Info("(✓) ProximaLectio services are running.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-sig

	slog.Info("(✓) Shutdown signal received. Cleaning up...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := healthChecker.Stop(shutdownCtx); err != nil {
		slog.Warn("Health checker shutdown error", "error", err)
	}

	time.Sleep(1 * time.Second)

	slog.Info("(✓) Exiting program.")
}
