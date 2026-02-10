package events

import (
	"context"
	"fmt"
	"log/slog"
	"proximaLectio/internal/database/models/untis"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

func (h *Handler) RegisterNotificationHooks() {
	h.DB.Untis.RegisterNotificationHook(func(ctx context.Context, userID string, target untis.NotificationTarget, subject, date, changeType, old, new string) {
		var title string
		var description string
		var color = 0x5865F2

		switch changeType {
		case "STATUS":
			title = "Lesson Status Changed"
			description = fmt.Sprintf("Your **%s** lesson on **%s** changed from `%s` to **`%s`**.", subject, date, old, new)
		case "TEACHER":
			title = "Teacher Substitution"
			description = fmt.Sprintf("Your **%s** lesson on **%s** has a new teacher: `%s` → **`%s`**.", subject, date, old, new)
		case "ROOM":
			title = "Room Change"
			description = fmt.Sprintf("The room for **%s** on **%s** moved: `%s` → **`%s`**.", subject, date, old, new)

		case "ABSENCE_NEW":
			title = "⚠️ New Absence Recorded"
			description = fmt.Sprintf("A new absence has been recorded for **%s**.\nReason: `%s`", date, new)
			color = 0xE67E22
		case "ABSENCE_EXCUSED":
			title = "✅ Absence Status Updated"
			description = fmt.Sprintf("Your absence on **%s** is now marked as **%s**.", date, new)
			color = 0x2ECC71
		case "ABSENCE_REASON":
			title = "📝 Absence Reason Updated"
			description = fmt.Sprintf("The reason for your absence on **%s** was updated: `%s` → **`%s`**.", date, old, new)

		case "EXAM_NEW":
			title = "📝 New Exam Scheduled"
			description = fmt.Sprintf("A new exam/test for **%s** has been detected in your timetable!\n\n**Date:** %s\n**Type:** %s", subject, date, new)
			color = 0x9B59B6

		case "AUTH_FAILURE":
			title = "🔒 Authentication Failed"
			description = "The bot could no longer log into your WebUntis account. Did you change your password? Please use `/login` again to restore service."
			color = 0xED4245

		case "HOMEWORK_DUE":
			title = "📚 Homework Due Tomorrow"
			description = fmt.Sprintf("You have homework due tomorrow (**%s**) for **%s**:\n\n> %s", date, subject, new)
			color = 0xF1C40F
		}

		embed := discord.NewEmbedBuilder().
			SetTitle(title).
			SetDescription(description).
			SetColor(color).
			SetTimestamp(time.Now()).
			Build()

		h.sendNotification(ctx, userID, target, embed)
	})
}

func (h *Handler) sendNotification(ctx context.Context, userID string, target untis.NotificationTarget, embed discord.Embed) {
	if h.Bot == nil {
		slog.Error("Cannot send notification: Bot client not initialized")
		return
	}
	b := *h.Bot

	switch target.Type {
	case "DM":
		channel, err := b.Rest().CreateDMChannel(snowflake.MustParse(userID))
		if err != nil {
			slog.Error("Failed to create DM channel", "user", userID, "error", err)
			return
		}
		_, _ = b.Rest().CreateMessage(channel.ID(), discord.NewMessageCreateBuilder().SetEmbeds(embed).Build())

	case "CHANNEL":
		_, _ = b.Rest().CreateMessage(
			snowflake.MustParse(target.Address),
			discord.NewMessageCreateBuilder().SetContentf("||<@%s>|| <a:alert:1467490337839648818>", userID).SetEmbeds(embed).Build(),
		)

	case "WEBHOOK":
		client := rest.NewWebhooks(b.Rest())
		parts := strings.Split(target.Address, "/")
		if len(parts) < 2 {
			return
		}
		webhookID := snowflake.MustParse(parts[len(parts)-2])
		token := parts[len(parts)-1]

		_, _ = client.CreateWebhookMessage(
			webhookID,
			token,
			discord.NewWebhookMessageCreateBuilder().SetEmbeds(embed).Build(), rest.CreateWebhookMessageParams{Wait: false},
		)
	}
}
