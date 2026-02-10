package utils

import (
	"fmt"
	"io"
	"time"

	"github.com/disgoorg/disgo/discord"
)

func CodeBloc(str string) string {
	return fmt.Sprintf("```\n%s\n```", str)
}

func buildErrorEmbed(str string, err error) discord.Embed {
	emb := discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(16711741).SetTitle("Failed").SetDescription(str)
	if err != nil {
		emb.AddField("Error", fmt.Sprintf("```\n%s\n```", err.Error()), false)
	}
	return emb.Build()
}

func buildSuccessEmbed(body string, fields ...discord.EmbedField) discord.Embed {
	emb := discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(9036596).SetTitle("Success").SetDescription(body)
	emb.AddFields(fields...)
	return emb.Build()
}

func buildWarnEmbed(desc string) discord.Embed {
	return discord.NewEmbedBuilder().SetTimestamp(time.Now()).SetColor(16751872).SetTitle("Warning").SetDescription(desc).Build()
}

// --- Regular Message Handlers (discord.MessageCreate) ---

func GetErrorEmbed(str string, err error) discord.MessageCreate {
	return discord.NewMessageCreateBuilder().SetEphemeral(true).AddEmbeds(buildErrorEmbed(str, err)).Build()
}
func GetSuccessEmbed(body string, fields ...discord.EmbedField) discord.MessageCreate {
	return discord.NewMessageCreateBuilder().SetEphemeral(true).AddEmbeds(buildSuccessEmbed(body, fields...)).Build()
}
func GetWarnEmbed(desc string) discord.MessageCreate {
	return discord.NewMessageCreateBuilder().SetEphemeral(true).AddEmbeds(buildWarnEmbed(desc)).Build()
}
func GetSuccessFileEmbed(name, desc string, file io.Reader) discord.MessageCreate {
	return discord.NewMessageCreateBuilder().AddFile(name, desc, file).SetEphemeral(true).Build()
}

// --- Deferred Response Handlers (discord.MessageUpdate) ---

func GetErrorUpdateEmbed(str string, err error) discord.MessageUpdate {
	return discord.NewMessageUpdateBuilder().AddEmbeds(buildErrorEmbed(str, err)).Build()
}
func GetSuccessUpdateEmbed(body string, fields ...discord.EmbedField) discord.MessageUpdate {
	return discord.NewMessageUpdateBuilder().AddEmbeds(buildSuccessEmbed(body, fields...)).Build()
}
func GetWarnUpdateEmbed(desc string) discord.MessageUpdate {
	return discord.NewMessageUpdateBuilder().AddEmbeds(buildWarnEmbed(desc)).Build()
}
func GetSuccessFileUpdateEmbed(name, desc string, file io.Reader) discord.MessageUpdate {
	return discord.NewMessageUpdateBuilder().AddFile(name, desc, file).Build()
}
