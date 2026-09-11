package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, error)

type Store struct{ database Database }

type groupRow struct {
	bun.BaseModel `bun:"table:groups,alias:group"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string    `bun:"name,notnull"`
	RoomID        *int64    `bun:"room_id"`
}

func New(database Database) *Store {
	if database == nil {
		panic("school structure postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) FindByID(ctx context.Context, id int64) (domain.Group, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Group{}, false, domain.OperationStats{}, err
	}
	row := groupRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = groupSelect(db, &row).Where(`"group".id = ?`, id).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Group{}, false, stats, nil
	}
	if err != nil {
		return domain.Group{}, false, stats, fmt.Errorf("school structure postgres: find group: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) ListByIDs(ctx context.Context, ids []int64) ([]domain.Group, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []groupRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = groupSelect(db, &rows).
		Where(`"group".id IN (?)`, bun.List(ids)).
		OrderExpr(`"group".name ASC, "group".id ASC`).
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("school structure postgres: list groups: %w", err)
	}
	result := make([]domain.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func groupSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`education.groups AS "group"`)
}

func toDomain(row groupRow) domain.Group {
	return domain.Group{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, RoomID: row.RoomID,
	}
}
