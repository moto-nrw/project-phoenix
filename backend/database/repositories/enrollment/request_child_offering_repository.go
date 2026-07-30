package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestChildOfferingTableExpr = `enrollment.request_child_offerings AS "request_child_offering"`

type RequestChildOfferingRepository struct {
	db *bun.DB
}

// ScheduleReplacementForRequestChild preserves the selection that is active
// before effectiveFrom and writes the replacement as a subsequent interval.
// Rows scheduled after effectiveFrom are superseded by the newer decision.
func (r *RequestChildOfferingRepository) ScheduleReplacementForRequestChild(ctx context.Context, requestChildID int64, effectiveFrom timezone.Date, rows []*enrollment.RequestChildOffering) error {
	if requestChildID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	if effectiveFrom.IsZero() {
		return fmt.Errorf("effective_from is required")
	}
	db := base.GetDB(ctx, r.db)
	if err := lockRequestChildOfferings(ctx, db, requestChildID); err != nil {
		return err
	}
	deleteQuery := db.NewDelete().
		Model((*enrollment.RequestChildOffering)(nil)).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		Where(`"request_child_offering".valid_from >= ?`, effectiveFrom)
	deleteQuery = base.WithTenantFilter(ctx, deleteQuery, "request_child_offering")
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete superseded scheduled request child offerings: %w", err)
	}
	updateQuery := db.NewUpdate().
		Model((*enrollment.RequestChildOffering)(nil)).
		ModelTableExpr(requestChildOfferingTableExpr).
		Set(`valid_until = ?`, effectiveFrom).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		Where(`("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from < ?)`, effectiveFrom).
		Where(`("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)`, effectiveFrom)
	updateQuery = base.WithTenantFilter(ctx, updateQuery, "request_child_offering")
	if _, err := updateQuery.Exec(ctx); err != nil {
		return fmt.Errorf("failed to close current request child offerings: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if row == nil {
			return fmt.Errorf("request child offering row cannot be nil")
		}
		row.RequestChildID = requestChildID
		row.ValidFrom = &effectiveFrom
		row.ValidUntil = nil
		base.EnsureTenantID(ctx, row)
	}
	if _, err := db.NewInsert().Model(&rows).ModelTableExpr("enrollment.request_child_offerings").Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert scheduled request child offerings: %w", err)
	}
	return nil
}

func NewRequestChildOfferingRepository(db *bun.DB) enrollment.RequestChildOfferingRepository {
	return &RequestChildOfferingRepository{db: db}
}

// Create inserts a new request_child × care_offering link. PR 7's
// submission service is the primary caller; PR 6 ships only the
// schema + plumbing.
func (r *RequestChildOfferingRepository) Create(ctx context.Context, row *enrollment.RequestChildOffering) error {
	base.EnsureTenantID(ctx, row)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(requestChildOfferingTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request child offering: %w", err)
	}
	return nil
}

// ReplaceForRequestChild atomically replaces all offering links for one
// request child. Callers must run it in the surrounding tenant transaction
// when the replacement is part of a larger workflow.
func (r *RequestChildOfferingRepository) ReplaceForRequestChild(ctx context.Context, requestChildID int64, rows []*enrollment.RequestChildOffering) error {
	if requestChildID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	db := base.GetDB(ctx, r.db)
	if err := lockRequestChildOfferings(ctx, db, requestChildID); err != nil {
		return err
	}
	deleteQuery := db.NewDelete().
		Model((*enrollment.RequestChildOffering)(nil)).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID)
	deleteQuery = base.WithTenantFilter(ctx, deleteQuery, "request_child_offering")
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete request child offerings: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if row == nil {
			return fmt.Errorf("request child offering row cannot be nil")
		}
		row.RequestChildID = requestChildID
		base.EnsureTenantID(ctx, row)
	}
	if _, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr("enrollment.request_child_offerings").
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert replacement request child offerings: %w", err)
	}
	return nil
}

