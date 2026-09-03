package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
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

func (s *Store) Create(ctx context.Context, input domain.CreateRoom) (domain.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Room{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Room{}, domain.OperationStats{}, errors.New("facilities postgres: tenant is required")
	}
	row := roomRow{
		TenantID: tenantID, Name: input.Name, Building: input.Building, Floor: input.Floor,
		Capacity: input.Capacity, Category: input.Category, Color: input.Color, IsSystem: input.IsSystem,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`facilities.rooms`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Room{}, stats, fmt.Errorf("facilities postgres: create room: %w", classifyWriteError(err))
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Update(ctx context.Context, input domain.UpdateRoom) (domain.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Room{}, domain.OperationStats{}, err
	}
	row := roomRow{
		ID: input.ID, TenantID: tenantID, Name: input.Name, Building: input.Building,
		Floor: input.Floor, Capacity: input.Capacity, Category: input.Category, Color: input.Color,
	}
	query := db.NewUpdate().Model(&row).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Column("name", "building", "floor", "capacity", "category", "color").
		Where(`"room".id = ?`, input.ID)
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Room{}, stats, fmt.Errorf("facilities postgres: update room: %w", classifyWriteError(err))
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Delete(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*roomRow)(nil)).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Where(`"room".id = ?`, id)
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("facilities postgres: delete room: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("facilities postgres: count deleted rooms: %w", err)
	}
	if rows != 1 {
		return stats, fmt.Errorf("facilities postgres: expected one deleted room, got %d", rows)
	}
	stats.Rows = rows
	return stats, nil
}

func (s *Store) FindByID(ctx context.Context, id int64, lock string) (domain.Room, bool, domain.OperationStats, error) {
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
	if lock != "" {
		query = query.For(lock)
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

func (s *Store) FindByName(ctx context.Context, name string) (domain.Room, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Room{}, false, domain.OperationStats{}, err
	}
	row := roomRow{}
	query := roomSelect(db, &row).Where(`LOWER("room".name) = LOWER(?)`, name)
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Room{}, false, stats, nil
	}
	if err != nil {
		return domain.Room{}, false, stats, fmt.Errorf("facilities postgres: find room by name: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) FindToilet(ctx context.Context, excludeRoomID int64) (domain.Room, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Room{}, false, domain.OperationStats{}, err
	}
	row := roomRow{}
	query := roomSelect(db, &row).
		Where(`"room".name IN (?, ?)`, domain.WCRoomName, domain.WCRoomAliasName).
		OrderExpr(`CASE "room".name WHEN ? THEN 0 ELSE 1 END`, domain.WCRoomName).
		Limit(1)
	if excludeRoomID > 0 {
		query = query.Where(`"room".id <> ?`, excludeRoomID)
	}
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Room{}, false, stats, nil
	}
	if err != nil {
		return domain.Room{}, false, stats, fmt.Errorf("facilities postgres: find toilet room: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) List(ctx context.Context, filter domain.RoomFilter) ([]domain.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []roomRow{}
	query := roomSelect(db, &rows).OrderExpr(`"room".name ASC, "room".id ASC`)
	if tenantID != 0 {
		query = query.Where(`"room".tenant_id = ?`, tenantID)
	}
	if filter.Name != nil {
		query = query.Where(`LOWER("room".name) = LOWER(?)`, *filter.Name)
	}
	if filter.NameContains != nil {
		query = query.Where(`"room".name ILIKE ?`, "%"+*filter.NameContains+"%")
	}
	if filter.Building != nil {
		query = query.Where(`LOWER("room".building) = LOWER(?)`, *filter.Building)
	}
	if filter.BuildingContains != nil {
		query = query.Where(`"room".building ILIKE ?`, "%"+*filter.BuildingContains+"%")
	}
	if filter.Floor != nil {
		query = query.Where(`"room".floor = ?`, *filter.Floor)
	}
	if filter.Category != nil {
		query = query.Where(`LOWER("room".category) = LOWER(?)`, *filter.Category)
	}
	if filter.MinimumCapacity != nil {
		query = query.Where(`("room".capacity IS NULL OR "room".capacity <= 0 OR "room".capacity >= ?)`, *filter.MinimumCapacity)
	}
	if filter.MaximumCapacity != nil {
		query = query.Where(`"room".capacity <= ?`, *filter.MaximumCapacity)
	}
	if filter.Search != nil {
		if *filter.Search == "" {
			return []domain.Room{}, domain.OperationStats{}, nil
		}
		pattern := "%" + *filter.Search + "%"
		query = query.Where(`("room".name ILIKE ? OR "room".building ILIKE ? OR "room".category ILIKE ?)`, pattern, pattern, pattern)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
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

func classifyWriteError(err error) error {
	var postgresError pgdriver.Error
	if !errors.As(err, &postgresError) || postgresError.Field('C') != "23505" {
		return err
	}
	switch postgresError.Field('n') {
	case domain.RoomNameUniqueConstraint:
		return domain.ErrDuplicate
	case domain.RoomColorUniqueConstraint:
		return domain.ErrColorAlreadyInUse
	case domain.RoomWCAliasUniqueConstraint:
		return domain.ErrDuplicateToilet
	default:
		return err
	}
}
