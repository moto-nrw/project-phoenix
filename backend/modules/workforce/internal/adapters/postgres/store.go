// Package postgres is the Workforce work-time persistence adapter. It is the
// only place that reads or writes config.work_time_models,
// config.work_time_model_entries and config.staff_work_schedules.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const (
	tableWorkTimeModels       = "config.work_time_models"
	tableWorkTimeModelEntries = "config.work_time_model_entries"
	tableStaffWorkSchedules   = "config.staff_work_schedules"
)

// Database resolves the connection and the tenant of the current request. A
// zero tenant means no tenant is bound, which keeps the bootstrap and CLI
// reads unscoped exactly as the legacy repositories were.
type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("workforce postgres: database runtime is required")
	}
	return &Store{database: database}
}

// calendarDate binds the module's YYYY-MM-DD strings to PostgreSQL DATE
// columns. Binding the plain string keeps the driver from shifting the day
// through a timezone conversion.
type calendarDate string

func optionalCalendarDate(value string) *calendarDate {
	if value == "" {
		return nil
	}
	date := calendarDate(value)
	return &date
}

func calendarDateString(value *calendarDate) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

// nullableDate renders a date predicate argument; an unset date binds NULL so
// the comparison behaves exactly as it did for the legacy repositories.
func nullableDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// optionalClock binds a wall clock to a PostgreSQL TIME column. time.Parse
// anchors the clock at UTC, so the driver's UTC conversion cannot move it to
// another hour the way a zoned instant would. A malformed value is an error,
// never a silently unset start time.
func optionalClock(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(domain.ClockLayout, value)
	if err != nil {
		return nil, fmt.Errorf("workforce postgres: start time %q is not a %s wall clock: %w", value, domain.ClockLayout, err)
	}
	return &parsed, nil
}

// clockString reads the time-of-day components only, discarding whatever date
// anchor and location the driver attached on scan.
func clockString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(domain.ClockLayout)
}

type workTimeModelRow struct {
	bun.BaseModel      `bun:"table:config.work_time_models,alias:work_time_model"`
	ID                 int64        `bun:"id,pk,autoincrement"`
	TenantID           int64        `bun:"tenant_id,notnull"`
	Name               string       `bun:"name,notnull"`
	RotationLength     int          `bun:"rotation_length,notnull"`
	RotationAnchorDate calendarDate `bun:"rotation_anchor_date,notnull,type:date"`
	CreatedAt          time.Time    `bun:"created_at,nullzero,notnull,default:now()"`
	UpdatedAt          time.Time    `bun:"updated_at,nullzero,notnull,default:now()"`
}

type workTimeModelEntryRow struct {
	bun.BaseModel `bun:"table:config.work_time_model_entries,alias:work_time_model_entry"`
	ID            int64      `bun:"id,pk,autoincrement"`
	ModelID       int64      `bun:"model_id,notnull"`
	WeekIndex     int        `bun:"week_index,notnull"`
	DayOfWeek     int        `bun:"day_of_week,notnull"`
	TargetMinutes int        `bun:"target_minutes,notnull"`
	StartTime     *time.Time `bun:"start_time"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:now()"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:now()"`
}

type staffWorkScheduleRow struct {
	bun.BaseModel      `bun:"table:config.staff_work_schedules,alias:staff_work_schedule"`
	ID                 int64         `bun:"id,pk,autoincrement"`
	TenantID           int64         `bun:"tenant_id,notnull"`
	StaffID            int64         `bun:"staff_id,notnull"`
	WeekIndex          int           `bun:"week_index,notnull"`
	RotationLength     int           `bun:"rotation_length,notnull"`
	DayOfWeek          int           `bun:"day_of_week,notnull"`
	TargetMinutes      int           `bun:"target_minutes,notnull"`
	StartTime          *time.Time    `bun:"start_time"`
	RotationAnchorDate *calendarDate `bun:"rotation_anchor_date,type:date"`
	ValidFrom          calendarDate  `bun:"valid_from,notnull,type:date"`
	ValidUntil         *calendarDate `bun:"valid_until,type:date"`
	CreatedAt          time.Time     `bun:"created_at,notnull"`
	UpdatedAt          time.Time     `bun:"updated_at,notnull"`
}

// --- work-time templates ---

func (s *Store) ListWorkTimeModels(ctx context.Context) ([]domain.WorkTimeModel, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []workTimeModelRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		OrderExpr(`"work_time_model".name ASC`), "work_time_model", tenantID)

	stats, err := scanAll(ctx, query, "list work-time models")
	if err != nil {
		return nil, stats, err
	}
	return s.attachEntries(ctx, db, rows, stats)
}

