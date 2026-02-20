package services

import (
	"context"
	"database/sql"
)

type GuildService struct {
	db *sql.DB
}

func NewGuildService(db *sql.DB) *GuildService {
	return &GuildService{db: db}
}

func (s *GuildService) AllowChannel(ctx context.Context, guildID, channelID string) error {
	query := `INSERT INTO allowed_notification_channels (guild_id, channel_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, guildID, channelID)
	return err
}

func (s *GuildService) RevokeChannel(ctx context.Context, guildID, channelID string) error {
	query := `DELETE FROM allowed_notification_channels WHERE guild_id = $1 AND channel_id = $2`
	_, err := s.db.ExecContext(ctx, query, guildID, channelID)
	return err
}

func (s *GuildService) IsChannelAllowed(ctx context.Context, guildID, channelID string) (bool, error) {
	var allowed bool
	query := `SELECT EXISTS(SELECT 1 FROM allowed_notification_channels WHERE guild_id = $1 AND channel_id = $2)`
	err := s.db.QueryRowContext(ctx, query, guildID, channelID).Scan(&allowed)
	return allowed, err
}

type GuildMember struct {
	ID          string
	DisplayName string
}

func (s *GuildService) GetGuildMembers(ctx context.Context, guildID string) ([]GuildMember, error) {
	query := `SELECT u.id, COALESCE(u.display_name, u.username) as display_name 
			  FROM users u JOIN guild_members gm ON u.id = gm.user_id 
			  WHERE gm.guild_id = $1 ORDER BY display_name ASC`
	rows, err := s.db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []GuildMember
	for rows.Next() {
		var m GuildMember
		if err := rows.Scan(&m.ID, &m.DisplayName); err == nil {
			members = append(members, m)
		}
	}
	return members, nil
}

func (s *GuildService) GetGuildMemberByDiscordID(ctx context.Context, guildID, discordID string) (*GuildMember, error) {
	query := `SELECT u.id, COALESCE(u.display_name, u.username) as display_name 
			  FROM users u JOIN guild_members gm ON u.id = gm.user_id 
			  WHERE gm.guild_id = $1 AND u.id = $2`
	var m GuildMember
	err := s.db.QueryRowContext(ctx, query, guildID, discordID).Scan(&m.ID, &m.DisplayName)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
