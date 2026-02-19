package config

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Verbose            bool
	NoCommandUpdate    bool
	BotToken           string
	DBConnectionString string
	EncryptionKey      string
}

func Load() *Config {
	config := &Config{
		Verbose:            os.Getenv("VERBOSE") == "1",
		NoCommandUpdate:    os.Getenv("NO_COMMAND_UPDATE") == "1",
		BotToken:           os.Getenv("DISCORD_TOKEN"),
		DBConnectionString: os.Getenv("DB_CONNECTION_STRING"),
		EncryptionKey:      getEnvWithDefault("ENCRYPTION_KEY", "default-32-byte-encryption!"),
	}

	var missing []string

	if config.BotToken == "" {
		missing = append(missing, "DISCORD_TOKEN")
	}
	if config.DBConnectionString == "" {
		missing = append(missing, "DB_CONNECTION_STRING")
	}

	if len(missing) > 0 {
		slog.Error("Missing required environment variables", "variables", strings.Join(missing, " "))
		os.Exit(1)
	}

	if len(config.EncryptionKey) != 16 && len(config.EncryptionKey) != 24 && len(config.EncryptionKey) != 32 {
		slog.Warn("ENCRYPTION_KEY should be 16, 24, or 32 bytes for AES encryption. Using default key.")
		config.EncryptionKey = "default-32-byte-encryption!"
	}

	return config
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
