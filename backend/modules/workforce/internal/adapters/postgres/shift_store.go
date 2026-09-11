package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// The four Dienstplan tables Workforce owns (#2689). Nothing outside this
// package reads or writes them.
const (
	tableStaffShifts               = "schedule.staff_shifts"
	tableStaffShiftSeries          = "schedule.staff_shift_series"
	tableStaffShiftSeriesException = "schedule.staff_shift_series_exceptions"
	tableShiftTypes                = "schedule.shift_types"

	aliasStaffShift           = "staff_shift"
	aliasStaffShiftSeries     = "staff_shift_series"
	aliasStaffShiftException  = "staff_shift_series_exception"
	aliasShiftType            = "shift_type"
	staffShiftStartUniqueName = "uniq_staff_shift_start_active"
	shiftTypeNameUniqueIndex  = "uniq_shift_types_tenant_name"
)

type staffShiftRow struct {
	bun.BaseModel        `bun:"table:schedule.staff_shifts,alias:staff_shift"`
	ID                   int64         `bun:"id,pk,autoincrement"`
	TenantID             int64         `bun:"tenant_id,notnull"`
	StaffID              int64         `bun:"staff_id,notnull"`
	Date                 calendarDate  `bun:"date,notnull,type:date"`
	StartTime            time.Time     `bun:"start_time,notnull"`
	EndTime              time.Time     `bun:"end_time,notnull"`
	BreakMinutes         int           `bun:"break_minutes,notnull,default:0"`
	ShiftTypeID          *int64        `bun:"shift_type_id"`
	Notes                string        `bun:"notes"`
	SeriesID             *int64        `bun:"series_id"`
	Detached             bool          `bun:"detached,notnull,default:false"`
	SeriesOccurrenceDate *calendarDate `bun:"series_occurrence_date,type:date"`
	Cancelled            bool          `bun:"cancelled,notnull,default:false"`
	ChangeReason         *string       `bun:"change_reason"`
	OriginShiftID        *int64        `bun:"origin_shift_id"`
	SickAbsenceID        *int64        `bun:"sick_absence_id"`
	CreatedBy            int64         `bun:"created_by,notnull"`
	UpdatedBy            *int64        `bun:"updated_by"`
	CreatedAt            time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffShiftSeriesRow struct {
	bun.BaseModel             `bun:"table:schedule.staff_shift_series,alias:staff_shift_series"`
	ID                        int64         `bun:"id,pk,autoincrement"`
	TenantID                  int64         `bun:"tenant_id,notnull"`
	StaffID                   int64         `bun:"staff_id,notnull"`
	Weekdays                  []int16       `bun:"weekdays,array,notnull"`
	StartTime                 time.Time     `bun:"start_time,notnull"`
	EndTime                   time.Time     `bun:"end_time,notnull"`
	BreakMinutes              int           `bun:"break_minutes,notnull,default:0"`
	ShiftTypeID               *int64        `bun:"shift_type_id"`
	Notes                     string        `bun:"notes"`
	CalendarPeriodID          int64         `bun:"calendar_period_id,notnull"`
	WeekPattern               int           `bun:"week_pattern,notnull,default:0"`
	ValidFrom                 calendarDate  `bun:"valid_from,notnull,type:date"`
	ValidUntil                *calendarDate `bun:"valid_until,type:date"`
	SeriesRootID              *int64        `bun:"series_root_id"`
	RetainedOccurrenceShiftID *int64        `bun:"retained_occurrence_shift_id"`
	CreatedBy                 int64         `bun:"created_by,notnull"`
	UpdatedBy                 *int64        `bun:"updated_by"`
	CreatedAt                 time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                 time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffShiftSeriesExceptionRow struct {
	bun.BaseModel `bun:"table:schedule.staff_shift_series_exceptions,alias:staff_shift_series_exception"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	SeriesID      int64        `bun:"series_id,notnull"`
	Date          calendarDate `bun:"date,notnull,type:date"`
	CreatedBy     int64        `bun:"created_by,notnull"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type shiftTypeRow struct {
	bun.BaseModel `bun:"table:schedule.shift_types,alias:shift_type"`
	ID            int64  `bun:"id,pk,autoincrement"`
	TenantID      int64  `bun:"tenant_id,notnull"`
	Name          string `bun:"name,notnull"`
	Color         string `bun:"color,notnull"`
	Description   string `bun:"description"`
	// IsActive carries no default tag on purpose: bun would write DEFAULT for
	// a false value and the column default is TRUE.
	IsActive  bool      `bun:"is_active,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// --- staff shifts ---

func (s *Store) FindStaffShift(ctx context.Context, id int64) (domain.StaffShift, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShift{}, false, domain.OperationStats{}, err
	}
	row := &staffShiftRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		Where(`"staff_shift".id = ?`, id), aliasStaffShift, tenantID)
	found, stats, err := scanOne(ctx, query, "find staff shift")
	if err != nil || !found {
		return domain.StaffShift{}, found, stats, err
	}
	return staffShiftToDomain(*row), true, stats, nil
}

func (s *Store) ListStaffShifts(ctx context.Context, filter domain.StaffShiftFilter) ([]domain.StaffShift, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffShiftRow{}
	query, empty := applyStaffShiftFilter(withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`), aliasStaffShift, tenantID), filter)
	if empty {
		return []domain.StaffShift{}, domain.OperationStats{}, nil
	}
	stats, err := scanAll(ctx, query, "list staff shifts")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return staffShiftsToDomain(rows), stats, nil
}

