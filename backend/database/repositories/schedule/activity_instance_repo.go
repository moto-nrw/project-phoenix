package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableActivityInstances   = "schedule.activity_instances"
	aliasActivityInstance    = "activity_instance"
	modelTblActivityInstance = `schedule.activity_instances AS "activity_instance"`
)

var errActivityInstanceNil = fmt.Errorf("activity instance cannot be nil")

// ActivityInstanceRepository implements schedule.ActivityInstanceRepository.
type ActivityInstanceRepository struct {
	*base.Repository[*schedule.ActivityInstance]
	db *bun.DB
}

// NewActivityInstanceRepository creates a new ActivityInstanceRepository.
func NewActivityInstanceRepository(db *bun.DB) schedule.ActivityInstanceRepository {
	repo := base.NewRepository[*schedule.ActivityInstance](db, tableActivityInstances, "ActivityInstance")
	repo.TenantScoped = true
	return &ActivityInstanceRepository{
		Repository: repo,
		db:         db,
	}
}

// Create inserts a new activity instance after running model-level validation.
func (r *ActivityInstanceRepository) Create(ctx context.Context, i *schedule.ActivityInstance) error {
	if i == nil {
		return errActivityInstanceNil
	}
	if err := i.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, i)
}

// CreateTemplateBackedIfAbsent inserts a template-backed activity instance,
// absorbing the duplicate-template-slot race at the database layer. Catching a
// 23505 after a plain INSERT would leave the surrounding PostgreSQL
// transaction aborted, so materialization must use ON CONFLICT DO NOTHING.
func (r *ActivityInstanceRepository) CreateTemplateBackedIfAbsent(ctx context.Context, i *schedule.ActivityInstance) (bool, error) {
	if i == nil {
		return false, errActivityInstanceNil
	}
	if i.ActivityGroupID == nil {
		return false, fmt.Errorf("activity_group_id is required for template-backed insert")
	}
	if err := i.Validate(); err != nil {
		return false, err
	}

	if i.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			i.SetTenantID(tid)
		}
	}

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(i).
		ModelTableExpr(tableActivityInstances).
		On("CONFLICT (tenant_id, date, activity_group_id, start_time) WHERE activity_group_id IS NOT NULL DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "create template-backed if absent",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "create template-backed if absent rows affected",
			Err: err,
		}
	}
	return affected > 0, nil
}

// Update writes the given activity instance back to the database.
func (r *ActivityInstanceRepository) Update(ctx context.Context, i *schedule.ActivityInstance) error {
	if i == nil {
		return errActivityInstanceNil
	}
	if err := i.Validate(); err != nil {
		return err
	}
	return r.Repository.Update(ctx, i)
}

// MarkCompleted updates only lifecycle columns. It intentionally avoids the
// generic full-row Update path because SQL TIME columns on activity instances
// are unsafe to write back after a DB decode round-trip.
func (r *ActivityInstanceRepository) MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	inst := &schedule.ActivityInstance{
		Status:      schedule.InstanceStatusCompleted,
		CompletedAt: &completedAt,
	}
	inst.ID = instanceID

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model(inst).
		ModelTableExpr(modelTblActivityInstance).
		Column("status", "completed_at").
		Where(`"activity_instance".id = ?`, instanceID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		q = q.Where(`"activity_instance".tenant_id = ?`, tenantID)
	}

	result, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark completed", Err: err}
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return &modelBase.DatabaseError{Op: "mark completed", Err: sql.ErrNoRows}
	}
	return nil
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *ActivityInstanceRepository) FindByID(ctx context.Context, id any) (*schedule.ActivityInstance, error) {
	var instance schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instance).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, aliasActivityInstance); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}
	return &instance, nil
}

// List retrieves activity instances matching the provided query options.
func (r *ActivityInstanceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance)

	if where, val, ok := base.TenantWhere(ctx, aliasActivityInstance); ok {
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
	return instances, nil
}

// FindByTenantAndDate returns instances for the current tenant on a given date.
func (r *ActivityInstanceRepository) FindByTenantAndDate(ctx context.Context, date timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".date = ?`, date).
		Order("start_time ASC")

	if where, val, ok := base.TenantWhere(ctx, aliasActivityInstance); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant and date",
			Err: err,
		}
	}
	return instances, nil
}

// FindByTenantAndDateRange returns instances within an inclusive date range.
func (r *ActivityInstanceRepository) FindByTenantAndDateRange(ctx context.Context, from, to timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		Order("date ASC", "start_time ASC")

	if where, val, ok := base.TenantWhere(ctx, aliasActivityInstance); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant and date range",
			Err: err,
		}
	}
	return instances, nil
}

// FindByActivityGroupAndDate returns instances for a template on a given date.
func (r *ActivityInstanceRepository) FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".activity_group_id = ?`, activityGroupID).
		Where(`"activity_instance".date = ?`, date).
		Order("start_time ASC")

	if where, val, ok := base.TenantWhere(ctx, aliasActivityInstance); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by activity group and date",
			Err: err,
		}
	}
	return instances, nil
}

// FindByActiveGroupID returns the instance bridged to the given active.group,
// or nil if none exists. The bridge is 1:1 — enforced by a UNIQUE partial
// index on schedule.activity_instances(active_group_id) WHERE IS NOT NULL.
// The Limit(1) is belt-and-suspenders for windows where the index is not yet
// present (e.g. a test DB restored from a pre-fix dump).
func (r *ActivityInstanceRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*schedule.ActivityInstance, error) {
	var instance schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instance).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".active_group_id = ?`, activeGroupID)

	if where, val, ok := base.TenantWhere(ctx, aliasActivityInstance); ok {
		query = query.Where(where, val)
	}

	err := query.Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group id",
			Err: err,
		}
	}
	return &instance, nil
}
