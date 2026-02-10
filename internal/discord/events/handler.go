package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"proximaLectio/internal/config"
	"proximaLectio/internal/database"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
)

type Handler struct {
	DB        *database.DB
	Cfg       *config.Config
	Bot       *bot.Client
	syncCache sync.Map
}

func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}

func (h *Handler) safeSyncGuild(userID, guildID string) {
	cacheKey := userID + ":" + guildID

	if _, found := h.syncCache.Load(cacheKey); found {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = h.DB.RegisterGuild(ctx, guildID, "Active Server")

	err := h.DB.SyncGuildMembership(ctx, userID, guildID)
	if err != nil {
		fmt.Printf("Warning: Failed to sync guild membership for user %s: %v\n", userID, err)
		return
	}

	h.syncCache.Store(cacheKey, true)
}

// --- UTILS ---

func updateInteractionResp(bot *bot.Client, token string, msg discord.MessageUpdate) error {
	if bot == nil {
		return errors.New("bot is nil")
	}
	b := *bot

	_, err := b.Rest().UpdateInteractionResponse(b.ApplicationID(), token, msg)
	return err
}

func parseParams(data *discord.SlashCommandInteractionData, names ...string) (a []any, errParam *string) {
	findOption := func(name string) (json.RawMessage, bool) {
		for _, opt := range data.Options {
			if opt.Name == name {
				return opt.Value, true
			}
		}
		return nil, false
	}
	for _, name := range names {
		val, ok := findOption(name)
		if !ok {
			return nil, &name
		}
		var v any
		if err := json.Unmarshal(val, &v); err != nil {
			return nil, &name
		}
		a = append(a, v)
	}
	return a, nil
}
