package database

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
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
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("(✓) Connected to PostgreSQL")

	for _, q := range models.GetSQLCreationQueries() {
		if _, err = db.Exec(q); err != nil {
			log.Fatal(err)
		}
	}

	db.Exec(`CREATE INDEX IF NOT EXISTS idx_stats_teacher ON timetable_entries(teacher);`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_stats_status ON timetable_entries(status);`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_stats_date ON timetable_entries(entry_date);`)

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