// applyStaffShiftFilter renders the filter; empty reports a set predicate that
// can match nothing, which bun cannot render as an IN list.
func applyStaffShiftFilter(query *bun.SelectQuery, filter domain.StaffShiftFilter) (*bun.SelectQuery, bool) {
	if filter.StaffID > 0 {
		query = query.Where(`"staff_shift".staff_id = ?`, filter.StaffID)
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			return query, true
		}
		query = query.Where(`"staff_shift".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.From != "" {
		query = query.Where(`"staff_shift".date >= ?`, filter.From)
	}
	if filter.To != "" {
		query = query.Where(`"staff_shift".date <= ?`, filter.To)
	}
	if filter.Dates != nil {
		if len(filter.Dates) == 0 {
			return query, true
		}
		dates := make([]calendarDate, 0, len(filter.Dates))
		for _, date := range filter.Dates {
			dates = append(dates, calendarDate(date))
		}
		query = query.Where(`"staff_shift".date IN (?)`, bun.List(dates))
	}
	if filter.SeriesID > 0 {
		query = query.Where(`"staff_shift".series_id = ?`, filter.SeriesID)
	}
	if filter.OriginShiftID > 0 {
		query = query.Where(`"staff_shift".origin_shift_id = ?`, filter.OriginShiftID)
	}
	if filter.OriginShiftIDs != nil {
		if len(filter.OriginShiftIDs) == 0 {
			return query, true
		}
		query = query.Where(`"staff_shift".origin_shift_id IN (?)`, bun.List(filter.OriginShiftIDs))
	}
	if filter.SickAbsenceID != nil {
		query = query.Where(`"staff_shift".sick_absence_id = ?`, *filter.SickAbsenceID)
	}
	if filter.Cancelled != nil {
		query = query.Where(`"staff_shift".cancelled = ?`, *filter.Cancelled)
	}
	if filter.Detached != nil {
		query = query.Where(`"staff_shift".detached = ?`, *filter.Detached)
	}
	for _, order := range filter.Order {
		column := bun.Ident(aliasStaffShift + "." + string(order.Field))
		if order.Descending {
			query = query.OrderExpr("? DESC", column)
		} else {
			query = query.OrderExpr("? ASC", column)
		}
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	return query, false
}

func (s *Store) UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		WeekStart calendarDate `bun:"week_start,type:date"`
	}
	query := withTenant(db.NewSelect().
		TableExpr(tableStaffShifts+` AS "staff_shift"`).
		ColumnExpr(`DISTINCT date_trunc('week', "staff_shift".date)::date AS week_start`).
		Where(`"staff_shift".date >= ?`, from).
		Where(`"staff_shift".date <= ?`, to).
		Where(`"staff_shift".cancelled = FALSE`).
		OrderExpr(`week_start ASC`), aliasStaffShift, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("workforce postgres: find used staff-shift calendar weeks: %w", err)
	}
	weeks := make([]string, 0, len(rows))
	for _, row := range rows {
		weeks = append(weeks, string(row.WeekStart))
	}
	stats.Rows = int64(len(rows))
	return weeks, stats, nil
}

func (s *Store) CreateStaffShift(ctx context.Context, value domain.StaffShift) (domain.StaffShift, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShift{}, domain.OperationStats{}, err
	}
	row, err := staffShiftFromDomain(value)
	if err != nil {
		return domain.StaffShift{}, domain.OperationStats{}, err
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffShifts).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, staffShiftStartUniqueName) {
			return domain.StaffShift{}, stats, &domain.ConflictError{Kind: domain.ErrStaffShiftDuplicate, Cause: err}
		}
		return domain.StaffShift{}, stats, fmt.Errorf("workforce postgres: insert staff shift: %w", err)
	}
	stats.Rows = 1
	return staffShiftToDomain(*row), stats, nil
}

func (s *Store) CreateStaffShifts(ctx context.Context, values []domain.StaffShift) ([]domain.StaffShift, domain.OperationStats, error) {
	if len(values) == 0 {
		return []domain.StaffShift{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]*staffShiftRow, 0, len(values))
	for _, value := range values {
		row, err := staffShiftFromDomain(value)
		if err != nil {
			return nil, domain.OperationStats{}, err
		}
		if row.TenantID == 0 {
			row.TenantID = tenantID
		}
		rows = append(rows, row)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(tableStaffShifts).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, staffShiftStartUniqueName) {
			return nil, stats, &domain.ConflictError{Kind: domain.ErrStaffShiftDuplicate, Cause: err}
		}
		return nil, stats, fmt.Errorf("workforce postgres: bulk insert staff shifts: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffShift, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffShiftToDomain(*row))
	}
	return result, stats, nil
}

func (s *Store) UpdateStaffShift(ctx context.Context, value domain.StaffShift) (domain.StaffShift, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShift{}, false, domain.OperationStats{}, err
	}
	row, err := staffShiftFromDomain(value)
	if err != nil {
		return domain.StaffShift{}, false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		ExcludeColumn("tenant_id", "created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*")
	if tenantID > 0 {
		query = query.Where(`"staff_shift".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, staffShiftStartUniqueName) {
			return domain.StaffShift{}, false, stats, &domain.ConflictError{Kind: domain.ErrStaffShiftDuplicate, Cause: err}
		}
		return domain.StaffShift{}, false, stats, fmt.Errorf("workforce postgres: update staff shift: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.StaffShift{}, false, stats, fmt.Errorf("workforce postgres: update staff shift rows affected: %w", err)
	}
	if affected != 1 {
		return domain.StaffShift{}, false, stats, nil
	}
	stats.Rows = affected
	return staffShiftToDomain(*row), true, stats, nil
}

