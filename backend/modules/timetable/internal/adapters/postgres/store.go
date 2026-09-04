package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/enrollmentprovenance"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("timetable postgres: database runtime is required")
	}
	return &Store{database: database}
}

type categoryRow struct {
	bun.BaseModel `bun:"table:categories,alias:category"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string     `bun:"name,notnull"`
	Description   string     `bun:"description"`
	Color         string     `bun:"color"`
	IsSystem      bool       `bun:"is_system,notnull,default:false"`
	ShiftTypeID   *int64     `bun:"shift_type_id"`
	ArchivedAt    *time.Time `bun:"archived_at"`
}

func (s *Store) FindCategory(ctx context.Context, id int64, lock string) (domain.Category, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Category{}, false, domain.OperationStats{}, err
	}
	row := categoryRow{}
	query := categorySelect(db, &row).
		Where(`"category".tenant_id = ?`, tenantID).
		Where(`"category".id = ?`, id)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find category")
	return toDomain(row), found, stats, err
}

func (s *Store) FindCategoryByName(ctx context.Context, name string, includeArchived bool, lock string) (domain.Category, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Category{}, false, domain.OperationStats{}, err
	}
	row := categoryRow{}
	query := categorySelect(db, &row).
		Where(`"category".tenant_id = ?`, tenantID).
		Where(`LOWER("category".name) = LOWER(?)`, name)
	query = categoryNameScope(query, includeArchived)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find category by name")
	return toDomain(row), found, stats, err
}

func categoryNameScope(query *bun.SelectQuery, includeArchived bool) *bun.SelectQuery {
	if !includeArchived {
		return query.Where(`"category".archived_at IS NULL`)
	}
	return query.
		OrderExpr(`"category".archived_at ASC NULLS FIRST`).
		OrderExpr(`"category".updated_at DESC`).
		Limit(1)
}

func (s *Store) ListCategories(ctx context.Context) ([]domain.Category, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []categoryRow{}
	query := categorySelect(db, &rows).
		Where(`"category".tenant_id = ?`, tenantID).
		OrderExpr(`"category".name ASC, "category".id ASC`)
	stats, err := scanAll(ctx, query, "list categories")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}

func (s *Store) CountCategoryUsage(ctx context.Context) (map[int64]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []struct {
		CategoryID int64 `bun:"category_id"`
		Count      int   `bun:"usage_count"`
	}{}
	query := db.NewSelect().Model(&rows).TableExpr(`activities.groups AS "group"`).
		ColumnExpr(`"group".category_id`).
		ColumnExpr(`COUNT(*) AS usage_count`).
		Where(`"group".tenant_id = ?`, tenantID).
		GroupExpr(`"group".category_id`)
	stats, err := scanAll(ctx, query, "count category usage")
	if err != nil {
		return nil, stats, err
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.CategoryID] = row.Count
	}
	stats.Rows = int64(len(rows))
	return counts, stats, nil
}

func (s *Store) CreateCategory(ctx context.Context, fields domain.CategoryFields) (domain.Category, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Category{}, domain.OperationStats{}, err
	}
	row := categoryRow{TenantID: tenantID}
	applyFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`activities.categories`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Category{}, stats, classifyWriteError("create category", err, &stats)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) UpdateCategoryIfActive(ctx context.Context, id int64, fields domain.CategoryFields) (domain.Category, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Category{}, false, domain.OperationStats{}, err
	}
	row := categoryRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`activities.categories`).
		Set("name = ?", fields.Name).Set("description = ?", fields.Description).Set("color = ?", fields.Color).
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Where("archived_at IS NULL").
		Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Category{}, false, stats, nil
	}
	if err != nil {
		return domain.Category{}, false, stats, classifyWriteError("update category", err, &stats)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) SetCategoryArchivedAt(ctx context.Context, id int64, archivedAt *time.Time) (domain.Category, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Category{}, false, domain.OperationStats{}, err
	}
	row := categoryRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`activities.categories`).
		Set("archived_at = ?", archivedAt).Where("id = ?", id).Where("tenant_id = ?", tenantID).
		Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Category{}, false, stats, nil
	}
	if err != nil {
		return domain.Category{}, false, stats, classifyWriteError("set category archive state", err, &stats)
	}
	stats.Rows = 1
	return toDomain(row), true, stats, nil
}

func (s *Store) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	ids := dedupe(categoryIDs)
	stats := domain.OperationStats{}
	if err := validateCategoryIDs(ctx, db, tenantID, ids, &stats); err != nil {
		return stats, err
	}
	if err := clearShiftTypeLinks(ctx, db, tenantID, shiftTypeID, ids, &stats); err != nil {
		return stats, err
	}
	if len(ids) == 0 {
		return stats, nil
	}
	return stats, setShiftTypeLinks(ctx, db, tenantID, shiftTypeID, ids, &stats)
}

func (s *Store) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.ExecContext(ctx, `
		SELECT enrollment.id
		FROM activities.student_enrollments AS enrollment
		WHERE enrollment.tenant_id = ? AND enrollment.student_id IN (?)
		  AND (enrollment.valid_until IS NULL OR enrollment.valid_until > ?::date)
		FOR UPDATE OF enrollment`, tenantID, bun.List(studentIDs), validUntil)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("timetable postgres: lock student enrollments for care exit: %w", err)
	}
	return stats, nil
}

func (s *Store) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (domain.CareExitEnrollmentChanges, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.CareExitEnrollmentChanges{}, domain.OperationStats{}, err
	}
	changes := domain.CareExitEnrollmentChanges{}
	stats := domain.OperationStats{}
	if err := deleteFutureEnrollments(ctx, db, tenantID, studentIDs, validUntil, &changes, &stats); err != nil {
		return changes, stats, err
	}
	if err := listCappedEnrollments(ctx, db, tenantID, studentIDs, validUntil, &changes, &stats); err != nil {
		return changes, stats, err
	}
	if err := capStudentEnrollments(ctx, db, tenantID, studentIDs, validUntil, &stats); err != nil {
		return changes, stats, err
	}
	return changes, stats, nil
}

func deleteFutureEnrollments(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil string, changes *domain.CareExitEnrollmentChanges, stats *domain.OperationStats) error {
	started := time.Now()
	err := db.NewRaw(`
		DELETE FROM activities.student_enrollments AS enrollment
		WHERE enrollment.tenant_id = ? AND enrollment.student_id IN (?)
		  AND enrollment.valid_from >= ?::date
		  AND (enrollment.valid_until IS NULL OR enrollment.valid_until > ?::date)
		RETURNING enrollment.id, enrollment.tenant_id, enrollment.student_id,
		          enrollment.activity_group_id, enrollment.valid_from::text AS valid_from,
		          enrollment.valid_until::text AS valid_until, enrollment.calendar_period_id,
		          enrollment.enrollment_request_child_id, enrollment.selected_weekdays,
		          enrollment.attendance_status, enrollment.weekday`,
		tenantID, bun.List(studentIDs), validUntil, validUntil).Scan(ctx, &changes.Deleted)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("timetable postgres: delete future enrollments for care exit: %w", err)
	}
	stats.Rows += int64(len(changes.Deleted))
	return nil
}

const cappedEnrollmentPredicate = `
	 enrollment.tenant_id = ? AND enrollment.student_id IN (?)
	 AND enrollment.valid_from < ?::date
	 AND (enrollment.valid_until IS NULL OR enrollment.valid_until > ?::date)`

func listCappedEnrollments(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil string, changes *domain.CareExitEnrollmentChanges, stats *domain.OperationStats) error {
	started := time.Now()
	err := db.NewRaw(`SELECT enrollment.tenant_id, enrollment.student_id, enrollment.id,
		enrollment.valid_until::text AS previous_valid_until
		FROM activities.student_enrollments AS enrollment WHERE `+cappedEnrollmentPredicate,
		tenantID, bun.List(studentIDs), validUntil, validUntil).Scan(ctx, &changes.Capped)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("timetable postgres: list capped enrollments for care exit: %w", err)
	}
	return nil
}

func capStudentEnrollments(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, validUntil string, stats *domain.OperationStats) error {
	started := time.Now()
	result, err := db.ExecContext(ctx, `UPDATE activities.student_enrollments AS enrollment
		SET valid_until = ?::date, updated_at = NOW()
		WHERE `+cappedEnrollmentPredicate, validUntil, tenantID, bun.List(studentIDs), validUntil, validUntil)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("timetable postgres: cap student enrollments for care exit: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("timetable postgres: count capped enrollments for care exit: %w", err)
	}
	stats.Rows += rows
	return nil
}

func (s *Store) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []domain.CareExitEnrollmentRemoval) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	payload := careExitEnrollmentPayload(removals)
	requestedChildIDs := enrollmentRequestChildIDs(removals)
	requestChildIDs := []int64{}
	stats := domain.OperationStats{}
	if len(requestedChildIDs) > 0 {
		started := time.Now()
		requestChildIDs, err = enrollmentprovenance.ExistingRequestChildIDs(ctx, db, tenantID, requestedChildIDs)
		stats.Queries++
		stats.StatementDuration += time.Since(started)
		if err != nil {
			return 0, stats, err
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: encode care exit enrollment removals: %w", err)
	}
	capped, err := restoreCappedEnrollments(ctx, db, tenantID, studentIDs, encoded, &stats)
	if err != nil {
		return 0, stats, err
	}
	deleted, err := restoreDeletedEnrollments(ctx, db, tenantID, studentIDs, periodIDs, requestChildIDs, encoded, countEligibleDeletedEnrollmentRemovals(payload, tenantID, studentIDs), &stats)
	return capped + deleted, stats, err
}

func restoreCappedEnrollments(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, payload []byte, stats *domain.OperationStats) (int64, error) {
	started := time.Now()
	result, err := db.ExecContext(ctx, `UPDATE activities.student_enrollments AS enrollment
		SET valid_until = NULLIF(removal.previous_valid_until, '')::date, updated_at = NOW()
		FROM jsonb_to_recordset(?::jsonb) AS removal(
			tenant_id bigint, student_id bigint, enrollment_id bigint, was_deleted boolean,
			previous_valid_until text, activity_group_id bigint, valid_from text,
			calendar_period_id bigint, enrollment_request_child_id bigint,
			selected_weekdays jsonb, attendance_status text, weekday smallint)
		WHERE removal.was_deleted = FALSE
		  AND removal.tenant_id = enrollment.tenant_id AND removal.enrollment_id = enrollment.id
		  AND removal.tenant_id = ? AND removal.student_id IN (?)
		  AND enrollment.valid_until IS DISTINCT FROM NULLIF(removal.previous_valid_until, '')::date`, string(payload), tenantID, bun.List(studentIDs))
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return 0, fmt.Errorf("timetable postgres: restore capped enrollments after care exit: %w", err)
	}
	return rowsAffected(result, "restored capped enrollments", stats)
}

func restoreDeletedEnrollments(ctx context.Context, db bun.IDB, tenantID int64, studentIDs, periodIDs, requestChildIDs []int64, payload []byte, expected int64, stats *domain.OperationStats) (int64, error) {
	started := time.Now()
	result, err := db.ExecContext(ctx, `INSERT INTO activities.student_enrollments (
		id, tenant_id, student_id, activity_group_id, valid_from, valid_until,
		calendar_period_id, enrollment_request_child_id, selected_weekdays,
		attendance_status, weekday)
		SELECT removal.enrollment_id, removal.tenant_id, removal.student_id, removal.activity_group_id,
		       NULLIF(removal.valid_from, '')::date, NULLIF(removal.previous_valid_until, '')::date,
		       CASE WHEN removal.calendar_period_id = ANY(?::BIGINT[]) THEN removal.calendar_period_id END,
		       CASE WHEN removal.enrollment_request_child_id = ANY(?::BIGINT[])
		            THEN removal.enrollment_request_child_id END,
		       removal.selected_weekdays, removal.attendance_status, removal.weekday
		FROM jsonb_to_recordset(?::jsonb) AS removal(
			tenant_id bigint, student_id bigint, enrollment_id bigint, was_deleted boolean,
			previous_valid_until text, activity_group_id bigint, valid_from text,
			calendar_period_id bigint, enrollment_request_child_id bigint,
			selected_weekdays jsonb, attendance_status text, weekday smallint)
		JOIN activities.groups AS activity_group
		  ON activity_group.tenant_id = removal.tenant_id AND activity_group.id = removal.activity_group_id
		WHERE removal.was_deleted = TRUE AND removal.tenant_id = ? AND removal.student_id IN (?)
		ON CONFLICT DO NOTHING`, pgdialect.Array(periodIDs), pgdialect.Array(requestChildIDs), string(payload), tenantID, bun.List(studentIDs))
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return 0, fmt.Errorf("timetable postgres: restore deleted enrollments after care exit: %w", err)
	}
	rows, err := rowsAffected(result, "restored deleted enrollments", stats)
	if expected > rows {
		stats.DuplicatePreventionConflicts += expected - rows
	}
	return rows, err
}

func rowsAffected(result sql.Result, operation string, stats *domain.OperationStats) (int64, error) {
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("timetable postgres: count %s: %w", operation, err)
	}
	stats.Rows += rows
	return rows, nil
}

type careExitEnrollmentJSON struct {
	TenantID                 int64   `json:"tenant_id"`
	StudentID                int64   `json:"student_id"`
	EnrollmentID             int64   `json:"enrollment_id"`
	WasDeleted               bool    `json:"was_deleted"`
	PreviousValidUntil       *string `json:"previous_valid_until"`
	ActivityGroupID          int64   `json:"activity_group_id"`
	ValidFrom                string  `json:"valid_from"`
	CalendarPeriodID         *int64  `json:"calendar_period_id"`
	EnrollmentRequestChildID *int64  `json:"enrollment_request_child_id"`
	SelectedWeekdays         []int   `json:"selected_weekdays"`
	AttendanceStatus         *string `json:"attendance_status"`
	Weekday                  *int    `json:"weekday"`
}

func careExitEnrollmentPayload(removals []domain.CareExitEnrollmentRemoval) []careExitEnrollmentJSON {
	result := make([]careExitEnrollmentJSON, 0, len(removals))
	for _, removal := range removals {
		result = append(result, careExitEnrollmentJSON{
			TenantID: removal.TenantID, StudentID: removal.StudentID, EnrollmentID: removal.ID,
			WasDeleted: removal.WasDeleted, PreviousValidUntil: removal.PreviousValidUntil,
			ActivityGroupID: removal.ActivityGroupID, ValidFrom: removal.ValidFrom,
			CalendarPeriodID: removal.CalendarPeriodID, EnrollmentRequestChildID: removal.EnrollmentRequestChildID,
			SelectedWeekdays: removal.SelectedWeekdays, AttendanceStatus: removal.AttendanceStatus, Weekday: removal.Weekday,
		})
	}
	return result
}

func countEligibleDeletedEnrollmentRemovals(removals []careExitEnrollmentJSON, tenantID int64, studentIDs []int64) int64 {
	students := make(map[int64]struct{}, len(studentIDs))
	for _, studentID := range studentIDs {
		students[studentID] = struct{}{}
	}
	var count int64
	for _, removal := range removals {
		if _, ok := students[removal.StudentID]; removal.WasDeleted && removal.TenantID == tenantID && ok {
			count++
		}
	}
	return count
}

func enrollmentRequestChildIDs(removals []domain.CareExitEnrollmentRemoval) []int64 {
	result := make([]int64, 0)
	for _, removal := range removals {
		if removal.EnrollmentRequestChildID != nil {
			result = append(result, *removal.EnrollmentRequestChildID)
		}
	}
	return dedupe(result)
}

func validateCategoryIDs(ctx context.Context, db bun.IDB, tenantID int64, ids []int64, stats *domain.OperationStats) error {
	if len(ids) == 0 {
		return nil
	}
	var found int
	started := time.Now()
	err := db.NewSelect().Table("activities.categories").ColumnExpr("count(*)").
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(ids)).Scan(ctx, &found)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("timetable postgres: validate category IDs: %w", err)
	}
	if found != len(ids) {
		stats.DuplicatePreventionConflicts++
		return domain.ErrUnknownCategoryIDs
	}
	return nil
}

func clearShiftTypeLinks(ctx context.Context, db bun.IDB, tenantID, shiftTypeID int64, ids []int64, stats *domain.OperationStats) error {
	query := db.NewUpdate().Table("activities.categories").Set("shift_type_id = NULL").
		Where("tenant_id = ?", tenantID).Where("shift_type_id = ?", shiftTypeID)
	if len(ids) > 0 {
		query = query.Where("id NOT IN (?)", bun.List(ids))
	}
	return execUpdate(ctx, query, "clear category shift type", stats)
}

func setShiftTypeLinks(ctx context.Context, db bun.IDB, tenantID, shiftTypeID int64, ids []int64, stats *domain.OperationStats) error {
	query := db.NewUpdate().Table("activities.categories").Set("shift_type_id = ?", shiftTypeID).
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(ids))
	return execUpdate(ctx, query, "set category shift type", stats)
}

func execUpdate(ctx context.Context, query *bun.UpdateQuery, operation string, stats *domain.OperationStats) error {
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("timetable postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("timetable postgres: count %s rows: %w", operation, err)
	}
	stats.Rows += rows
	return nil
}

func categorySelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`activities.categories AS "category"`)
}

func scanOne(ctx context.Context, query *bun.SelectQuery, operation string) (bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return false, stats, nil
	}
	if err != nil {
		return false, stats, fmt.Errorf("timetable postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return true, stats, nil
}

func scanAll(ctx context.Context, query *bun.SelectQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("timetable postgres: %s: %w", operation, err)
	}
	return stats, nil
}

func classifyWriteError(operation string, err error, stats *domain.OperationStats) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == domain.CategoryNameActiveIndex {
		stats.DuplicatePreventionConflicts++
		return fmt.Errorf("%w: %w", domain.ErrCategoryNameConflict, err)
	}
	return fmt.Errorf("timetable postgres: %s: %w", operation, err)
}

func applyFields(row *categoryRow, fields domain.CategoryFields) {
	row.Name = fields.Name
	row.Description = fields.Description
	row.Color = fields.Color
	row.IsSystem = fields.IsSystem
}

func toDomain(row categoryRow) domain.Category {
	return domain.Category{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, Description: row.Description, Color: row.Color, IsSystem: row.IsSystem,
		ShiftTypeID: row.ShiftTypeID, ArchivedAt: row.ArchivedAt,
	}
}

func dedupe(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
