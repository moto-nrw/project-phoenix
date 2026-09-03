package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// calendarPeriodNameConstraint is the per-tenant name uniqueness pinned by
// migration 1.15.33.
const calendarPeriodNameConstraint = "unique_calendar_period_name"

// Database resolves the connection and the tenant of the current request. A
// zero tenant means no tenant is bound: reads run unscoped, writes refuse.
type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("school calendar postgres: database runtime is required")
	}
	return &Store{database: database}
}

// calendarDate maps the module's YYYY-MM-DD strings onto PostgreSQL DATE
// columns. Binding the plain string keeps the driver from shifting the day
// through a timezone conversion; the column type tag lets PostgreSQL cast.
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

type calendarPeriodRow struct {
	bun.BaseModel   `bun:"table:calendar_periods,alias:calendar_period"`
	ID              int64         `bun:"id,pk,autoincrement"`
	CreatedAt       time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID        int64         `bun:"tenant_id,notnull"`
	Name            string        `bun:"name,notnull"`
	PeriodType      string        `bun:"period_type,notnull"`
	StartDate       calendarDate  `bun:"start_date,notnull,type:date"`
	EndDate         calendarDate  `bun:"end_date,notnull,type:date"`
	WeekCycleLength int           `bun:"week_cycle_length,notnull"`
	WeekCycleAnchor *calendarDate `bun:"week_cycle_anchor,type:date"`
	IsActive        bool          `bun:"is_active,notnull"`
}

type closingDayRow struct {
	bun.BaseModel `bun:"table:closing_days,alias:closing_day"`
	ID            int64        `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	StartDate     calendarDate `bun:"start_date,notnull,type:date"`
	EndDate       calendarDate `bun:"end_date,notnull,type:date"`
	Reason        string       `bun:"reason,notnull"`
}

// tenantDatabase resolves the connection for a write that must stay inside
// one tenant: without a bound tenant the row predicate would be missing and
// a superuser connection could reach another school's rows by bare id.
func (s *Store) tenantDatabase(ctx context.Context, operation string) (bun.IDB, int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if tenantID <= 0 {
		return nil, 0, fmt.Errorf("school calendar postgres: tenant is required to %s", operation)
	}
	return db, tenantID, nil
}

// --- calendar periods ---

func (s *Store) FindCalendarPeriod(ctx context.Context, id int64) (domain.CalendarPeriod, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.CalendarPeriod{}, false, domain.OperationStats{}, err
	}
	row := &calendarPeriodRow{}
	query := withTenant(calendarPeriodSelect(db, row).Where(`"calendar_period".id = ?`, id), "calendar_period", tenantID)
	found, stats, err := scanOne(ctx, query, "find calendar period")
	if err != nil || !found {
		return domain.CalendarPeriod{}, found, stats, err
	}
	return calendarPeriodToDomain(*row), true, stats, nil
}

func (s *Store) ListCalendarPeriods(ctx context.Context, filter domain.CalendarPeriodFilter) ([]domain.CalendarPeriod, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []calendarPeriodRow{}
	query := withTenant(calendarPeriodSelect(db, &rows), "calendar_period", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.CalendarPeriod{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"calendar_period".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.Name != "" {
		query = query.Where(`"calendar_period".name = ?`, filter.Name)
	}
	if filter.PeriodType != "" {
		query = query.Where(`"calendar_period".period_type = ?`, filter.PeriodType)
	}
	if filter.ActiveOnly {
		query = query.Where(`"calendar_period".is_active = ?`, true)
	}
	if filter.OverlappingFrom != "" {
		query = query.
			Where(`"calendar_period".start_date <= ?`, calendarDate(filter.OverlappingTo)).
			Where(`"calendar_period".end_date >= ?`, calendarDate(filter.OverlappingFrom))
	}
	if filter.ExcludeID > 0 {
		query = query.Where(`"calendar_period".id != ?`, filter.ExcludeID)
	}
	query = query.OrderExpr(`"calendar_period".start_date ASC, "calendar_period".id ASC`)
	stats, err := scanAll(ctx, query, "list calendar periods")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CalendarPeriod, 0, len(rows))
	for _, row := range rows {
		result = append(result, calendarPeriodToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateCalendarPeriod(ctx context.Context, fields domain.CalendarPeriodFields, ifAbsent bool) (domain.CalendarPeriod, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.CalendarPeriod{}, false, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.CalendarPeriod{}, false, domain.OperationStats{}, errors.New("school calendar postgres: tenant is required to create a calendar period")
	}
	row := calendarPeriodRow{TenantID: tenantID}
	applyCalendarPeriodFields(&row, fields)
	query := db.NewInsert().Model(&row).ModelTableExpr(`schedule.calendar_periods`)
	if ifAbsent {
		query = query.On("CONFLICT (tenant_id, name) DO NOTHING")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if ifAbsent && errors.Is(err, sql.ErrNoRows) {
		return domain.CalendarPeriod{}, false, stats, nil
	}
	if err != nil {
		return domain.CalendarPeriod{}, false, stats, wrapCalendarPeriodWriteError("create", err)
	}
	stats.Rows = 1
	return calendarPeriodToDomain(row), true, stats, nil
}

func (s *Store) UpdateCalendarPeriod(ctx context.Context, id int64, fields domain.CalendarPeriodFields) (domain.CalendarPeriod, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "update a calendar period")
	if err != nil {
		return domain.CalendarPeriod{}, domain.OperationStats{}, err
	}
	row := calendarPeriodRow{ID: id}
	applyCalendarPeriodFields(&row, fields)
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Column("name", "period_type", "start_date", "end_date", "week_cycle_length", "week_cycle_anchor", "is_active").
		Set(`updated_at = NOW()`).
		Where(`"calendar_period".id = ?`, id), "calendar_period", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CalendarPeriod{}, stats, domain.ErrCalendarPeriodNotFound
	}
	if err != nil {
		return domain.CalendarPeriod{}, stats, wrapCalendarPeriodWriteError("update", err)
	}
	stats.Rows = 1
	return calendarPeriodToDomain(row), stats, nil
}

func (s *Store) DeleteCalendarPeriod(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "delete a calendar period")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*calendarPeriodRow)(nil)).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".id = ?`, id), "calendar_period", tenantID)
	return execDelete(ctx, query, "delete calendar period", domain.ErrCalendarPeriodNotFound)
}