func (s *Store) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().
		Model((*staffShiftRow)(nil)).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		Set("sick_absence_id = ?", absenceID).
		Where(`"staff_shift".id = ?`, shiftID), aliasStaffShift, tenantID)
	return execCount(ctx, query, "set staff shift sick absence")
}

func (s *Store) DeleteStaffShift(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffShiftRow)(nil)).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		Where(`"staff_shift".id = ?`, id), aliasStaffShift, tenantID)
	return execAffected(ctx, query, "delete staff shift")
}

func (s *Store) DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffShiftRow)(nil)).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		Where(`"staff_shift".staff_id = ?`, staffID).
		Where(`"staff_shift".date >= ?`, from), aliasStaffShift, tenantID)
	return execCount(ctx, query, "delete upcoming staff shifts")
}

func (s *Store) DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffShiftRow)(nil)).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		Where(`"staff_shift".series_id = ?`, seriesID).
		Where(`"staff_shift".detached = FALSE`).
		Where(`"staff_shift".date >= ?`, from), aliasStaffShift, tenantID)
	return execCount(ctx, query, "delete non-detached staff shifts by series")
}

func (s *Store) RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().
		Model((*staffShiftRow)(nil)).
		ModelTableExpr(tableStaffShifts+` AS "staff_shift"`).
		Set("series_id = ?", toSeriesID).
		Where(`"staff_shift".series_id = ?`, fromSeriesID).
		Where(`"staff_shift".detached = TRUE`).
		Where(`"staff_shift".series_occurrence_date >= ?`, from), aliasStaffShift, tenantID)
	return execCount(ctx, query, "repoint detached staff shifts to successor series")
}

