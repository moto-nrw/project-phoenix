package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

type schoolRow struct {
	bun.BaseModel  `bun:"table:schools,alias:school"`
	ID             int64      `bun:"id,pk,autoincrement"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	OrganizationID int64      `bun:"organization_id,notnull"`
	Name           string     `bun:"name,notnull"`
	Slug           string     `bun:"slug,notnull"`
	Subdomain      string     `bun:"subdomain,notnull"`
	Active         bool       `bun:"active,notnull"`
	Hidden         bool       `bun:"hidden,notnull"`
	DeletedAt      *time.Time `bun:"deleted_at"`
	Settings       string     `bun:"settings,nullzero,default:'{}'"`
	Address        string     `bun:"address"`
	City           string     `bun:"city"`
	Zip            string     `bun:"zip"`
	Phone          string     `bun:"phone"`
	Email          string     `bun:"email"`
	DevicePinHash  string     `bun:"device_pin_hash"`
}

func (s *Store) CreateSchool(ctx context.Context, input domain.CreateSchool) (domain.School, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.School{}, domain.OperationStats{}, err
	}
	row := schoolRow{
		OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug,
		Subdomain: input.Subdomain, Active: input.Active, Hidden: input.Hidden,
		Settings: input.Settings, Address: input.Address, City: input.City, Zip: input.Zip,
		Phone: input.Phone, Email: input.Email, DevicePinHash: input.DevicePinHash,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`platform.schools`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.School{}, stats, wrapSchoolWriteError("create", err)
	}
	stats.Rows = 1
	return toSchoolDomain(row), stats, nil
}

func (s *Store) UpdateSchool(ctx context.Context, input domain.UpdateSchool) (domain.School, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.School{}, domain.OperationStats{}, err
	}
	row := schoolRow{
		ID: input.ID, OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug,
		Subdomain: input.Subdomain, Active: input.Active, Hidden: input.Hidden,
		Settings: input.Settings, Address: input.Address, City: input.City, Zip: input.Zip,
		Phone: input.Phone, Email: input.Email, DevicePinHash: input.DevicePinHash,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).
		ModelTableExpr(`platform.schools AS "school"`).
		Column("organization_id", "name", "slug", "subdomain", "address", "city", "zip", "phone", "email", "active", "hidden", "settings", "device_pin_hash").
		Where(`"school".id = ?`, input.ID).
		Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.School{}, stats, wrapSchoolWriteError("update", err)
	}
	stats.Rows = 1
	return toSchoolDomain(row), stats, nil
}

func (s *Store) FindSchoolByID(ctx context.Context, id int64, lock string) (domain.School, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.School{}, false, domain.OperationStats{}, err
	}
	row := &schoolRow{}
	return scanSchool(ctx, row, schoolSelect(db, row).Where(`"school".id = ?`, id), lock)
}

func (s *Store) FindSchoolBySlug(ctx context.Context, slug string) (domain.School, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.School{}, false, domain.OperationStats{}, err
	}
	row := &schoolRow{}
	return scanSchool(ctx, row, schoolSelect(db, row).
		Where(`"school".slug = ?`, slug).
		Where(`"school".deleted_at IS NULL`), "")
}

func (s *Store) FindSchoolByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (domain.School, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.School{}, false, domain.OperationStats{}, err
	}
	row := &schoolRow{}
	return scanSchool(ctx, row, schoolSelect(db, row).
		Where(`"school".organization_id = ?`, organizationID).
		Where(`"school".slug = ?`, slug), "")
}

func (s *Store) FindSchoolBySubdomain(ctx context.Context, subdomain string) (domain.School, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.School{}, false, domain.OperationStats{}, err
	}
	row := &schoolRow{}
	return scanSchool(ctx, row, schoolSelect(db, row).Where(`"school".subdomain = ?`, subdomain), "")
}

func scanSchool(ctx context.Context, row *schoolRow, query *bun.SelectQuery, lock string) (domain.School, bool, domain.OperationStats, error) {
	if lock != "" {
		query = query.For(lock)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.School{}, false, stats, nil
	}
	if err != nil {
		return domain.School{}, false, stats, fmt.Errorf("organization tenancy postgres: find school: %w", err)
	}
	return toSchoolDomain(*row), true, stats, nil
}

func (s *Store) ListSchools(ctx context.Context, ids []int64, organizationID *int64, state string) ([]domain.School, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []schoolRow{}
	query := schoolSelect(db, &rows)
	if ids != nil {
		query = query.Where(`"school".id IN (?)`, bun.List(ids))
	}
	if organizationID != nil {
		query = query.Where(`"school".organization_id = ?`, *organizationID)
	}
	switch state {
	case "non_deleted":
		query = query.Where(`"school".deleted_at IS NULL`)
	case "active":
		query = query.Where(`"school".active = TRUE`).Where(`"school".deleted_at IS NULL`)
	case "public":
		query = query.Where(`"school".active = TRUE`).Where(`"school".hidden = FALSE`).Where(`"school".deleted_at IS NULL`)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.OrderExpr(`"school".name ASC`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("organization tenancy postgres: list schools: %w", err)
	}
	result := make([]domain.School, 0, len(rows))
	for _, row := range rows {
		result = append(result, toSchoolDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CountSchoolsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := db.NewSelect().Model((*schoolRow)(nil)).
		ModelTableExpr(`platform.schools AS "school"`).
		Where(`"school".id IN (?)`, bun.List(ids)).
		Where(`"school".deleted_at IS NULL`).Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("organization tenancy postgres: count schools: %w", err)
	}
	return count, stats, nil
}

func (s *Store) SetSchoolDeleted(ctx context.Context, id int64, deleted bool) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Model((*schoolRow)(nil)).
		ModelTableExpr(`platform.schools AS "school"`).Where(`"school".id = ?`, id)
	if deleted {
		query = query.Set("deleted_at = NOW()").Where(`"school".deleted_at IS NULL`)
	} else {
		query = query.Set("deleted_at = NULL").Where(`"school".deleted_at IS NOT NULL`)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("organization tenancy postgres: change school deletion state: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("organization tenancy postgres: count changed schools: %w", err)
	}
	if rows != 1 {
		return stats, fmt.Errorf("organization tenancy postgres: expected one changed school, got %d", rows)
	}
	stats.Rows = rows
	return stats, nil
}

func schoolSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).
		ModelTableExpr(`platform.schools AS "school"`)
}

func toSchoolDomain(row schoolRow) domain.School {
	return domain.School{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		OrganizationID: row.OrganizationID, Name: row.Name, Slug: row.Slug, Subdomain: row.Subdomain,
		Active: row.Active, Hidden: row.Hidden, DeletedAt: row.DeletedAt, Settings: row.Settings,
		Address: row.Address, City: row.City, Zip: row.Zip, Phone: row.Phone, Email: row.Email,
		DevicePinHash: row.DevicePinHash,
	}
}

func wrapSchoolWriteError(operation string, err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() {
		switch postgresError.Field('n') {
		case "schools_subdomain_key":
			return domain.ErrSchoolDomainConflict
		case "schools_organization_id_slug_key":
			return domain.ErrSchoolSlugConflict
		}
	}
	return fmt.Errorf("organization tenancy postgres: %s school: %w", operation, err)
}
