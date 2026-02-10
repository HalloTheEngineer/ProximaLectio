package database

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"proximaLectio/internal/config"
	"proximaLectio/internal/database/models"
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

func Connect(cfg *config.Config) *DB {
	db, err := sql.Open("postgres", cfg.DBConnectionString)
	if err != nil {
		slog.Error("Failed to open database connection", "error", err)
		os.Exit(1)
	}

	err = db.Ping()
	if err != nil {
		slog.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}

	slog.Info("(✓) Connected to PostgreSQL")

	for _, q := range models.GetSQLCreationQueries() {
		if _, err = db.Exec(q); err != nil {
			slog.Error("Failed to execute schema query", "error", err)
			os.Exit(1)
		}
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_stats_teacher ON timetable_entries(teacher);`); err != nil {
		slog.Warn("Failed to create index idx_stats_teacher", "error", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_stats_status ON timetable_entries(status);`); err != nil {
		slog.Warn("Failed to create index idx_stats_status", "error", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_stats_date ON timetable_entries(entry_date);`); err != nil {
		slog.Warn("Failed to create index idx_stats_date", "error", err)
	}

	return &DB{db: db, Untis: services.NewUntisService(db)}
}

func (d *DB) RegisterGuild(ctx context.Context, id, name string) error {
	query := `
		INSERT INTO guilds (id, name) 
		VALUES ($1, $2) 
		ON CONFLICT (id) DO UPDATE SET 
			name = EXCLUDED.name`

	_, err := d.db.ExecContext(ctx, query, id, name)
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