// --- shift series ---

func (s *Store) FindStaffShiftSeries(ctx context.Context, id int64) (domain.StaffShiftSeries, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShiftSeries{}, false, domain.OperationStats{}, err
	}
	row := &staffShiftSeriesRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableStaffShiftSeries+` AS "staff_shift_series"`).
		Where(`"staff_shift_series".id = ?`, id), aliasStaffShiftSeries, tenantID)
	found, stats, err := scanOne(ctx, query, "find staff shift series")
	if err != nil || !found {
		return domain.StaffShiftSeries{}, found, stats, err
	}
	return staffShiftSeriesToDomain(*row), true, stats, nil
}

func (s *Store) FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (domain.StaffShiftSeries, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShiftSeries{}, false, domain.OperationStats{}, err
	}
	row := &staffShiftSeriesRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableStaffShiftSeries+` AS "staff_shift_series"`).
		Where(`COALESCE("staff_shift_series".series_root_id, "staff_shift_series".id) = ?`, rootID).
		Where(`"staff_shift_series".id != ?`, excludeID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, from).
		OrderExpr(`"staff_shift_series".valid_from ASC`).
		Limit(1), aliasStaffShiftSeries, tenantID)
	found, stats, err := scanOne(ctx, query, "find overlapping staff shift series in lineage")
	if err != nil || !found {
		return domain.StaffShiftSeries{}, found, stats, err
	}
	return staffShiftSeriesToDomain(*row), true, stats, nil
}