func (s *Store) FindWorkTimeModel(ctx context.Context, id int64) (domain.WorkTimeModel, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.WorkTimeModel{}, false, domain.OperationStats{}, err
	}
	row := &workTimeModelRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		Where(`"work_time_model".id = ?`, id), "work_time_model", tenantID)

	found, stats, err := scanOne(ctx, query, "find work-time model")
	if err != nil || !found {
		return domain.WorkTimeModel{}, found, stats, err
	}
	models, stats, err := s.attachEntries(ctx, db, []workTimeModelRow{*row}, stats)
	if err != nil {
		return domain.WorkTimeModel{}, false, stats, err
	}
	return models[0], true, stats, nil
}

func (s *Store) ListWorkTimeModelsByIDs(ctx context.Context, ids []int64) ([]domain.WorkTimeModel, domain.OperationStats, error) {
	if len(ids) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []workTimeModelRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		Where(`"work_time_model".id IN (?)`, bun.List(ids)), "work_time_model", tenantID)

	stats, err := scanAll(ctx, query, "find work-time models by ids")
	if err != nil {
		return nil, stats, err
	}
	return s.attachEntries(ctx, db, rows, stats)
}

// attachEntries loads the slot rows of every template in one extra SELECT.
// bun's Relation() helper drops the schema qualifier on the FROM clause for
// these tables, so the join is spelled out here instead.
func (s *Store) attachEntries(
	ctx context.Context,
	db bun.IDB,
	rows []workTimeModelRow,
	stats domain.OperationStats,
) ([]domain.WorkTimeModel, domain.OperationStats, error) {
	models := make([]domain.WorkTimeModel, 0, len(rows))
	for _, row := range rows {
		models = append(models, workTimeModelToDomain(row))
	}
	if len(models) == 0 {
		return models, stats, nil
	}
	ids := make([]int64, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	entryRows := []workTimeModelEntryRow{}
	query := db.NewSelect().
		Model(&entryRows).
		ModelTableExpr(tableWorkTimeModelEntries+` AS "work_time_model_entry"`).
		Where(`"work_time_model_entry".model_id IN (?)`, bun.List(ids)).
		OrderExpr(`"work_time_model_entry".week_index ASC, "work_time_model_entry".day_of_week ASC`)

	entryStats, err := scanAll(ctx, query, "load work-time model entries")
	stats.Add(entryStats)
	if err != nil {
		return nil, stats, err
	}
	byModel := make(map[int64][]domain.WorkTimeModelEntry, len(models))
	for _, row := range entryRows {
		byModel[row.ModelID] = append(byModel[row.ModelID], workTimeModelEntryToDomain(row))
	}
	for index := range models {
		models[index].Entries = byModel[models[index].ID]
	}
	return models, stats, nil
}

func (s *Store) CreateWorkTimeModel(ctx context.Context, fields domain.WorkTimeModelFields) (domain.WorkTimeModel, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.WorkTimeModel{}, domain.OperationStats{}, err
	}
	row := &workTimeModelRow{
		TenantID:           tenantID,
		Name:               fields.Name,
		RotationLength:     fields.RotationLength,
		RotationAnchorDate: calendarDate(fields.RotationAnchorDate),
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableWorkTimeModels).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.WorkTimeModel{}, stats, fmt.Errorf("workforce postgres: insert work-time model: %w", err)
	}
	stats.Rows = 1

	entryStats, err := s.replaceEntries(ctx, db, row.ID, fields.Entries)
	stats.Add(entryStats)
	if err != nil {
		return domain.WorkTimeModel{}, stats, err
	}
	return s.reload(ctx, db, row, stats)
}

func (s *Store) UpdateWorkTimeModel(ctx context.Context, id int64, fields domain.WorkTimeModelFields) (domain.WorkTimeModel, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.WorkTimeModel{}, false, domain.OperationStats{}, err
	}
	row := &workTimeModelRow{ID: id, TenantID: tenantID}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableWorkTimeModels).
		Set("name = ?", fields.Name).
		Set("rotation_length = ?", fields.RotationLength).
		Set("rotation_anchor_date = ?", calendarDate(fields.RotationAnchorDate)).
		Set("updated_at = NOW()").
		Where("id = ?", id)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.WorkTimeModel{}, false, stats, fmt.Errorf("workforce postgres: update work-time model: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.WorkTimeModel{}, false, stats, fmt.Errorf("workforce postgres: update work-time model rows affected: %w", err)
	}
	if affected != 1 {
		return domain.WorkTimeModel{}, false, stats, nil
	}
	stats.Rows = affected

	entryStats, err := s.replaceEntries(ctx, db, id, fields.Entries)
	stats.Add(entryStats)
	if err != nil {
		return domain.WorkTimeModel{}, false, stats, err
	}
	model, stats, err := s.reload(ctx, db, row, stats)
	return model, err == nil, stats, err
}

