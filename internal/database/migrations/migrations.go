package migrations

import (
	"database/sql"
	"log/slog"
)

type Migration struct {
	Version int
	Name    string
	Up      string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		Up: `
			CREATE TABLE IF NOT EXISTS schools (
			    tenant_id INT PRIMARY KEY,
			    school_id INT,
				display_name VARCHAR(255) NOT NULL,
				login_name VARCHAR(255) NOT NULL, 
				server VARCHAR(255) NOT NULL,
				address TEXT,
				last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
			
			CREATE TABLE IF NOT EXISTS users (
				id VARCHAR(64) PRIMARY KEY,
				username VARCHAR(255) NOT NULL,
			    display_name VARCHAR(255),
				email VARCHAR(255),
				untis_school_tenant_id INT REFERENCES schools(tenant_id),
				untis_user VARCHAR(255),
				untis_password TEXT,
				untis_person_id INTEGER,
				theme_id VARCHAR(64) DEFAULT 'default',
				notifications_enabled BOOLEAN DEFAULT FALSE,
				notification_target VARCHAR(20) DEFAULT 'DM',
				notification_address TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
			
			CREATE TABLE IF NOT EXISTS guilds (
				id VARCHAR(64) PRIMARY KEY, 
				name VARCHAR(255) NOT NULL, 
				joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
			
			CREATE TABLE IF NOT EXISTS guild_members (
				user_id VARCHAR(64) REFERENCES users(id) ON DELETE CASCADE,
				guild_id VARCHAR(64) REFERENCES guilds(id) ON DELETE CASCADE,
				joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (user_id, guild_id)
			);
			
			CREATE TABLE IF NOT EXISTS allowed_notification_channels (
				guild_id VARCHAR(64) REFERENCES guilds(id) ON DELETE CASCADE,
				channel_id VARCHAR(64) NOT NULL,
				PRIMARY KEY (guild_id, channel_id)
			);
			
			CREATE TABLE IF NOT EXISTS timetable_entries (
				id SERIAL PRIMARY KEY,
				user_id VARCHAR(64) REFERENCES users(id) ON DELETE CASCADE,
				entry_date DATE NOT NULL,
				start_time TIME NOT NULL,
				end_time TIME NOT NULL,
				subject VARCHAR(50),
				teacher VARCHAR(50),
				room VARCHAR(50),
				status VARCHAR(50),
				last_synced TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(user_id, entry_date, start_time, subject)
			);
			
			CREATE TABLE IF NOT EXISTS absences (
				id SERIAL PRIMARY KEY,
				untis_id INTEGER NOT NULL,
				user_id VARCHAR(64) REFERENCES users(id) ON DELETE CASCADE,
				start_date DATE NOT NULL,
				end_date DATE NOT NULL,
				start_time INTEGER,
				end_time INTEGER,
				reason TEXT,
				status VARCHAR(64),
				is_excused BOOLEAN DEFAULT FALSE,
				UNIQUE(user_id, untis_id)
			);
			
			CREATE TABLE IF NOT EXISTS exams (
				id SERIAL PRIMARY KEY,
				untis_id INTEGER NOT NULL,
				user_id VARCHAR(64) REFERENCES users(id) ON DELETE CASCADE,
				exam_date DATE NOT NULL,
				start_time TIME,
				end_time TIME,
				subject VARCHAR(255),
				name TEXT,
				last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(user_id, untis_id)
			);
		`,
	},
	{
		Version: 2,
		Name:    "add_indexes",
		Up: `
			CREATE INDEX IF NOT EXISTS idx_stats_teacher ON timetable_entries(teacher);
			CREATE INDEX IF NOT EXISTS idx_stats_status ON timetable_entries(status);
			CREATE INDEX IF NOT EXISTS idx_stats_date ON timetable_entries(entry_date);
			CREATE INDEX IF NOT EXISTS idx_timetable_user_date ON timetable_entries(user_id, entry_date);
			CREATE INDEX IF NOT EXISTS idx_absences_user ON absences(user_id);
			CREATE INDEX IF NOT EXISTS idx_exams_user_date ON exams(user_id, exam_date);
		`,
	},
	{
		Version: 3,
		Name:    "schema_migrations_table",
		Up: `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INT PRIMARY KEY,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
	{
		Version: 4,
		Name:    "add_substitution_text",
		Up: `
			ALTER TABLE timetable_entries ADD COLUMN IF NOT EXISTS substitution_text VARCHAR(50) DEFAULT '';
		`,
	},
	{
		Version: 5,
		Name:    "add_homeworks_table",
		Up: `
			CREATE TABLE IF NOT EXISTS homeworks (
				id SERIAL PRIMARY KEY,
				untis_id INTEGER NOT NULL,
				user_id VARCHAR(64) REFERENCES users(id) ON DELETE CASCADE,
				subject VARCHAR(255),
				text TEXT,
				due_date DATE,
				completed BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(user_id, untis_id)
			);
			CREATE INDEX IF NOT EXISTS idx_homeworks_user_due ON homeworks(user_id, due_date);
		`,
	},
}

func Migrate(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	for _, m := range migrations {
		applied, err := isMigrationApplied(db, m.Version)
		if err != nil {
			return err
		}

		if applied {
			slog.Debug("Migration already applied", "version", m.Version, "name", m.Name)
			continue
		}

		slog.Info("Applying migration", "version", m.Version, "name", m.Name)

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(m.Up); err != nil {
			_ = tx.Rollback()
			slog.Error("Failed to apply migration", "version", m.Version, "error", err)
			return err
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING",
			m.Version,
		); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		slog.Info("Migration applied successfully", "version", m.Version, "name", m.Name)
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func isMigrationApplied(db *sql.DB, version int) (bool, error) {
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
		version,
	).Scan(&exists)
	return exists, err
}

func GetCurrentVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version)
	return version, err
}