// --- closing days ---

func (s *Store) FindClosingDay(ctx context.Context, id int64) (domain.ClosingDay, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ClosingDay{}, false, domain.OperationStats{}, err
	}
	row := &closingDayRow{}
	query := withTenant(closingDaySelect(db, row).Where(`"closing_day".id = ?`, id), "closing_day", tenantID)
	found, stats, err := scanOne(ctx, query, "find closing day")
	if err != nil || !found {
		return domain.ClosingDay{}, found, stats, err
	}
	return closingDayToDomain(*row), true, stats, nil
}

func (s *Store) ListClosingDays(ctx context.Context, filter domain.ClosingDayFilter) ([]domain.ClosingDay, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []closingDayRow{}
	query := withTenant(closingDaySelect(db, &rows), "closing_day", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.ClosingDay{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"closing_day".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.OverlappingFrom != "" {
		query = query.
			Where(`"closing_day".start_date <= ?`, calendarDate(filter.OverlappingTo)).
			Where(`"closing_day".end_date >= ?`, calendarDate(filter.OverlappingFrom))
	}
	query = query.OrderExpr(`"closing_day".start_date ASC, "closing_day".id ASC`)
	stats, err := scanAll(ctx, query, "list closing days")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.ClosingDay, 0, len(rows))
	for _, row := range rows {
		result = append(result, closingDayToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateClosingDay(ctx context.Context, fields domain.ClosingDayFields) (domain.ClosingDay, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ClosingDay{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.ClosingDay{}, domain.OperationStats{}, errors.New("school calendar postgres: tenant is required to create a closing day")
	}
	row := closingDayRow{TenantID: tenantID, StartDate: calendarDate(fields.StartDate), EndDate: calendarDate(fields.EndDate), Reason: fields.Reason}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.closing_days`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ClosingDay{}, stats, fmt.Errorf("school calendar postgres: create closing day: %w", err)
	}
	stats.Rows = 1
	return closingDayToDomain(row), stats, nil
}

func (s *Store) UpdateClosingDay(ctx context.Context, id int64, fields domain.ClosingDayFields) (domain.ClosingDay, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "update a closing day")
	if err != nil {
		return domain.ClosingDay{}, domain.OperationStats{}, err
	}
	row := closingDayRow{ID: id, StartDate: calendarDate(fields.StartDate), EndDate: calendarDate(fields.EndDate), Reason: fields.Reason}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`schedule.closing_days AS "closing_day"`).
		Column("start_date", "end_date", "reason").
		Set(`updated_at = NOW()`).
		Where(`"closing_day".id = ?`, id), "closing_day", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ClosingDay{}, stats, domain.ErrClosingDayNotFound
	}
	if err != nil {
		return domain.ClosingDay{}, stats, fmt.Errorf("school calendar postgres: update closing day: %w", err)
	}
	stats.Rows = 1
	return closingDayToDomain(row), stats, nil
}

func (s *Store) DeleteClosingDay(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "delete a closing day")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*closingDayRow)(nil)).
		ModelTableExpr(`schedule.closing_days AS "closing_day"`).
		Where(`"closing_day".id = ?`, id), "closing_day", tenantID)
	return execDelete(ctx, query, "delete closing day", domain.ErrClosingDayNotFound)
}

// --- dateframes ---

type dateframeRow struct {
	bun.BaseModel `bun:"table:dateframes,alias:dateframe"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StartDate     time.Time `bun:"start_date,notnull"`
	EndDate       time.Time `bun:"end_date,notnull"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
}

func (s *Store) FindDateframe(ctx context.Context, id int64) (domain.Dateframe, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Dateframe{}, false, domain.OperationStats{}, err
	}
	row := &dateframeRow{}
	query := withTenant(dateframeSelect(db, row).Where(`"dateframe".id = ?`, id), "dateframe", tenantID)
	found, stats, err := scanOne(ctx, query, "find dateframe")
	if err != nil || !found {
		return domain.Dateframe{}, found, stats, err
	}
	return dateframeToDomain(*row), true, stats, nil
}

func (s *Store) ListDateframes(ctx context.Context, filter domain.DateframeFilter) ([]domain.Dateframe, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []dateframeRow{}
	query := withTenant(dateframeSelect(db, &rows), "dateframe", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.Dateframe{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"dateframe".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.Name != "" {
		query = query.Where(`"dateframe".name = ?`, filter.Name)
	}
	if filter.NameFold != "" {
		query = query.Where(`LOWER("dateframe".name) = LOWER(?)`, filter.NameFold)
	}
	if filter.NamePattern != "" {
		query = query.Where(`"dateframe".name ILIKE ?`, filter.NamePattern)
	}
	if filter.Contains != nil {
		query = query.
			Where(`"dateframe".start_date <= ?`, *filter.Contains).
			Where(`"dateframe".end_date >= ?`, *filter.Contains)
	}
	if filter.OverlappingFrom != nil && filter.OverlappingTo != nil {
		query = query.
			Where(`"dateframe".start_date <= ?`, *filter.OverlappingTo).
			Where(`"dateframe".end_date >= ?`, *filter.OverlappingFrom)
	}
	if len(filter.Sort) == 0 {
		query = query.OrderExpr(`"dateframe".id ASC`)
	}
	for _, sort := range filter.Sort {
		query, err = orderDateframes(query, sort)
		if err != nil {
			return nil, domain.OperationStats{}, err
		}
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	stats, err := scanAll(ctx, query, "list dateframes")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Dateframe, 0, len(rows))
	for _, row := range rows {
		result = append(result, dateframeToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateDateframe(ctx context.Context, fields domain.DateframeFields) (domain.Dateframe, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Dateframe{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Dateframe{}, domain.OperationStats{}, errors.New("school calendar postgres: tenant is required to create a dateframe")
	}
	row := dateframeRow{TenantID: tenantID, StartDate: fields.StartDate, EndDate: fields.EndDate, Name: fields.Name, Description: fields.Description}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.dateframes`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Dateframe{}, stats, fmt.Errorf("school calendar postgres: create dateframe: %w", err)
	}
	stats.Rows = 1
	return dateframeToDomain(row), stats, nil
}

func (s *Store) UpdateDateframe(ctx context.Context, id int64, fields domain.DateframeFields) (domain.Dateframe, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "update a dateframe")
	if err != nil {
		return domain.Dateframe{}, domain.OperationStats{}, err
	}
	row := dateframeRow{ID: id, StartDate: fields.StartDate, EndDate: fields.EndDate, Name: fields.Name, Description: fields.Description}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`schedule.dateframes AS "dateframe"`).
		Column("start_date", "end_date", "name", "description").
		Set(`updated_at = NOW()`).
		Where(`"dateframe".id = ?`, id), "dateframe", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Dateframe{}, stats, domain.ErrDateframeNotFound
	}
	if err != nil {
		return domain.Dateframe{}, stats, fmt.Errorf("school calendar postgres: update dateframe: %w", err)
	}
	stats.Rows = 1
	return dateframeToDomain(row), stats, nil
}