// replaceEntries deletes the template's slot rows and reinserts the given
// ones, which is what keeps UNIQUE(model_id, week_index, day_of_week) exact
// without a per-slot upsert.
func (s *Store) replaceEntries(
	ctx context.Context,
	db bun.IDB,
	modelID int64,
	entries []domain.WorkTimeModelEntryFields,
) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err := db.NewDelete().
		Model((*workTimeModelEntryRow)(nil)).
		ModelTableExpr(tableWorkTimeModelEntries).
		Where("model_id = ?", modelID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: delete work-time model entries: %w", err)
	}
	if len(entries) == 0 {
		return stats, nil
	}
	rows := make([]workTimeModelEntryRow, 0, len(entries))
	for _, entry := range entries {
		startTime, clockErr := optionalClock(entry.StartTime)
		if clockErr != nil {
			return stats, clockErr
		}
		rows = append(rows, workTimeModelEntryRow{
			ModelID:       modelID,
			WeekIndex:     entry.WeekIndex,
			DayOfWeek:     entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes,
			StartTime:     startTime,
		})
	}
	stats.Queries++
	started = time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(tableWorkTimeModelEntries).Exec(ctx)
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: insert work-time model entries: %w", err)
	}
	stats.Rows += int64(len(rows))
	return stats, nil
}

func (s *Store) reload(
	ctx context.Context,
	db bun.IDB,
	row *workTimeModelRow,
	stats domain.OperationStats,
) (domain.WorkTimeModel, domain.OperationStats, error) {
	reloaded := &workTimeModelRow{}
	query := db.NewSelect().
		Model(reloaded).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		Where(`"work_time_model".id = ?`, row.ID)
	if row.TenantID > 0 {
		query = query.Where(`"work_time_model".tenant_id = ?`, row.TenantID)
	}
	found, readStats, err := scanOne(ctx, query, "reload work-time model")
	stats.Add(readStats)
	if err != nil {
		return domain.WorkTimeModel{}, stats, err
	}
	if !found {
		return domain.WorkTimeModel{}, stats, domain.ErrWorkTimeModelNotFound
	}
	models, stats, err := s.attachEntries(ctx, db, []workTimeModelRow{*reloaded}, stats)
	if err != nil {
		return domain.WorkTimeModel{}, stats, err
	}
	return models[0], stats, nil
}

func (s *Store) DeleteWorkTimeModel(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewDelete().
		Model((*workTimeModelRow)(nil)).
		ModelTableExpr(tableWorkTimeModels).
		Where("id = ?", id)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("workforce postgres: delete work-time model: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("workforce postgres: delete work-time model rows affected: %w", err)
	}
	stats.Rows = affected
	return affected > 0, stats, nil
}

// --- staff schedules ---

func (s *Store) CurrentStaffSchedule(ctx context.Context, staffID int64) ([]domain.StaffWorkSchedule, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffWorkScheduleRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id = ?", staffID).
		Where("valid_until IS NULL").
		OrderExpr("day_of_week ASC"), "staff_work_schedule", tenantID)

	stats, err := scanAll(ctx, query, "get current schedule")
	if err != nil {
		return nil, stats, err
	}
	return staffSchedulesToDomain(rows), stats, nil
}

func (s *Store) StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]domain.StaffWorkSchedule, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	value := nullableDate(date)
	rows := []staffWorkScheduleRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id = ?", staffID).
		Where("valid_from <= ?", value).
		Where("(valid_until IS NULL OR valid_until > ?)", value).
		OrderExpr("day_of_week ASC"), "staff_work_schedule", tenantID)

	stats, err := scanAll(ctx, query, "get schedule for date")
	if err != nil {
		return nil, stats, err
	}
	return staffSchedulesToDomain(rows), stats, nil
}

