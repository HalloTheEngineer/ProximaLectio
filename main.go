package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"proximaLectio/internal/config"
	"proximaLectio/internal/database"
	"proximaLectio/internal/database/services"
	"proximaLectio/internal/discord"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()

	if cfg.Verbose == "1" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	db := database.Connect(cfg)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	services.StartSyncWorker(ctx, db.Untis, 2*time.Hour)

	go func() {
		client := discord.Launch(db, cfg)

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

	time.Sleep(1 * time.Second)

	slog.Info("(✓) Exiting program.")
}