func (s *Store) CreateStaffShiftSeries(ctx context.Context, value domain.StaffShiftSeries) (domain.StaffShiftSeries, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShiftSeries{}, domain.OperationStats{}, err
	}
	row, err := staffShiftSeriesFromDomain(value)
	if err != nil {
		return domain.StaffShiftSeries{}, domain.OperationStats{}, err
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffShiftSeries).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffShiftSeries{}, stats, fmt.Errorf("workforce postgres: insert staff shift series: %w", err)
	}
	stats.Rows = 1
	return staffShiftSeriesToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffShiftSeries(ctx context.Context, value domain.StaffShiftSeries) (domain.StaffShiftSeries, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffShiftSeries{}, false, domain.OperationStats{}, err
	}
	row, err := staffShiftSeriesFromDomain(value)
	if err != nil {
		return domain.StaffShiftSeries{}, false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableStaffShiftSeries+` AS "staff_shift_series"`).
		ExcludeColumn("tenant_id", "created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*")
	if tenantID > 0 {
		query = query.Where(`"staff_shift_series".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffShiftSeries{}, false, stats, fmt.Errorf("workforce postgres: update staff shift series: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.StaffShiftSeries{}, false, stats, fmt.Errorf("workforce postgres: update staff shift series rows affected: %w", err)
	}
	if affected != 1 {
		return domain.StaffShiftSeries{}, false, stats, nil
	}
	stats.Rows = affected
	return staffShiftSeriesToDomain(*row), true, stats, nil
}

func (s *Store) DeleteStaffShiftSeries(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffShiftSeriesRow)(nil)).
		ModelTableExpr(tableStaffShiftSeries+` AS "staff_shift_series"`).
		Where(`"staff_shift_series".id = ?`, id), aliasStaffShiftSeries, tenantID)
	return execAffected(ctx, query, "delete staff shift series")
}

// CapStaffShiftSeries bounds the segment at the exclusive date. A cap at or
// before valid_from clamps to valid_from, an empty segment that materializes
// nothing, instead of violating the validity check constraint.
func (s *Store) CapStaffShiftSeries(ctx context.Context, id int64, until string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().
		Model((*staffShiftSeriesRow)(nil)).
		ModelTableExpr(tableStaffShiftSeries+` AS "staff_shift_series"`).
		Set("valid_until = GREATEST(valid_from, ?)", calendarDate(until)).
		Where(`"staff_shift_series".id = ?`, id).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, until), aliasStaffShiftSeries, tenantID)
	return execAffected(ctx, query, "cap staff shift series valid_until")
}

func (s *Store) CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().
		Model((*staffShiftSeriesRow)(nil)).
		ModelTableExpr(tableStaffShiftSeries+` AS "staff_shift_series"`).
		Set("valid_until = GREATEST(valid_from, ?)", calendarDate(until)).
		Where(`"staff_shift_series".staff_id = ?`, staffID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, until), aliasStaffShiftSeries, tenantID)
	return execCount(ctx, query, "cap staff shift series by staff")
}

// --- series exceptions ---

func (s *Store) RecordSeriesException(ctx context.Context, value domain.StaffShiftSeriesException) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := &staffShiftSeriesExceptionRow{
		TenantID: value.TenantID, SeriesID: value.SeriesID, Date: calendarDate(value.Date), CreatedBy: value.CreatedBy,
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().
		Model(row).
		ModelTableExpr(tableStaffShiftSeriesException).
		On("CONFLICT (tenant_id, series_id, date) DO NOTHING").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: insert staff shift series exception: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil {
		stats.Rows = affected
	}
	return stats, nil
}

func (s *Store) SeriesExceptionDates(ctx context.Context, seriesID int64) ([]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		Date calendarDate `bun:"date,type:date"`
	}
	query := withTenant(db.NewSelect().
		TableExpr(tableStaffShiftSeriesException+` AS "staff_shift_series_exception"`).
		ColumnExpr(`"staff_shift_series_exception".date`).
		Where(`"staff_shift_series_exception".series_id = ?`, seriesID).
		OrderExpr(`"staff_shift_series_exception".date ASC`), aliasStaffShiftException, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("workforce postgres: find staff shift series exception dates: %w", err)
	}
	dates := make([]string, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, string(row.Date))
	}
	stats.Rows = int64(len(rows))
	return dates, stats, nil
}

func (s *Store) RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().
		Model((*staffShiftSeriesExceptionRow)(nil)).
		ModelTableExpr(tableStaffShiftSeriesException+` AS "staff_shift_series_exception"`).
		Set("series_id = ?", toSeriesID).
		Where(`"staff_shift_series_exception".series_id = ?`, fromSeriesID).
		Where(`"staff_shift_series_exception".date >= ?`, from), aliasStaffShiftException, tenantID)
	return execCount(ctx, query, "repoint staff shift series exceptions")
}

// --- shift types ---

func (s *Store) ListShiftTypes(ctx context.Context) ([]domain.ShiftType, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []shiftTypeRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableShiftTypes+` AS "shift_type"`).
		OrderExpr(`"shift_type".name ASC`), aliasShiftType, tenantID)
	stats, err := scanAll(ctx, query, "list shift types")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.ShiftType, 0, len(rows))
	for _, row := range rows {
		result = append(result, shiftTypeToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) FindShiftType(ctx context.Context, id int64) (domain.ShiftType, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ShiftType{}, false, domain.OperationStats{}, err
	}
	row := &shiftTypeRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableShiftTypes+` AS "shift_type"`).
		Where(`"shift_type".id = ?`, id), aliasShiftType, tenantID)
	found, stats, err := scanOne(ctx, query, "find shift type")
	if err != nil || !found {
		return domain.ShiftType{}, found, stats, err
	}
	return shiftTypeToDomain(*row), true, stats, nil
}

func (s *Store) CreateShiftType(ctx context.Context, value domain.ShiftType) (domain.ShiftType, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ShiftType{}, domain.OperationStats{}, err
	}
	row := shiftTypeFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableShiftTypes).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, shiftTypeNameUniqueIndex) {
			return domain.ShiftType{}, stats, &domain.ConflictError{Kind: domain.ErrShiftTypeNameTaken, Cause: err}
		}
		return domain.ShiftType{}, stats, fmt.Errorf("workforce postgres: insert shift type: %w", err)
	}
	stats.Rows = 1
	return shiftTypeToDomain(*row), stats, nil
}

// CreateShiftTypeIfAbsent is the race-free seed insert: ON CONFLICT DO NOTHING
// lets exactly one concurrent insert win while the loser stays a clean no-op
// inside the surrounding tenant transaction.
func (s *Store) CreateShiftTypeIfAbsent(ctx context.Context, value domain.ShiftType) (domain.ShiftType, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ShiftType{}, false, domain.OperationStats{}, err
	}
	row := shiftTypeFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().
		Model(row).
		ModelTableExpr(tableShiftTypes).
		On("CONFLICT (tenant_id, LOWER(name)) DO NOTHING").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ShiftType{}, false, stats, fmt.Errorf("workforce postgres: insert shift type if absent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.ShiftType{}, false, stats, fmt.Errorf("workforce postgres: insert shift type if absent rows affected: %w", err)
	}
	stats.Rows = affected
	return shiftTypeToDomain(*row), affected == 1, stats, nil
}

func (s *Store) UpdateShiftType(ctx context.Context, value domain.ShiftType) (domain.ShiftType, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ShiftType{}, false, domain.OperationStats{}, err
	}
	row := shiftTypeFromDomain(value)
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableShiftTypes+` AS "shift_type"`).
		Column("name", "color", "description", "is_active").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*")
	if tenantID > 0 {
		query = query.Where(`"shift_type".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, shiftTypeNameUniqueIndex) {
			return domain.ShiftType{}, false, stats, &domain.ConflictError{Kind: domain.ErrShiftTypeNameTaken, Cause: err}
		}
		return domain.ShiftType{}, false, stats, fmt.Errorf("workforce postgres: update shift type: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.ShiftType{}, false, stats, fmt.Errorf("workforce postgres: update shift type rows affected: %w", err)
	}
	if affected != 1 {
		return domain.ShiftType{}, false, stats, nil
	}
	stats.Rows = affected
	return shiftTypeToDomain(*row), true, stats, nil
}

