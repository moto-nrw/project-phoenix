package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const deviationEventTableExpr = `audit.deviation_events AS "deviation_event"`

type deviationEventRepository struct {
	*base.Repository[*audit.DeviationEvent]
	db *bun.DB
}

func NewDeviationEventRepository(db *bun.DB) audit.DeviationEventRepository {
	repo := base.NewRepository[*audit.DeviationEvent](db, "audit.deviation_events", "DeviationEvent")
	repo.TenantScoped = true
	return &deviationEventRepository{Repository: repo, db: db}
}

func (r *deviationEventRepository) ListByRange(ctx context.Context, from, to timezone.Date, activityGroupID *int64, startTime *string) ([]*audit.DeviationEvent, error) {
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("date range is required")
	}
	var rows []*audit.DeviationEvent
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(deviationEventTableExpr).
		Where(`"deviation_event".occurrence_date >= ?`, from).
		Where(`"deviation_event".occurrence_date <= ?`, to).
		// Slot-anchored events only: shift-anchored rows (#1884 Dienstplan
		// moves) belong to the Dienstplan, not the Betreuungsplan history this
		// read serves. The event-type exclusion is the durable filter — the
		// staff_shift_id FK is ON DELETE SET NULL, so a deleted shift clears
		// the anchor; the IS NULL check stays as defense for future
		// shift-anchored types missing from the list. No shift-history read
		// surface exists yet; add a filter parameter here when one does.
		Where(`"deviation_event".event_type NOT IN (?)`, bun.List(audit.ShiftScopedDeviationEventTypes)).
		Where(`"deviation_event".staff_shift_id IS NULL`)
	// Defense-in-depth on top of RLS, same as the generic TenantScoped path.
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		q = q.Where(`"deviation_event".tenant_id = ?`, tenantID)
	}
	if activityGroupID != nil {
		q = q.Where(`"deviation_event".activity_group_id = ?`, *activityGroupID)
	}
	if startTime != nil {
		q = q.Where(`"deviation_event".start_time = ?`, *startTime)
	}
	if err := q.
		OrderExpr(`"deviation_event".occurred_at DESC, "deviation_event".id DESC`).
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list deviation events", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

func (r *deviationEventRepository) DeleteOlderThan(ctx context.Context, cutoff timezone.Date) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, &modelBase.DatabaseError{Op: "delete expired deviation events", Err: fmt.Errorf("tenant context is required")}
	}
	var deleted int64
	if err := base.GetDB(ctx, r.db).NewRaw(
		`SELECT audit.delete_expired_deviation_events(?, ?)`, tenantID, cutoff,
	).Scan(ctx, &deleted); err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete expired deviation events", Err: base.TranslateNotFound(err)}
	}
	return deleted, nil
}
