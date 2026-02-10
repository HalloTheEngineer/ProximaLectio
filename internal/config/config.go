package config

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Verbose            string
	NoCommandUpdate    bool
	BotToken           string
	DBConnectionString string
}

func Load() *Config {

	config := &Config{
		Verbose:            os.Getenv("VERBOSE"),
		NoCommandUpdate:    os.Getenv("NO_COMMAND_UPDATE") == "1",
		BotToken:           os.Getenv("DISCORD_TOKEN"),
		DBConnectionString: os.Getenv("DB_CONNECTION_STRING"),
	}

	var s []string

	if config.BotToken == "" {
		s = append(s, "DISCORD_TOKEN")
	}
	if config.DBConnectionString == "" {
		s = append(s, "DB_CONNECTION_STRING")
	}

	if len(s) > 0 {
		slog.Error("Missing required environment variables", "variables", strings.Join(s, " "))
		os.Exit(1)
	}

	return config
}