// lockRequestChildOfferings serializes the delete/update/insert replacement
// sequence for one child. Locking offering rows protects capacity across
// children; this row lock protects the child's own interval history.
func lockRequestChildOfferings(ctx context.Context, db bun.IDB, requestChildID int64) error {
	var lockedID int64
	if err := db.NewSelect().
		TableExpr(`enrollment.request_children AS "request_child"`).
		ColumnExpr(`"request_child".id`).
		Where(`"request_child".id = ?`, requestChildID).
		For("UPDATE").
		Scan(ctx, &lockedID); err != nil {
		return fmt.Errorf("failed to lock request child offerings: %w", err)
	}
	return nil
}

// ListByRequestChildID returns the selection active today. Historical rows
// remain available through ListHistoryByRequestChildID.
func (r *RequestChildOfferingRepository) ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*enrollment.RequestChildOffering, error) {
	return r.ListByRequestChildIDAtDate(ctx, requestChildID, timezone.TodayDate())
}

// ListHistoryByRequestChildID returns all offering intervals for reporting.
func (r *RequestChildOfferingRepository) ListHistoryByRequestChildID(ctx context.Context, requestChildID int64) ([]*enrollment.RequestChildOffering, error) {
	var rows []*enrollment.RequestChildOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		OrderExpr(`"request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings: %w", err)
	}
	return rows, nil
}

// ListByRequestChildIDAtDate returns the selection active on onDate.
func (r *RequestChildOfferingRepository) ListByRequestChildIDAtDate(ctx context.Context, requestChildID int64, onDate timezone.Date) ([]*enrollment.RequestChildOffering, error) {
	var rows []*enrollment.RequestChildOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		Where(`("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from <= ?)`, onDate).
		Where(`("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)`, onDate).
		OrderExpr(`"request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings: %w", err)
	}
	return rows, nil
}

// ListByRequestChildIDs returns every historical offering link across the
// given children in a single query, sorted by request_child_id, id. Powers
// the phase export's N+1-free load. Empty input short-circuits to an empty
// slice. Tenant-scoped via RLS.
func (r *RequestChildOfferingRepository) ListByRequestChildIDs(ctx context.Context, requestChildIDs []int64) ([]*enrollment.RequestChildOffering, error) {
	if len(requestChildIDs) == 0 {
		return nil, nil
	}
	var rows []*enrollment.RequestChildOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id IN (?)`, bun.List(requestChildIDs)).
		OrderExpr(`"request_child_offering".request_child_id, "request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings by child ids: %w", err)
	}
	return rows, nil
}

// ListByRequestChildIDsAtDate returns the offering links active on onDate
// across the given children in one query.
func (r *RequestChildOfferingRepository) ListByRequestChildIDsAtDate(ctx context.Context, requestChildIDs []int64, onDate timezone.Date) ([]*enrollment.RequestChildOffering, error) {
	if len(requestChildIDs) == 0 {
		return nil, nil
	}
	var rows []*enrollment.RequestChildOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id IN (?)`, bun.List(requestChildIDs)).
		Where(`("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from <= ?)`, onDate).
		Where(`("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)`, onDate).
		OrderExpr(`"request_child_offering".request_child_id, "request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings by child ids at date: %w", err)
	}
	return rows, nil
}

// CountActiveByCareOffering returns the count of children currently
// holding (or competing for) a slot in the given care offering. Joins
// to enrollment.request_children and filters out terminal statuses
// (rejected, withdrawn) so the count reflects what the admin is
// actually managing. Tenant-scoped via RLS on both tables.
func (r *RequestChildOfferingRepository) CountActiveByCareOffering(ctx context.Context, careOfferingID int64) (int, error) {
	return r.CountActiveByCareOfferingOnDate(ctx, careOfferingID, timezone.TodayDate())
}

// CountActiveByCareOfferingOnDate counts slots held on the requested date.
func (r *RequestChildOfferingRepository) CountActiveByCareOfferingOnDate(ctx context.Context, careOfferingID int64, onDate timezone.Date) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(requestChildOfferingTableExpr).
		Join(`INNER JOIN enrollment.request_children AS "child" ON "child".id = "request_child_offering".request_child_id`).
		Where(`"request_child_offering".care_offering_id = ?`, careOfferingID).
		Where(`("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from <= ?)`, onDate).
		Where(`("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)`, onDate).
		Where(`"child".status NOT IN (?)`, bun.List([]string{
			enrollment.ChildStatusRejected,
			enrollment.ChildStatusWithdrawn,
		})).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count active children for care offering %d: %w", careOfferingID, err)
	}
	return count, nil
}

// CountMaxActiveByCareOfferingInRange returns the maximum number of children
// holding a slot at the same time within [from, until). Looking only at the
// first day misses a later approved offering change; summing intervals would
// instead double-count children whose non-overlapping intervals share a slot.
func (r *RequestChildOfferingRepository) CountMaxActiveByCareOfferingInRange(
	ctx context.Context,
	careOfferingID int64,
	from, until timezone.Date,
) (int, error) {
	return r.countMaxActiveByCareOfferingInRange(ctx, careOfferingID, 0, from, until)
}

// CountMaxActiveByCareOfferingInRangeExcludingRequestChild returns the
// maximum occupancy in [from, until) after excluding one child's intervals.
// This models a dated replacement before its old rows have been removed.
func (r *RequestChildOfferingRepository) CountMaxActiveByCareOfferingInRangeExcludingRequestChild(
	ctx context.Context,
	careOfferingID, requestChildID int64,
	from, until timezone.Date,
) (int, error) {
	if requestChildID <= 0 {
		return 0, fmt.Errorf("request_child_id is required")
	}
	return r.countMaxActiveByCareOfferingInRange(ctx, careOfferingID, requestChildID, from, until)
}

func (r *RequestChildOfferingRepository) countMaxActiveByCareOfferingInRange(
	ctx context.Context,
	careOfferingID, excludedRequestChildID int64,
	from, until timezone.Date,
) (int, error) {
	if !from.Before(until) {
		return 0, fmt.Errorf("capacity range must not be empty")
	}
	var count int
	err := base.GetDB(ctx, r.db).NewRaw(`
		WITH intervals AS (
			SELECT
				"request_child_offering".request_child_id,
				GREATEST(COALESCE("request_child_offering".valid_from, ?), ?) AS starts_at,
				LEAST(COALESCE("request_child_offering".valid_until, ?), ?) AS ends_at
			FROM enrollment.request_child_offerings AS "request_child_offering"
			INNER JOIN enrollment.request_children AS "child"
				ON "child".id = "request_child_offering".request_child_id
			WHERE "request_child_offering".care_offering_id = ?
				AND ("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from < ?)
				AND ("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)
				AND (? = 0 OR "request_child_offering".request_child_id <> ?)
				AND "child".status NOT IN (?)
		), boundaries AS (
			SELECT starts_at AS boundary FROM intervals
			UNION
			SELECT ends_at AS boundary FROM intervals
		)
		SELECT COALESCE(MAX((
			SELECT COUNT(DISTINCT interval_row.request_child_id)
			FROM intervals AS interval_row
			WHERE interval_row.starts_at <= boundaries.boundary
				AND interval_row.ends_at > boundaries.boundary
		)), 0)
		FROM boundaries
	`, from, from, until, until, careOfferingID, until, from, excludedRequestChildID, excludedRequestChildID,
		bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn}),
	).Scan(ctx, &count)
	if err != nil {
		return 0, fmt.Errorf("failed to count peak active children for care offering %d: %w", careOfferingID, err)
	}
	return count, nil
}

// CountMaterializableByCareOffering counts non-terminal selections that are
// active today or scheduled for the future. Completed intervals no longer need
// an offering template and must not block catalog maintenance.
func (r *RequestChildOfferingRepository) CountMaterializableByCareOffering(ctx context.Context, careOfferingID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(requestChildOfferingTableExpr).
		Join(`INNER JOIN enrollment.request_children AS "child" ON "child".id = "request_child_offering".request_child_id`).
		Where(`"request_child_offering".care_offering_id = ?`, careOfferingID).
		Where(`("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)`, timezone.TodayDate()).
		Where(`"child".status NOT IN (?)`, bun.List([]string{
			enrollment.ChildStatusRejected,
			enrollment.ChildStatusWithdrawn,
		})).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count materializable children for care offering %d: %w", careOfferingID, err)
	}
	return count, nil
}
