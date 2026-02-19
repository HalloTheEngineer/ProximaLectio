package services

import (
	"context"
	"database/sql"
	"proximaLectio/internal/crypto"
	"proximaLectio/internal/database/models/untis"
)

type UserService struct {
	db        *sql.DB
	encryptor *crypto.Encryptor
	hooks     []NotificationHandler
}

func NewUserService(db *sql.DB, encryptor *crypto.Encryptor) *UserService {
	return &UserService{db: db, encryptor: encryptor}
}

func (s *UserService) RegisterNotificationHook(handler NotificationHandler) {
	s.hooks = append(s.hooks, handler)
}

func (s *UserService) notifyHooks(ctx context.Context, userID string, target untis.NotificationTarget, subject, date, cType, old, new string) {
	for _, hook := range s.hooks {
		hook(ctx, userID, target, subject, date, cType, old, new)
	}
}

func (s *UserService) GetUser(ctx context.Context, id string) (*untis.User, error) {
	var u untis.User
	var target, address sql.NullString
	var encryptedPassword string
	query := `SELECT id, username, display_name, email, untis_school_tenant_id, untis_user, untis_password, untis_person_id, theme_id, 
                      notifications_enabled, notification_target, notification_address 
               FROM users WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &encryptedPassword, &u.UntisPersonID, &u.ThemeID,
		&u.NotificationsEnabled, &target, &address,
	)
	if err != nil {
		return nil, err
	}

	u.UntisPassword, err = s.encryptor.Decrypt(encryptedPassword)
	if err != nil {
		u.UntisPassword = encryptedPassword
	}

	u.NotificationTarget = target.String
	u.NotificationAddress = address.String
	return &u, nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]*untis.User, error) {
	query := `SELECT id, username, display_name, email, untis_school_tenant_id, untis_user, untis_password, untis_person_id, theme_id, 
                      notifications_enabled, notification_target, notification_address FROM users`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*untis.User
	for rows.Next() {
		var u untis.User
		var target, address sql.NullString
		var encryptedPassword string
		err := rows.Scan(
			&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.UntisSchoolID, &u.UntisUser, &encryptedPassword, &u.UntisPersonID, &u.ThemeID,
			&u.NotificationsEnabled, &target, &address,
		)
		if err != nil {
			continue
		}
		u.UntisPassword, _ = s.encryptor.Decrypt(encryptedPassword)
		u.NotificationTarget = target.String
		u.NotificationAddress = address.String
		users = append(users, &u)
	}
	return users, nil
}

func (s *UserService) UserExists(ctx context.Context, id string) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (s *UserService) SetNotificationConfig(ctx context.Context, id string, enabled bool, target string, address string) error {
	query := `UPDATE users SET notifications_enabled = $1, notification_target = $2, notification_address = $3 WHERE id = $4`
	_, err := s.db.ExecContext(ctx, query, enabled, target, address, id)
	return err
}

func (s *UserService) ToggleNotifications(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE users SET notifications_enabled = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, enabled, id)
	return err
}

func (s *UserService) DeleteUser(ctx context.Context, id string) bool {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return false
	}
	aff, err := res.RowsAffected()
	if err != nil {
		return false
	}
	return aff > 0
}
