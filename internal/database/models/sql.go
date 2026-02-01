package models

func GetSQLCreationQueries() []string {
	return []string{
		`
			CREATE TABLE IF NOT EXISTS schools (
			    tenant_id INT PRIMARY KEY,
			    school_id INT,
				display_name VARCHAR(255) NOT NULL,
				login_name VARCHAR(255) NOT NULL, 
				server VARCHAR(255) NOT NULL,
				address TEXT,
				last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`,
		`
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
		`,
		`
			CREATE TABLE IF NOT EXISTS guilds (
				id VARCHAR(64) PRIMARY KEY, 
				name VARCHAR(255) NOT NULL, 
				joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`,
		`
			CREATE TABLE IF NOT EXISTS allowed_notification_channels (
				guild_id VARCHAR(64) REFERENCES guilds(id) ON DELETE CASCADE,
				channel_id VARCHAR(64) NOT NULL,
				PRIMARY KEY (guild_id, channel_id)
			);
		`,
		`
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
		`,
		`
			CREATE TABLE IF NOT EXISTS absences (
				id SERIAL PRIMARY KEY,
				untis_id INTEGER NOT NULL,
				user_id VARCHAR(64) REFERENCES users(id) ON DELETE CASCADE,
				start_date DATE NOT NULL,
				end_date DATE NOT NULL,
				start_time INTEGER,
				end_time INTEGER,
				reason TEXT,
				is_excused BOOLEAN DEFAULT FALSE,
				UNIQUE(user_id, untis_id)
			);
		`,
	}
}
