package discord

import (
	"context"
	"log"
	"log/slog"
	"proximaLectio/internal/config"
	"proximaLectio/internal/database"
	"proximaLectio/internal/database/services"
	"proximaLectio/internal/discord/events"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	ev "github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

func Launch(db *database.DB, cfg *config.Config) bot.Client {
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
				// gateway.IntentMessageContent,
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
		log.Fatalf("Failed to initialize Disgo client: %v", err)
		return nil
	}

	if !cfg.NoCommandUpdate {
		if _, err = client.Rest().SetGlobalCommands(client.ApplicationID(), GlobalCommands); err != nil {
			slog.Error("Failed to register global commands", "error", err)
		} else {
			slog.Info("(✓) Discord Bot Registered Commands")
		}
	}

	if err = client.OpenGateway(context.Background()); err != nil {
		log.Fatalf("Failed to open gateway: %v", err)
		return nil
	}

	h.Bot = &client

	h.RegisterNotificationHooks()

	slog.Info("(✓) Discord Bot Connected to Gateway")

	return client
}
