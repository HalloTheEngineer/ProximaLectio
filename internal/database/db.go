package database

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"proximaLectio/internal/config"
	"proximaLectio/internal/constants"
	"proximaLectio/internal/crypto"
	"proximaLectio/internal/database/migrations"
	"proximaLectio/internal/database/services"

	_ "github.com/lib/pq"
)

type DB struct {
	db    *sql.DB
	Untis *services.UntisService
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DB) RawDB() *sql.DB {
	return d.db
}

func Connect(cfg *config.Config) *DB {
	db, err := sql.Open("postgres", cfg.DBConnectionString)
	if err != nil {
		slog.Error("Failed to open database connection", "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(constants.DBMaxOpenConns)
	db.SetMaxIdleConns(constants.DBMaxIdleConns)
	db.SetConnMaxLifetime(constants.DBConnMaxLifetime)

	err = db.Ping()
	if err != nil {
		slog.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}

	slog.Info("(✓) Connected to PostgreSQL")

	if err := migrations.Migrate(db); err != nil {
		slog.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	encryptor, err := crypto.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		slog.Error("Failed to initialize encryptor", "error", err)
		os.Exit(1)
	}

	return &DB{db: db, Untis: services.NewUntisService(db, encryptor)}
}

func (d *DB) RegisterGuild(ctx context.Context, id, name string) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO guilds (id, name) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`, id, name)
	return err
}
func (d *DB) SyncGuildMembership(ctx context.Context, userID, guildID string) error {
	query := `
		INSERT INTO guild_members (user_id, guild_id)
		SELECT id, $2 FROM users WHERE id = $1
		ON CONFLICT (user_id, guild_id) DO NOTHING`

	_, err := d.db.ExecContext(ctx, query, userID, guildID)
	return err
}
