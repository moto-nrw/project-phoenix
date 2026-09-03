package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

type roomRow struct {
	bun.BaseModel `bun:"table:rooms,alias:room"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string    `bun:"name,notnull"`
	Building      string    `bun:"building"`
	Floor         *int      `bun:"floor"`
	Capacity      *int      `bun:"capacity"`
	Category      *string   `bun:"category"`
	Color         *string   `bun:"color"`
	IsSystem      bool      `bun:"is_system,notnull,default:false"`
}

func New(database Database) *Store {
	if database == nil {
		panic("facilities postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) FindByID(ctx context.Context, id int64) (domain.Room, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Room{}, false, domain.OperationStats{}, err
	}
	row := roomRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := roomSelect(db, &row).Where(`"room".id = ?`, id)
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Room{}, false, stats, nil
	}
	if err != nil {
		return domain.Room{}, false, stats, fmt.Errorf("facilities postgres: find room: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) ListByIDs(ctx context.Context, ids []int64) ([]domain.Room, domain.OperationStats, error) {
	return s.listByIDs(ctx, ids, "")
}

// LockByIDs resolves the rooms while holding key-share locks until the
// caller's transaction ends. A concurrent DELETE therefore cannot invalidate
// a room reference between restore validation and its INSERT.
func (s *Store) LockByIDs(ctx context.Context, ids []int64) ([]domain.Room, domain.OperationStats, error) {
	return s.listByIDs(ctx, ids, "KEY SHARE")
}

func (s *Store) listByIDs(ctx context.Context, ids []int64, lock string) ([]domain.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []roomRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := roomSelect(db, &rows).
		Where(`"room".id IN (?)`, bun.List(ids)).
		OrderExpr(`"room".name ASC, "room".id ASC`)
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	if lock != "" {
		query = query.For(lock)
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("facilities postgres: list rooms: %w", err)
	}
	result := make([]domain.Room, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func roomSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`facilities.rooms AS "room"`)
}

func toDomain(row roomRow) domain.Room {
	return domain.Room{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, Building: row.Building, Floor: row.Floor, Capacity: row.Capacity,
		Category: row.Category, Color: row.Color, IsSystem: row.IsSystem,
	}
}
