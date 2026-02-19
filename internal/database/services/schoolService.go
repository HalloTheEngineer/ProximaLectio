package services

import (
	"context"
	"database/sql"
	"proximaLectio/internal/database/models/untis"
)

type SchoolService struct {
	db *sql.DB
}

func NewSchoolService(db *sql.DB) *SchoolService {
	return &SchoolService{db: db}
}

func (s *SchoolService) UpsertSchool(ctx context.Context, tenantID, schoolID int, loginName, displayName, server, address string) error {
	query := `
		INSERT INTO schools (tenant_id, school_id, display_name, login_name, server, address, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			login_name = EXCLUDED.login_name,
			server = EXCLUDED.server,
			address = EXCLUDED.address,
			last_updated = NOW()`
	_, err := s.db.ExecContext(ctx, query, tenantID, schoolID, displayName, loginName, server, address)
	return err
}

func (s *SchoolService) GetSchool(ctx context.Context, tenantID string) (*untis.School, error) {
	var school untis.School
	query := `SELECT tenant_id, school_id, display_name, login_name, server, address, last_updated 
              FROM schools WHERE tenant_id = $1`
	err := s.db.QueryRowContext(ctx, query, tenantID).Scan(
		&school.TenantId, &school.SchoolId, &school.DisplayName, &school.LoginName, &school.Server, &school.Address, &school.LastUpdated,
	)
	if err != nil {
		return nil, err
	}
	return &school, nil
}
