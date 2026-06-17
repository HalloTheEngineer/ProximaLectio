package discord

import (
	"context"
	"log/slog"
	"math"
	"proximaLectio/internal/config"
	"proximaLectio/internal/database"
	"proximaLectio/internal/database/services"
	"proximaLectio/internal/discord/events"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	ev "github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

const (
	maxConnectAttempts = 8
	baseRetryDelay     = 2 * time.Second
	maxRetryDelay      = 5 * time.Minute
)

func Launch(ctx context.Context, db *database.DB, cfg *config.Config) bot.Client {
	h := events.NewHandler(db, cfg)

	go func() {
		slog.Info("(✓) Pre-warming renderer assets...")
		services.PreWarmRenderer()
		slog.Info("(✓) Renderer assets cached")
	}()

	client, err := disgo.New(cfg.BotToken,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
			),
			gateway.WithPresenceOpts(
				gateway.WithPlayingActivity("Untis"),
			),
		),
		bot.WithCacheConfigOpts(
			cache.WithGuildCachePolicy(cache.PolicyAll),
			cache.WithCaches(cache.FlagGuilds),
		),
		bot.WithDefaultShardManager(),
		bot.WithEventListeners(
			&ev.ListenerAdapter{
				OnApplicationCommandInteraction: h.CommandListener,
				OnAutocompleteInteraction:       h.AutocompleteListener,
				OnReady:                         h.OnReady,
			},
		),
	)
	if err != nil {
		slog.Error("Failed to initialize Disgo client", "error", err)
		return nil
	}

	if !cfg.NoCommandUpdate {
		if _, err = client.Rest().SetGlobalCommands(client.ApplicationID(), GlobalCommands); err != nil {
			slog.Error("Failed to register global commands", "error", err)
		} else {
			slog.Info("(✓) Discord Bot Registered Commands")
		}
	}

	if err = openGatewayWithRetry(ctx, client); err != nil {
		slog.Error("Failed to open gateway after all retries", "error", err)
		return nil
	}

	h.Bot = &client

	h.RegisterNotificationHooks()

	slog.Info("(✓) Discord Bot Connected to Gateway")

	return client
}

func openGatewayWithRetry(ctx context.Context, client bot.Client) error {
	var lastErr error
	for attempt := 1; attempt <= maxConnectAttempts; attempt++ {
		slog.Info("Opening Discord gateway", "attempt", attempt, "max", maxConnectAttempts)

		lastErr = client.OpenGateway(ctx)
		if lastErr == nil {
			return nil
		}

		slog.Warn("Gateway connection failed", "attempt", attempt, "error", lastErr)

		delay := time.Duration(math.Min(
			float64(baseRetryDelay)*math.Pow(2, float64(attempt-1)),
			float64(maxRetryDelay),
		))

		slog.Info("Retrying gateway connection", "delay", delay.Round(time.Second))

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}
