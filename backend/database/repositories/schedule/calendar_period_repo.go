package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const tableCalendarPeriods = "schedule.calendar_periods"

var errCalendarPeriodNil = fmt.Errorf("calendar period cannot be nil")

// CalendarPeriodRepository implements schedule.CalendarPeriodRepository
type CalendarPeriodRepository struct {
	*base.Repository[*schedule.CalendarPeriod]
	db *bun.DB
}

// NewCalendarPeriodRepository creates a new CalendarPeriodRepository
func NewCalendarPeriodRepository(db *bun.DB) schedule.CalendarPeriodRepository {
	repo := base.NewRepository[*schedule.CalendarPeriod](db, tableCalendarPeriods, "CalendarPeriod")
	repo.TenantScoped = true
	return &CalendarPeriodRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByTenantID finds all calendar periods for the current tenant
func (r *CalendarPeriodRepository) FindByTenantID(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Order("start_date ASC")

	if where, val, ok := base.TenantWhere(ctx, "calendar_period"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant id",
			Err: err,
		}
	}

	return periods, nil
}

// FindActiveByTenantID finds all active calendar periods for the current tenant
func (r *CalendarPeriodRepository) FindActiveByTenantID(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".is_active = ?`, true).
		Order("start_date ASC")

	if where, val, ok := base.TenantWhere(ctx, "calendar_period"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by tenant id",
			Err: err,
		}
	}

	return periods, nil
}

// FindByName finds a calendar period by name within the current tenant
func (r *CalendarPeriodRepository) FindByName(ctx context.Context, name string) (*schedule.CalendarPeriod, error) {
	var period schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&period).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".name = ?`, name)

	if where, val, ok := base.TenantWhere(ctx, "calendar_period"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return &period, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *CalendarPeriodRepository) FindByID(ctx context.Context, id any) (*schedule.CalendarPeriod, error) {
	var period schedule.CalendarPeriod

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&period).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "calendar_period"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}

	return &period, nil
}

// Create overrides the base Create method to handle validation
func (r *CalendarPeriodRepository) Create(ctx context.Context, p *schedule.CalendarPeriod) error {
	if p == nil {
		return errCalendarPeriodNil
	}

	if err := p.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, p)
}

// Update overrides the base Update method to handle validation
func (r *CalendarPeriodRepository) Update(ctx context.Context, p *schedule.CalendarPeriod) error {
	if p == nil {
		return errCalendarPeriodNil
	}

	if err := p.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, p)
}

// List retrieves calendar periods matching the provided query options
func (r *CalendarPeriodRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`)

	if where, val, ok := base.TenantWhere(ctx, "calendar_period"); ok {
		query = query.Where(where, val)
	}

	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return periods, nil
}