// DeleteShiftType removes the type; shifts referencing it keep their row and
// lose only the reference through the FK's ON DELETE SET NULL.
func (s *Store) DeleteShiftType(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*shiftTypeRow)(nil)).
		ModelTableExpr(tableShiftTypes+` AS "shift_type"`).
		Where(`"shift_type".id = ?`, id), aliasShiftType, tenantID)
	return execAffected(ctx, query, "delete shift type")
}

// --- plumbing ---

// execCount runs a write and reports the affected rows next to the stats.
func execCount(ctx context.Context, query interface {
	Exec(context.Context, ...any) (sql.Result, error)
}, operation string) (int64, domain.OperationStats, error) {
	stats, err := execAffected(ctx, query, operation)
	return stats.Rows, stats, err
}

// requiredClock binds a wall clock to a TIME column. The clock is anchored at
// 0001-01-01 UTC: PostgreSQL rejects the year-0 anchor time.Parse produces,
// and the UTC location keeps the driver's conversion from moving the hour.
func requiredClock(value, field string) (time.Time, error) {
	parsed, err := time.Parse(domain.ClockLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("workforce postgres: %s %q is not a %s wall clock: %w", field, value, domain.ClockLayout, err)
	}
	return time.Date(1, time.January, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC), nil
}

// wallClockString reads the time-of-day components only, discarding the date
// anchor and location the driver attached on scan.
func wallClockString(value time.Time) string {
	return value.Format(domain.ClockLayout)
}

