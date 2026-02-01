package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"proximaLectio/internal/config"
	"proximaLectio/internal/database"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
)

type Handler struct {
	DB  *database.DB
	Cfg *config.Config
	Bot *bot.Client
}

func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
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
func getWeekRange(t time.Time) (time.Time, time.Time) {
	weekday := int(t.Weekday())

	daysToSubtract := weekday - 1
	if weekday == 0 {
		daysToSubtract = 6
	}

	monday := FloorToDay(t.AddDate(0, 0, -daysToSubtract))
	sunday := EndOfDay(monday.AddDate(0, 0, 6))

	return monday, sunday
}
func FloorToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}
func codeBloc(str string) string {
	return fmt.Sprintf("```\n%s\n```", str)
}
func getErrorEmbed(str string, err error) discord.MessageCreate {
	emb := discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(16711741).SetTitle("Failed").SetDescription(str)

	if err != nil {
		emb.AddField("Error", codeBloc(err.Error()), false)
	}

	return discord.NewMessageCreateBuilder().SetEphemeral(true).AddEmbeds(emb.Build()).Build()
}
func getErrorUpdateEmbed(str string, err error) discord.MessageUpdate {
	emb := discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(16711741).SetTitle("Failed").SetDescription(str)

	if err != nil {
		emb.AddField("Error", codeBloc(err.Error()), false)
	}

	return discord.NewMessageUpdateBuilder().AddEmbeds(emb.Build()).Build()
}
func getSuccessEmbed(body string, fields ...discord.EmbedField) discord.MessageCreate {
	emb := discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(9036596).SetTitle("Success").SetDescription(body)
	emb.AddFields(fields...)
	return discord.NewMessageCreateBuilder().SetEphemeral(true).AddEmbeds(emb.Build()).Build()
}
func getSuccessFileEmbed(name, desc string, file io.Reader) discord.MessageCreate {
	return discord.NewMessageCreateBuilder().AddFile(name, desc, file).SetEphemeral(true).Build()
}
func getSuccessFileUpdateEmbed(name, desc string, file io.Reader) discord.MessageUpdate {
	return discord.NewMessageUpdateBuilder().AddFile(name, desc, file).Build()
}
func getWarnEmbed(desc string) discord.MessageCreate {
	emb := discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(16751872).SetTitle("Warning").SetDescription(desc)
	return discord.NewMessageCreateBuilder().SetEphemeral(true).AddEmbeds(emb.Build()).Build()
}
