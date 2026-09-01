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

type Database func(context.Context) (bun.IDB, error)

type Store struct{ database Database }

type organizationRow struct {
	bun.BaseModel `bun:"table:organizations,alias:organization"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string     `bun:"name,notnull"`
	Slug          string     `bun:"slug,notnull"`
	Active        bool       `bun:"active,notnull"`
	DeletedAt     *time.Time `bun:"deleted_at"`
	Settings      string     `bun:"settings,nullzero,default:'{}'"`
}

func New(database Database) *Store {
	if database == nil {
		panic("organization tenancy postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) Create(ctx context.Context, input domain.CreateOrganization) (domain.Organization, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Organization{}, domain.OperationStats{}, err
	}
	row := organizationRow{Name: input.Name, Slug: input.Slug, Active: input.Active}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`platform.organizations`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if isIntegrityViolation(err) {
			return domain.Organization{}, stats, domain.ErrSlugConflict
		}
		return domain.Organization{}, stats, fmt.Errorf("organization tenancy postgres: create organization: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Update(ctx context.Context, input domain.UpdateOrganization) (domain.Organization, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Organization{}, domain.OperationStats{}, err
	}
	row := organizationRow{ID: input.ID, Name: input.Name, Slug: input.Slug, Active: input.Active}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).
		ModelTableExpr(`platform.organizations AS "organization"`).
		Column("name", "slug", "active").
		Where(`"organization".id = ?`, input.ID).
		Returning("*").
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if isIntegrityViolation(err) {
			return domain.Organization{}, stats, domain.ErrSlugConflict
		}
		return domain.Organization{}, stats, fmt.Errorf("organization tenancy postgres: update organization: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) FindByID(ctx context.Context, id int64, lock string) (domain.Organization, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Organization{}, false, domain.OperationStats{}, err
	}
	row := organizationRow{}
	query := db.NewSelect().Model(&row).
		ModelTableExpr(`platform.organizations AS "organization"`).
		Where(`"organization".id = ?`, id)
	if lock != "" {
		query = query.For(lock)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Organization{}, false, stats, nil
	}
	if err != nil {
		return domain.Organization{}, false, stats, fmt.Errorf("organization tenancy postgres: find organization by ID: %w", err)
	}
	return toDomain(row), true, stats, nil
}

func (s *Store) FindBySlug(ctx context.Context, slug string) (domain.Organization, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Organization{}, false, domain.OperationStats{}, err
	}
	row := organizationRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().Model(&row).
		ModelTableExpr(`platform.organizations AS "organization"`).
		Where(`"organization".slug = ?`, slug).
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Organization{}, false, stats, nil
	}
	if err != nil {
		return domain.Organization{}, false, stats, fmt.Errorf("organization tenancy postgres: find organization by slug: %w", err)
	}
	return toDomain(row), true, stats, nil
}

func (s *Store) List(ctx context.Context) ([]domain.Organization, domain.OperationStats, error) {
	return s.list(ctx, nil)
}

func (s *Store) ListByIDs(ctx context.Context, ids []int64) ([]domain.Organization, domain.OperationStats, error) {
	return s.list(ctx, ids)
}

func (s *Store) list(ctx context.Context, ids []int64) ([]domain.Organization, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []organizationRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`platform.organizations AS "organization"`).
		OrderExpr(`"organization".name ASC`)
	if ids != nil {
		query = query.Where(`"organization".id IN (?)`, bun.List(ids))
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	if err := query.Scan(ctx); err != nil {
		stats.StatementDuration = time.Since(started)
		return nil, stats, fmt.Errorf("organization tenancy postgres: list organizations: %w", err)
	}
	stats.StatementDuration = time.Since(started)
	result := make([]domain.Organization, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CountByIDs(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := db.NewSelect().Model((*organizationRow)(nil)).
		ModelTableExpr(`platform.organizations AS "organization"`).
		Where(`"organization".id IN (?)`, bun.List(ids)).
		Where(`"organization".deleted_at IS NULL`).
		Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("organization tenancy postgres: count organizations: %w", err)
	}
	return count, stats, nil
}

func (s *Store) CountNonDeletedSchools(ctx context.Context, organizationID int64) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := db.NewSelect().TableExpr(`platform.schools AS "school"`).
		Where(`"school".organization_id = ?`, organizationID).
		Where(`"school".deleted_at IS NULL`).
		Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("organization tenancy postgres: count organization schools: %w", err)
	}
	return count, stats, nil
}

func (s *Store) SoftDelete(ctx context.Context, id int64) (domain.OperationStats, error) {
	return s.setDeleted(ctx, id, true)
}

func (s *Store) Restore(ctx context.Context, id int64) (domain.OperationStats, error) {
	return s.setDeleted(ctx, id, false)
}

func (s *Store) setDeleted(ctx context.Context, id int64, deleted bool) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Model((*organizationRow)(nil)).
		ModelTableExpr(`platform.organizations AS "organization"`).
		Where(`"organization".id = ?`, id)
	if deleted {
		query = query.Set("deleted_at = NOW()").Where(`"organization".deleted_at IS NULL`)
	} else {
		query = query.Set("deleted_at = NULL").Where(`"organization".deleted_at IS NOT NULL`)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("organization tenancy postgres: change organization deletion state: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("organization tenancy postgres: count changed organizations: %w", err)
	}
	if rows != 1 {
		return stats, fmt.Errorf("organization tenancy postgres: expected one changed organization, got %d", rows)
	}
	stats.Rows = rows
	return stats, nil
}

func toDomain(row organizationRow) domain.Organization {
	return domain.Organization{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, Slug: row.Slug, Active: row.Active,
		DeletedAt: row.DeletedAt, Settings: row.Settings,
	}
}

func isIntegrityViolation(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.IntegrityViolation()
}