func staffShiftFromDomain(value domain.StaffShift) (*staffShiftRow, error) {
	start, err := requiredClock(value.StartTime, "start time")
	if err != nil {
		return nil, err
	}
	end, err := requiredClock(value.EndTime, "end time")
	if err != nil {
		return nil, err
	}
	return &staffShiftRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: calendarDate(value.Date),
		StartTime: start, EndTime: end, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, SeriesID: value.SeriesID, Detached: value.Detached,
		SeriesOccurrenceDate: optionalCalendarDate(value.SeriesOccurrenceDate), Cancelled: value.Cancelled,
		ChangeReason: value.ChangeReason, OriginShiftID: value.OriginShiftID, SickAbsenceID: value.SickAbsenceID,
		CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func staffShiftToDomain(row staffShiftRow) domain.StaffShift {
	return domain.StaffShift{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Date: string(row.Date),
		StartTime: wallClockString(row.StartTime), EndTime: wallClockString(row.EndTime), BreakMinutes: row.BreakMinutes,
		ShiftTypeID: row.ShiftTypeID, Notes: row.Notes, SeriesID: row.SeriesID, Detached: row.Detached,
		SeriesOccurrenceDate: calendarDateString(row.SeriesOccurrenceDate), Cancelled: row.Cancelled,
		ChangeReason: row.ChangeReason, OriginShiftID: row.OriginShiftID, SickAbsenceID: row.SickAbsenceID,
		CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffShiftsToDomain(rows []staffShiftRow) []domain.StaffShift {
	result := make([]domain.StaffShift, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffShiftToDomain(row))
	}
	return result
}

func staffShiftSeriesFromDomain(value domain.StaffShiftSeries) (*staffShiftSeriesRow, error) {
	start, err := requiredClock(value.StartTime, "start time")
	if err != nil {
		return nil, err
	}
	end, err := requiredClock(value.EndTime, "end time")
	if err != nil {
		return nil, err
	}
	weekdays := make([]int16, 0, len(value.Weekdays))
	for _, weekday := range value.Weekdays {
		weekdays = append(weekdays, int16(weekday)) // #nosec G115 -- validated ISO weekday 1..7
	}
	return &staffShiftSeriesRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Weekdays: weekdays,
		StartTime: start, EndTime: end, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, CalendarPeriodID: value.CalendarPeriodID, WeekPattern: value.WeekPattern,
		ValidFrom: calendarDate(value.ValidFrom), ValidUntil: optionalCalendarDate(value.ValidUntil),
		SeriesRootID: value.SeriesRootID, RetainedOccurrenceShiftID: value.RetainedOccurrenceShiftID,
		CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func staffShiftSeriesToDomain(row staffShiftSeriesRow) domain.StaffShiftSeries {
	weekdays := make([]int, 0, len(row.Weekdays))
	for _, weekday := range row.Weekdays {
		weekdays = append(weekdays, int(weekday))
	}
	return domain.StaffShiftSeries{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Weekdays: weekdays,
		StartTime: wallClockString(row.StartTime), EndTime: wallClockString(row.EndTime), BreakMinutes: row.BreakMinutes,
		ShiftTypeID: row.ShiftTypeID, Notes: row.Notes, CalendarPeriodID: row.CalendarPeriodID, WeekPattern: row.WeekPattern,
		ValidFrom: string(row.ValidFrom), ValidUntil: calendarDateString(row.ValidUntil),
		SeriesRootID: row.SeriesRootID, RetainedOccurrenceShiftID: row.RetainedOccurrenceShiftID,
		CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func shiftTypeFromDomain(value domain.ShiftType) *shiftTypeRow {
	return &shiftTypeRow{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, Color: value.Color,
		Description: value.Description, IsActive: value.IsActive, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func shiftTypeToDomain(row shiftTypeRow) domain.ShiftType {
	return domain.ShiftType{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, Color: row.Color,
		Description: row.Description, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
