package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// roomCategoryNameUniqueIndex is the unique index of migration 1.15.472.
const roomCategoryNameUniqueIndex = "uniq_facilities_room_categories_tenant_name"

type categoryRow struct {
	bun.BaseModel `bun:"table:room_categories,alias:category"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	Name          string    `bun:"name,notnull"`
	Color         string    `bun:"color,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// CategoryStore persists the room category catalog of the caller's school.
type CategoryStore struct{ database Database }

func NewCategoryStore(database Database) *CategoryStore {
	if database == nil {
		panic("facilities postgres: database runtime is required")
	}
	return &CategoryStore{database: database}
}

// List returns the catalog of the caller's school in name order.
func (s *CategoryStore) List(ctx context.Context) ([]facilities.RoomCategory, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := []categoryRow{}
	err = db.NewSelect().Model(&rows).
		ModelTableExpr(`facilities.room_categories AS "category"`).
		Where(`"category".tenant_id = ?`, tenantID).
		OrderExpr(`LOWER("category".name) ASC, "category".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("facilities postgres: list room categories: %w", err)
	}
	result := make([]facilities.RoomCategory, 0, len(rows))
	for _, row := range rows {
		result = append(result, toCategory(row))
	}
	return result, nil
}

// FindForUpdate locks one category of the caller's school.
func (s *CategoryStore) FindForUpdate(ctx context.Context, id int64) (facilities.RoomCategory, bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.RoomCategory{}, false, err
	}
	row := categoryRow{}
	err = db.NewSelect().Model(&row).
		ModelTableExpr(`facilities.room_categories AS "category"`).
		Where(`"category".id = ?`, id).
		Where(`"category".tenant_id = ?`, tenantID).
		For("UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return facilities.RoomCategory{}, false, nil
	}
	if err != nil {
		return facilities.RoomCategory{}, false, fmt.Errorf("facilities postgres: find room category: %w", err)
	}
	return toCategory(row), true, nil
}

func (s *CategoryStore) Create(ctx context.Context, input facilities.RoomCategoryInput) (facilities.RoomCategory, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.RoomCategory{}, err
	}
	row := categoryRow{TenantID: tenantID, Name: input.Name, Color: input.Color}
	err = db.NewInsert().Model(&row).ModelTableExpr(`facilities.room_categories`).Returning("*").Scan(ctx)
	if err != nil {
		return facilities.RoomCategory{}, fmt.Errorf("facilities postgres: create room category: %w", classifyCategoryWriteError(err))
	}
	return toCategory(row), nil
}

func (s *CategoryStore) Update(ctx context.Context, id int64, input facilities.RoomCategoryInput) (facilities.RoomCategory, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.RoomCategory{}, err
	}
	row := categoryRow{ID: id, TenantID: tenantID, Name: input.Name, Color: input.Color}
	err = db.NewUpdate().Model(&row).
		ModelTableExpr(`facilities.room_categories AS "category"`).
		Column("name", "color").
		Where(`"category".id = ?`, id).
		Where(`"category".tenant_id = ?`, tenantID).
		Returning("*").
		Scan(ctx)
	if err != nil {
		return facilities.RoomCategory{}, fmt.Errorf("facilities postgres: update room category: %w", classifyCategoryWriteError(err))
	}
	return toCategory(row), nil
}

func (s *CategoryStore) Delete(ctx context.Context, id int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*categoryRow)(nil)).
		ModelTableExpr(`facilities.room_categories AS "category"`).
		Where(`"category".id = ?`, id).
		Where(`"category".tenant_id = ?`, tenantID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("facilities postgres: delete room category: %w", err)
	}
	return nil
}

// RenameRooms moves the ordinary rooms of the old category name to the new
// one, so a renamed category keeps its rooms and their color.
func (s *CategoryStore) RenameRooms(ctx context.Context, from, to string) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().
		TableExpr(`facilities.rooms AS "room"`).
		Set(`category = ?`, to).
		Where(`"room".tenant_id = ?`, tenantID).
		Where(`NOT "room".is_system`).
		Where(`LOWER(BTRIM("room".category)) = LOWER(?)`, from).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("facilities postgres: rename room category on rooms: %w", err)
	}
	return nil
}

// CountRoomsByCategory counts the ordinary rooms per category, keyed the way
// roomSelect matches a room to its category: lower-case and trimmed.
func (s *CategoryStore) CountRoomsByCategory(ctx context.Context) (map[string]int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Name  string `bun:"name"`
		Rooms int    `bun:"rooms"`
	}
	err = db.NewSelect().
		TableExpr(`facilities.rooms AS "room"`).
		ColumnExpr(`LOWER(BTRIM("room".category)) AS name`).
		ColumnExpr(`COUNT(*) AS rooms`).
		Where(`"room".tenant_id = ?`, tenantID).
		Where(`NOT "room".is_system`).
		Where(`"room".category IS NOT NULL`).
		GroupExpr(`LOWER(BTRIM("room".category))`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("facilities postgres: count rooms per category: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.Name] = row.Rooms
	}
	return counts, nil
}

func toCategory(row categoryRow) facilities.RoomCategory {
	return facilities.RoomCategory{
		ID: row.ID, Name: row.Name, Color: row.Color,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func classifyCategoryWriteError(err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() &&
		postgresError.Field('n') == roomCategoryNameUniqueIndex {
		return facilities.ErrDuplicateRoomCategory
	}
	return err
}