func (s *Store) StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]domain.StaffWorkSchedule, domain.OperationStats, error) {
	if len(staffIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffWorkScheduleRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Where("valid_from <= ?", nullableDate(to)).
		Where("(valid_until IS NULL OR valid_until > ?)", nullableDate(from)).
		OrderExpr("staff_id ASC, day_of_week ASC"), "staff_work_schedule", tenantID)

	stats, err := scanAll(ctx, query, "get schedules for staff range")
	if err != nil {
		return nil, stats, err
	}
	return staffSchedulesToDomain(rows), stats, nil
}

func (s *Store) HasStaffScheduleHistory(ctx context.Context, staffID int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewSelect().
		Model((*staffWorkScheduleRow)(nil)).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id = ?", staffID), "staff_work_schedule", tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	exists, err := query.Exists(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("workforce postgres: check schedule history: %w", err)
	}
	if exists {
		stats.Rows = 1
	}
	return exists, stats, nil
}

func (s *Store) StaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, domain.OperationStats, error) {
	result := make(map[int64]bool, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		StaffID int64 `bun:"staff_id"`
	}
	query := withTenant(db.NewSelect().
		ColumnExpr("DISTINCT staff_id").
		TableExpr(tableStaffWorkSchedules).
		Where("staff_id IN (?)", bun.List(staffIDs)), "", tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("workforce postgres: check schedule history for staff batch: %w", err)
	}
	for _, row := range rows {
		result[row.StaffID] = true
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}

// CloseStaffSchedules closes every running version at the given day. The bound
// is exclusive, matching the read predicates.
func (s *Store) CloseStaffSchedules(ctx context.Context, staffIDs []int64, until string) (domain.OperationStats, error) {
	if len(staffIDs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		TableExpr(tableStaffWorkSchedules).
		Set("valid_until = ?", calendarDate(until)).
		Set("updated_at = ?", time.Now()).
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Where("valid_until IS NULL")
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: close schedule versions: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: close schedule versions rows affected: %w", err)
	}
	stats.Rows = affected
	return stats, nil
}

func (s *Store) InsertStaffSchedules(ctx context.Context, values []domain.StaffWorkSchedule) (domain.OperationStats, error) {
	if len(values) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	now := time.Now()
	rows := make([]staffWorkScheduleRow, 0, len(values))
	for _, value := range values {
		startTime, clockErr := optionalClock(value.StartTime)
		if clockErr != nil {
			return domain.OperationStats{}, clockErr
		}
		rows = append(rows, staffWorkScheduleRow{
			TenantID:           tenantID,
			StaffID:            value.StaffID,
			WeekIndex:          value.WeekIndex,
			RotationLength:     value.RotationLength,
			DayOfWeek:          value.DayOfWeek,
			TargetMinutes:      value.TargetMinutes,
			StartTime:          startTime,
			RotationAnchorDate: optionalCalendarDate(value.RotationAnchorDate),
			ValidFrom:          calendarDate(value.ValidFrom),
			CreatedAt:          now,
			UpdatedAt:          now,
		})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(tableStaffWorkSchedules).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: insert schedule versions: %w", err)
	}
	stats.Rows = int64(len(rows))
	return stats, nil
}

// --- plumbing ---

func workTimeModelToDomain(row workTimeModelRow) domain.WorkTimeModel {
	return domain.WorkTimeModel{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name,
		RotationLength: row.RotationLength, RotationAnchorDate: string(row.RotationAnchorDate),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func workTimeModelEntryToDomain(row workTimeModelEntryRow) domain.WorkTimeModelEntry {
	return domain.WorkTimeModelEntry{
		ID: row.ID, ModelID: row.ModelID, WeekIndex: row.WeekIndex, DayOfWeek: row.DayOfWeek,
		TargetMinutes: row.TargetMinutes, StartTime: clockString(row.StartTime),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffSchedulesToDomain(rows []staffWorkScheduleRow) []domain.StaffWorkSchedule {
	result := make([]domain.StaffWorkSchedule, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.StaffWorkSchedule{
			ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID,
			WeekIndex: row.WeekIndex, RotationLength: row.RotationLength,
			DayOfWeek: row.DayOfWeek, TargetMinutes: row.TargetMinutes,
			StartTime:          clockString(row.StartTime),
			RotationAnchorDate: calendarDateString(row.RotationAnchorDate),
			ValidFrom:          string(row.ValidFrom),
			ValidUntil:         calendarDateString(row.ValidUntil),
			CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return result
}

func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query
	}
	if alias == "" {
		return query.Where("tenant_id = ?", tenantID)
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
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
		return false, stats, fmt.Errorf("workforce postgres: %s: %w", operation, err)
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
		return stats, fmt.Errorf("workforce postgres: %s: %w", operation, err)
	}
	return stats, nil
}