func (s *Store) DeleteDateframe(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "delete a dateframe")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*dateframeRow)(nil)).
		ModelTableExpr(`schedule.dateframes AS "dateframe"`).
		Where(`"dateframe".id = ?`, id), "dateframe", tenantID)
	return execDelete(ctx, query, "delete dateframe", domain.ErrDateframeNotFound)
}

func dateframeSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.dateframes AS "dateframe"`)
}

// orderDateframes appends one sort field. Every order expression is a
// literal so the sortable columns stay a closed, statically visible set.
func orderDateframes(query *bun.SelectQuery, sort domain.DateframeSort) (*bun.SelectQuery, error) {
	switch {
	case sort.Field == "id" && !sort.Descending:
		return query.OrderExpr(`"dateframe".id ASC`), nil
	case sort.Field == "id":
		return query.OrderExpr(`"dateframe".id DESC`), nil
	case sort.Field == "name" && !sort.Descending:
		return query.OrderExpr(`"dateframe".name ASC`), nil
	case sort.Field == "name":
		return query.OrderExpr(`"dateframe".name DESC`), nil
	case sort.Field == "start_date" && !sort.Descending:
		return query.OrderExpr(`"dateframe".start_date ASC`), nil
	case sort.Field == "start_date":
		return query.OrderExpr(`"dateframe".start_date DESC`), nil
	case sort.Field == "end_date" && !sort.Descending:
		return query.OrderExpr(`"dateframe".end_date ASC`), nil
	case sort.Field == "end_date":
		return query.OrderExpr(`"dateframe".end_date DESC`), nil
	default:
		return nil, fmt.Errorf("school calendar postgres: unsupported dateframe sort field %q", sort.Field)
	}
}

func dateframeToDomain(row dateframeRow) domain.Dateframe {
	return domain.Dateframe{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartDate: row.StartDate, EndDate: row.EndDate, Name: row.Name, Description: row.Description,
	}
}

// --- plumbing ---

func calendarPeriodSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`)
}

func closingDaySelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.closing_days AS "closing_day"`)
}

func applyCalendarPeriodFields(row *calendarPeriodRow, fields domain.CalendarPeriodFields) {
	row.Name = fields.Name
	row.PeriodType = fields.PeriodType
	row.StartDate = calendarDate(fields.StartDate)
	row.EndDate = calendarDate(fields.EndDate)
	row.WeekCycleLength = fields.WeekCycleLength
	row.WeekCycleAnchor = optionalCalendarDate(fields.WeekCycleAnchor)
	row.IsActive = fields.IsActive
}

func calendarPeriodToDomain(row calendarPeriodRow) domain.CalendarPeriod {
	return domain.CalendarPeriod{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, PeriodType: row.PeriodType,
		StartDate: string(row.StartDate), EndDate: string(row.EndDate),
		WeekCycleLength: row.WeekCycleLength, WeekCycleAnchor: calendarDateString(row.WeekCycleAnchor),
		IsActive: row.IsActive,
	}
}

func closingDayToDomain(row closingDayRow) domain.ClosingDay {
	return domain.ClosingDay{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartDate: string(row.StartDate), EndDate: string(row.EndDate), Reason: row.Reason,
	}
}

func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
	}
	return query
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
		return false, stats, fmt.Errorf("school calendar postgres: %s: %w", operation, err)
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
		return stats, fmt.Errorf("school calendar postgres: %s: %w", operation, err)
	}
	return stats, nil
}

// execDelete keeps the driver error in the chain: a period still referenced
// by a non-nullable foreign key surfaces its constraint violation exactly as
// the legacy repository did.
func execDelete(ctx context.Context, query *bun.DeleteQuery, operation string, notFound error) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school calendar postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("school calendar postgres: %s: count rows: %w", operation, err)
	}
	if rows != 1 {
		return stats, notFound
	}
	stats.Rows = rows
	return stats, nil
}

// wrapCalendarPeriodWriteError classifies the per-tenant name collision but
// keeps the driver error in the chain for callers that inspect it.
func wrapCalendarPeriodWriteError(operation string, err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == calendarPeriodNameConstraint {
		return fmt.Errorf("%w: %w", domain.ErrCalendarPeriodNameConflict, err)
	}
	return fmt.Errorf("school calendar postgres: %s calendar period: %w", operation, err)
}
