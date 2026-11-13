package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/facilities"

	"github.com/moto-ogs/project-phoenix/modules/facilities/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

type roomRow struct {
	bun.BaseModel `bun:"table:rooms,alias:room"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string     `bun:"name,notnull"`
	Building      string     `bun:"building"`
	Floor         *int       `bun:"floor"`
	Capacity      *int       `bun:"capacity"`
	Category      *string    `bun:"category"`
	Color         *string    `bun:"color"`
	IsSystem      bool       `bun:"is_system,notnull,default:false"`
	IsOpenRoom    bool       `bun:"is_open_room,notnull,default:false"`
	ArchivedAt    *time.Time `bun:"archived_at"`
}

func New(database Database) *Store {
	if database == nil {
		panic("facilities postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) Create(ctx context.Context, input facilities.CreateRoom) (facilities.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.Room{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return facilities.Room{}, domain.OperationStats{}, errors.New("facilities postgres: tenant is required")
	}
	row := roomRow{
		TenantID: tenantID, Name: input.Name, Building: input.Building, Floor: input.Floor,
		Capacity: input.Capacity, Category: input.Category, Color: input.Color, IsSystem: input.IsSystem,
		IsOpenRoom: input.IsOpenRoom,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`facilities.rooms`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return facilities.Room{}, stats, fmt.Errorf("facilities postgres: create room: %w", classifyWriteError(err))
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Update(ctx context.Context, input facilities.UpdateRoom) (facilities.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.Room{}, domain.OperationStats{}, err
	}
	row := roomRow{
		ID: input.ID, TenantID: tenantID, Name: input.Name, Building: input.Building,
		Floor: input.Floor, Capacity: input.Capacity, Category: input.Category, Color: input.Color,
	}
	// The release column joins the SET list only when the caller expressed an
	// opinion. Writing it unconditionally would let any room edit that omits
	// the field revoke a standing release — the room form posts a partial
	// payload, so that would happen on the first colour change.
	columns := []string{"name", "building", "floor", "capacity", "category", "color"}
	if input.IsOpenRoom != nil {
		row.IsOpenRoom = *input.IsOpenRoom
		columns = append(columns, "is_open_room")
	}
	// A room renamed to a toilet name becomes the kiosk's toilet room, a
	// system room like the provisioned one: is_system is the one mark every
	// protection reads (#6845).
	query := db.NewUpdate().Model(&row).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Column(columns...).
		Set(`is_system = "room".is_system OR ? IN (?, ?)`, input.Name, domain.WCRoomName, domain.WCRoomAliasName).
		Where(`"room".id = ?`, input.ID).
		Where(`"room".tenant_id = ?`, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return facilities.Room{}, stats, fmt.Errorf("facilities postgres: update room: %w", classifyWriteError(err))
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

// Delete removes the room unless another row still references it. The
// statement runs inside a savepoint: a foreign-key violation rolls back only
// the delete, reports deleted=false and leaves the caller's transaction
// usable, so the room can be archived instead (#6845). Every reference to
// facilities.rooms is a NO ACTION key, so nothing is ever nulled.
func (s *Store) Delete(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 3}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()
	if _, err := db.ExecContext(ctx, `SAVEPOINT facilities_room_delete`); err != nil {
		return false, stats, fmt.Errorf("facilities postgres: open room delete savepoint: %w", err)
	}
	result, deleteErr := db.NewDelete().Model((*roomRow)(nil)).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Where(`"room".id = ?`, id).
		Where(`"room".tenant_id = ?`, tenantID).
		Exec(ctx)
	if deleteErr != nil {
		if !isForeignKeyViolation(deleteErr) {
			return false, stats, fmt.Errorf("facilities postgres: delete room: %w", deleteErr)
		}
		stats.Queries++
		if _, err := db.ExecContext(ctx, `ROLLBACK TO SAVEPOINT facilities_room_delete`); err != nil {
			return false, stats, fmt.Errorf("facilities postgres: roll back room delete: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `RELEASE SAVEPOINT facilities_room_delete`); err != nil {
		return false, stats, fmt.Errorf("facilities postgres: release room delete savepoint: %w", err)
	}
	if deleteErr != nil {
		return false, stats, nil
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("facilities postgres: count deleted rooms: %w", err)
	}
	if rows != 1 {
		return false, stats, fmt.Errorf("facilities postgres: expected one deleted room, got %d", rows)
	}
	stats.Rows = rows
	return true, stats, nil
}

// Archive keeps the room for the history that names it and takes it out of
// every list (#6845).
func (s *Store) Archive(ctx context.Context, id int64) (facilities.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.Room{}, domain.OperationStats{}, err
	}
	row := roomRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Set(`archived_at = COALESCE("room".archived_at, now())`).
		Where(`"room".id = ?`, id).
		Where(`"room".tenant_id = ?`, tenantID).
		Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return facilities.Room{}, stats, fmt.Errorf("facilities postgres: archive room: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func isForeignKeyViolation(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23503"
}

func (s *Store) FindByID(ctx context.Context, id int64, lock string) (facilities.Room, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.Room{}, false, domain.OperationStats{}, err
	}
	row := roomRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := roomSelect(db, &row).
		Where(`"room".id = ?`, id).
		Where(`"room".tenant_id = ?`, tenantID)
	if lock != "" {
		query = query.For(lock)
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return facilities.Room{}, false, stats, nil
	}
	if err != nil {
		return facilities.Room{}, false, stats, fmt.Errorf("facilities postgres: find room: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) FindByName(ctx context.Context, name string) (facilities.Room, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.Room{}, false, domain.OperationStats{}, err
	}
	row := roomRow{}
	query := roomSelect(db, &row).
		Where(`LOWER("room".name) = LOWER(?)`, name).
		Where(`"room".tenant_id = ?`, tenantID).
		Where(`"room".archived_at IS NULL`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return facilities.Room{}, false, stats, nil
	}
	if err != nil {
		return facilities.Room{}, false, stats, fmt.Errorf("facilities postgres: find room by name: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) FindToilet(ctx context.Context, excludeRoomID int64) (facilities.Room, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return facilities.Room{}, false, domain.OperationStats{}, err
	}
	row := roomRow{}
	query := roomSelect(db, &row).
		Where(`"room".name IN (?, ?)`, domain.WCRoomName, domain.WCRoomAliasName).
		Where(`"room".tenant_id = ?`, tenantID).
		Where(`"room".archived_at IS NULL`).
		OrderExpr(`CASE "room".name WHEN ? THEN 0 ELSE 1 END`, domain.WCRoomName).
		Limit(1)
	if excludeRoomID > 0 {
		query = query.Where(`"room".id <> ?`, excludeRoomID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return facilities.Room{}, false, stats, nil
	}
	if err != nil {
		return facilities.Room{}, false, stats, fmt.Errorf("facilities postgres: find toilet room: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) List(ctx context.Context, filter facilities.RoomFilter) ([]facilities.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []roomRow{}
	query := roomSelect(db, &rows).
		Where(`"room".tenant_id = ?`, tenantID).
		OrderExpr(`"room".name ASC, "room".id ASC`)
	query, empty := applyRoomListFilter(query, filter)
	if empty {
		return []facilities.Room{}, domain.OperationStats{}, nil
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("facilities postgres: list rooms: %w", err)
	}
	result := make([]facilities.Room, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

// pagedRoomRow carries the filtered total next to each row of the page.
type pagedRoomRow struct {
	roomRow `bun:",extend"`
	Total   int `bun:"room_total,scanonly"`
}

// ListPage reads the page and the filtered total in one statement, so both
// come from one READ COMMITTED snapshot (#5888, decision E2: one early shared
// read may replace the former separate count and page reads; the listing
// takes no lock). Only a page past the end, which carries no row to read the
// total from, counts separately.
func (s *Store) ListPage(ctx context.Context, filter facilities.RoomFilter, offset, limit int) ([]facilities.Room, int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, domain.OperationStats{}, err
	}
	rows := []pagedRoomRow{}
	query := roomSelect(db, &rows).
		ColumnExpr(`?TableColumns`).
		ColumnExpr(`COUNT(*) OVER () AS room_total`).
		Where(`"room".tenant_id = ?`, tenantID).
		OrderExpr(`"room".name ASC, "room".id ASC`)
	query, empty := applyRoomListFilter(query, filter)
	if empty {
		return []facilities.Room{}, 0, domain.OperationStats{}, nil
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	if err := query.Offset(int64(offset)).Limit(int64(limit)).Scan(ctx); err != nil {
		stats.StatementDuration = time.Since(started)
		return nil, 0, stats, fmt.Errorf("facilities postgres: list room page: %w", err)
	}
	total := 0
	if len(rows) > 0 {
		total = rows[0].Total
	} else if offset > 0 {
		countQuery, _ := applyRoomListFilter(db.NewSelect().
			TableExpr(`facilities.rooms AS "room"`).
			ColumnExpr("COUNT(*)").
			Where(`"room".tenant_id = ?`, tenantID), filter)
		stats.Queries++
		if err := countQuery.Scan(ctx, &total); err != nil {
			stats.StatementDuration = time.Since(started)
			return nil, 0, stats, fmt.Errorf("facilities postgres: count rooms: %w", err)
		}
	}
	stats.StatementDuration = time.Since(started)
	result := make([]facilities.Room, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row.roomRow))
	}
	stats.Rows = int64(len(result))
	return result, total, stats, nil
}

func applyRoomListFilter(query *bun.SelectQuery, filter facilities.RoomFilter) (*bun.SelectQuery, bool) {
	// Archived rooms only keep their history (#6845): no list or selection
	// offers them.
	query = query.Where(`"room".archived_at IS NULL`)
	if filter.Name != nil {
		query = query.Where(`LOWER("room".name) = LOWER(?)`, *filter.Name)
	}
	if filter.NameContains != nil {
		query = query.Where(`public.f_unaccent("room".name) ILIKE public.f_unaccent(?)`, "%"+*filter.NameContains+"%")
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
	if filter.IsOpenRoom != nil {
		query = query.Where(`"room".is_open_room = ?`, *filter.IsOpenRoom)
	}
	if filter.Search != nil {
		if *filter.Search == "" {
			return query, true
		}
		pattern := "%" + *filter.Search + "%"
		query = query.Where(`(public.f_unaccent("room".name) ILIKE public.f_unaccent(?) OR "room".building ILIKE ? OR "room".category ILIKE ?)`, pattern, pattern, pattern)
	}
	if filter.ExcludeSystem {
		// Toilet rooms are never staff-visible: they are kiosk infrastructure
		// with no planning meaning. The Schulhof stays selectable whether or
		// not it is released: schools plan blocks there, and revoking the
		// open-room release must not remove it from the planner (ADR 0019).
		query = query.Where(`"room".name NOT IN (?, ?) AND (NOT "room".is_system OR "room".name = ?)`, domain.WCRoomName, domain.WCRoomAliasName, domain.SchulhofRoomName)
	}
	return query, false
}

func (s *Store) ListByIDs(ctx context.Context, ids []int64) ([]facilities.Room, domain.OperationStats, error) {
	return s.listByIDs(ctx, ids, "")
}

// LockByIDs resolves the rooms while holding key-share locks until the
// caller's transaction ends. A concurrent DELETE therefore cannot invalidate
// a room reference between restore validation and its INSERT.
func (s *Store) LockByIDs(ctx context.Context, ids []int64) ([]facilities.Room, domain.OperationStats, error) {
	return s.listByIDs(ctx, ids, "KEY SHARE")
}

func (s *Store) listByIDs(ctx context.Context, ids []int64, lock string) ([]facilities.Room, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []roomRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := roomSelect(db, &rows).
		Where(`"room".id IN (?)`, bun.List(ids)).
		Where(`"room".tenant_id = ?`, tenantID).
		OrderExpr(`"room".name ASC, "room".id ASC`)
	if lock != "" {
		query = query.For(lock)
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("facilities postgres: list rooms: %w", err)
	}
	result := make([]facilities.Room, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func roomSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`facilities.rooms AS "room"`)
}

func toDomain(row roomRow) facilities.Room {
	return facilities.Room{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, Building: row.Building, Floor: row.Floor, Capacity: row.Capacity,
		Category: row.Category, Color: row.Color, IsSystem: row.IsSystem,
		IsOpenRoom: row.IsOpenRoom, ArchivedAt: row.ArchivedAt,
	}
}

func classifyWriteError(err error) error {
	var postgresError pgdriver.Error
	if !errors.As(err, &postgresError) || !postgresError.IntegrityViolation() {
		return err
	}
	switch postgresError.Field('n') {
	case domain.RoomNameUniqueConstraint:
		return facilities.ErrDuplicateRoom
	case domain.RoomColorUniqueConstraint:
		return facilities.ErrRoomColorAlreadyInUse
	case domain.RoomWCAliasUniqueConstraint:
		return facilities.ErrDuplicateToiletRoom
	default:
		return err
	}
}
