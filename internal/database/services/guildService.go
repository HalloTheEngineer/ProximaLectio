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
