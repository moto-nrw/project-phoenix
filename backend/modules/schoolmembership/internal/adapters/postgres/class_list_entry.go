package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/displayorder"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

// classListEntryNameIndex is the unique index behind "one child once per
// class" (tenant, LOWER(BTRIM(first/last/class))); pinned by the migration
// that created it.
const classListEntryNameIndex = "uniq_class_list_entries_name_class"

type classListEntryRow struct {
	bun.BaseModel `bun:"table:class_list_entries,alias:class_list_entry"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	FirstName     string    `bun:"first_name,notnull"`
	LastName      string    `bun:"last_name,notnull"`
	SchoolClass   string    `bun:"school_class,notnull"`
	CreatedBy     *int64    `bun:"created_by"`
}

func (s *Store) FindClassListEntry(ctx context.Context, id int64, lock string) (schoolmembership.ClassListEntry, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.ClassListEntry{}, false, domain.OperationStats{}, err
	}
	row := &classListEntryRow{}
	query := withTenant(classListEntrySelect(db, row).Where(`"class_list_entry".id = ?`, id), "class_list_entry", scope)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find class list entry")
	if err != nil || !found {
		return schoolmembership.ClassListEntry{}, found, stats, err
	}
	return classListEntryToPublic(*row), true, stats, nil
}

// ListClassListEntries orders by case-folded last name, first name and ID:
// the order the legacy class lookup produced, stable across equal names.
func (s *Store) ListClassListEntries(ctx context.Context, filter schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []classListEntryRow{}
	query := withTenant(classListEntrySelect(db, &rows), "class_list_entry", scope)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []schoolmembership.ClassListEntry{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"class_list_entry".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.FirstName != "" {
		query = query.Where(`LOWER(BTRIM("class_list_entry".first_name)) = LOWER(BTRIM(?))`, filter.FirstName)
	}
	if filter.LastName != "" {
		query = query.Where(`LOWER(BTRIM("class_list_entry".last_name)) = LOWER(BTRIM(?))`, filter.LastName)
	}
	if filter.SchoolClass != "" {
		query = query.Where(`LOWER(BTRIM("class_list_entry".school_class)) = LOWER(BTRIM(?))`, filter.SchoolClass)
	}
	query = query.OrderExpr(`LOWER("class_list_entry".last_name) ASC, LOWER("class_list_entry".first_name) ASC, "class_list_entry".id ASC`)
	stats, err := scanAll(ctx, query, "list class list entries")
	if err != nil {
		return nil, stats, err
	}
	result := make([]schoolmembership.ClassListEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, classListEntryToPublic(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

// ListClassListEntriesInDisplayOrder returns the same rows a class list is
// read in: class first (grade-aware), then the names in German dictionary
// order. PostgreSQL cannot express either leg, so the rows are fetched with
// the plain listing and ordered here.
func (s *Store) ListClassListEntriesInDisplayOrder(ctx context.Context, filter schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, domain.OperationStats, error) {
	entries, stats, err := s.ListClassListEntries(ctx, filter)
	if err != nil {
		return nil, stats, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return displayorder.CompareClassListEntry(displayOrderKey(entries[i]), displayOrderKey(entries[j])) < 0
	})
	return entries, stats, nil
}

func (s *Store) CreateClassListEntry(ctx context.Context, fields schoolmembership.ClassListEntryFields, createdBy *int64) (schoolmembership.ClassListEntry, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.ClassListEntry{}, domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return schoolmembership.ClassListEntry{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a class list entry")
	}
	row := classListEntryRow{
		TenantID: scope.TenantID, FirstName: fields.FirstName, LastName: fields.LastName,
		SchoolClass: fields.SchoolClass, CreatedBy: createdBy,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.class_list_entries`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return schoolmembership.ClassListEntry{}, stats, wrapClassListEntryWriteError("create", err)
	}
	stats.Rows = 1
	return classListEntryToPublic(row), stats, nil
}

func (s *Store) UpdateClassListEntry(ctx context.Context, id int64, fields schoolmembership.ClassListEntryFields) (schoolmembership.ClassListEntry, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.ClassListEntry{}, domain.OperationStats{}, err
	}
	row := classListEntryRow{ID: id, FirstName: fields.FirstName, LastName: fields.LastName, SchoolClass: fields.SchoolClass}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.class_list_entries AS "class_list_entry"`).
		Column("first_name", "last_name", "school_class").
		Set(`updated_at = NOW()`).
		Where(`"class_list_entry".id = ?`, id), "class_list_entry", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.ClassListEntry{}, stats, schoolmembership.ErrClassListEntryNotFound
	}
	if err != nil {
		return schoolmembership.ClassListEntry{}, stats, wrapClassListEntryWriteError("update", err)
	}
	stats.Rows = 1
	return classListEntryToPublic(row), stats, nil
}

func (s *Store) DeleteClassListEntry(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*classListEntryRow)(nil)).
		ModelTableExpr(`users.class_list_entries AS "class_list_entry"`).
		Where(`"class_list_entry".id = ?`, id), "class_list_entry", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: delete class list entry: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: delete class list entry: count rows: %w", err)
	}
	if rows != 1 {
		return stats, schoolmembership.ErrClassListEntryNotFound
	}
	stats.Rows = rows
	return stats, nil
}

func displayOrderKey(entry schoolmembership.ClassListEntry) displayorder.ClassListEntry {
	return displayorder.ClassListEntry{
		SchoolClass: entry.SchoolClass, LastName: entry.LastName, FirstName: entry.FirstName, ID: entry.ID,
	}
}

func classListEntrySelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.class_list_entries AS "class_list_entry"`)
}

func classListEntryToPublic(row classListEntryRow) schoolmembership.ClassListEntry {
	return schoolmembership.ClassListEntry{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, CreatedBy: row.CreatedBy,
	}
}

// wrapClassListEntryWriteError classifies the unique-index collision as the
// documented duplicate refusal. The sentinel's text is the screen's message,
// so the driver error does not travel with it.
func wrapClassListEntryWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, classListEntryNameIndex) {
		return schoolmembership.ErrClassListEntryDuplicate
	}
	return fmt.Errorf("school membership postgres: %s class list entry: %w", operation, err)
}
