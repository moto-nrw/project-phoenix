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
	_, phaseEndExclusive, err := requestChildOfferingValidityWindow(ctx, db, requestChildID)
	if err != nil {
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
		row.ValidUntil = &phaseEndExclusive
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
	if row == nil || row.RequestChildID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	phaseStart, phaseEndExclusive, err := requestChildOfferingValidityWindow(ctx, base.GetDB(ctx, r.db), row.RequestChildID)
	if err != nil {
		return err
	}
	setRequestChildOfferingValidity(row, phaseStart, phaseEndExclusive)
	base.EnsureTenantID(ctx, row)

	_, err = base.GetDB(ctx, r.db).NewInsert().
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
	phaseStart, phaseEndExclusive, err := requestChildOfferingValidityWindow(ctx, db, requestChildID)
	if err != nil {
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
		setRequestChildOfferingValidity(row, phaseStart, phaseEndExclusive)
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

// requestChildOfferingValidityWindow returns the phase window for a child
// selection. All offering links must be bounded to this window: capacity is
// calculated from these intervals and care offerings can be reused in later
// phases.
func requestChildOfferingValidityWindow(
	ctx context.Context,
	db bun.IDB,
	requestChildID int64,
) (timezone.Date, timezone.Date, error) {
	var window struct {
		Start timezone.Date `bun:"service_start_date"`
		End   timezone.Date `bun:"service_end_date"`
	}
	err := db.NewSelect().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id`).
		Join(`INNER JOIN enrollment.phases AS "phase" ON "phase".id = "request".phase_id`).
		ColumnExpr(`"phase".service_start_date`).
		ColumnExpr(`"phase".service_end_date`).
		Where(`"request_child".id = ?`, requestChildID).
		Scan(ctx, &window)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, fmt.Errorf("find care period for request child %d: %w", requestChildID, err)
	}
	return window.Start, window.End.AddDays(1), nil
}

func setRequestChildOfferingValidity(
	row *enrollment.RequestChildOffering,
	phaseStart, phaseEndExclusive timezone.Date,
) {
	if row.ValidFrom == nil {
		row.ValidFrom = &phaseStart
	}
	if row.ValidUntil == nil {
		row.ValidUntil = &phaseEndExclusive
	}
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

// ListByRequestChildID returns every selection interval for one child.
// Callers that need point-in-time semantics must use
// ListByRequestChildIDAtDate explicitly.
func (r *RequestChildOfferingRepository) ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*enrollment.RequestChildOffering, error) {
	return r.ListHistoryByRequestChildID(ctx, requestChildID)
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

// ListByRequestChildIDAtDate returns the selection active on onDate. Before a
// phase starts it returns the upcoming selection so a future enrollment stays
// visible. It deliberately returns no rows for a gap in an active phase:
// callers use it to evaluate the booking at that exact date.
func (r *RequestChildOfferingRepository) ListByRequestChildIDAtDate(ctx context.Context, requestChildID int64, onDate timezone.Date) ([]*enrollment.RequestChildOffering, error) {
	var rows []*enrollment.RequestChildOffering
	db := base.GetDB(ctx, r.db)
	err := db.NewSelect().
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
	if len(rows) > 0 {
		return rows, nil
	}

	var phaseStart timezone.Date
	err = db.NewSelect().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id`).
		Join(`INNER JOIN enrollment.phases AS "phase" ON "phase".id = "request".phase_id`).
		ColumnExpr(`"phase".service_start_date`).
		Where(`"request_child".id = ?`, requestChildID).
		Scan(ctx, &phaseStart)
	if err != nil {
		return nil, fmt.Errorf("failed to find request child care period: %w", err)
	}
	if !onDate.Before(phaseStart) {
		return nil, nil
	}

	nextValidFrom := db.NewSelect().
		TableExpr(`enrollment.request_child_offerings AS "next_request_child_offering"`).
		ColumnExpr(`MIN("next_request_child_offering".valid_from)`).
		Where(`"next_request_child_offering".request_child_id = ?`, requestChildID).
		Where(`"next_request_child_offering".valid_from > ?`, onDate)
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		Where(`"request_child_offering".valid_from = (?)`, nextValidFrom).
		OrderExpr(`"request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list upcoming request child offerings: %w", err)
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

func (r *RequestChildOfferingRepository) ListByRequestChildIDsAtDates(ctx context.Context, dates map[int64]timezone.Date) ([]*enrollment.RequestChildOffering, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(dates))
	for id := range dates {
		ids = append(ids, id)
	}
	var history []*enrollment.RequestChildOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&history).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id IN (?)`, bun.List(ids)).
		OrderExpr(`"request_child_offering".request_child_id, "request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings by child ids at dates: %w", err)
	}
	rows := make([]*enrollment.RequestChildOffering, 0, len(history))
	for _, row := range history {
		onDate, ok := dates[row.RequestChildID]
		if !ok || (row.ValidFrom != nil && row.ValidFrom.After(onDate)) || (row.ValidUntil != nil && !row.ValidUntil.After(onDate)) {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// CountActiveByCareOffering returns the number of non-terminal selections
// across all intervals. Capacity decisions must use the date/range-specific
// methods below; this legacy aggregate intentionally remains time-agnostic.
func (r *RequestChildOfferingRepository) CountActiveByCareOffering(ctx context.Context, careOfferingID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(requestChildOfferingTableExpr).
		Join(`INNER JOIN enrollment.request_children AS "child" ON "child".id = "request_child_offering".request_child_id`).
		Where(`"request_child_offering".care_offering_id = ?`, careOfferingID).
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
	return r.countMaxActiveByCareOfferingInRange(ctx, careOfferingID, nil, from, until)
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
	return r.CountMaxActiveByCareOfferingInRangeExcludingRequestChildren(ctx, careOfferingID, []int64{requestChildID}, from, until)
}

// CountMaxActiveByCareOfferingInRangeExcludingRequestChildren returns the
// peak occupancy in [from, until) after removing the supplied children. Batch
// replacement uses this instead of subtracting historical link counts from a
// concurrent occupancy peak.
func (r *RequestChildOfferingRepository) CountMaxActiveByCareOfferingInRangeExcludingRequestChildren(
	ctx context.Context,
	careOfferingID int64,
	requestChildIDs []int64,
	from, until timezone.Date,
) (int, error) {
	return r.countMaxActiveByCareOfferingInRange(ctx, careOfferingID, requestChildIDs, from, until)
}

func (r *RequestChildOfferingRepository) countMaxActiveByCareOfferingInRange(
	ctx context.Context,
	careOfferingID int64,
	excludedRequestChildIDs []int64,
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
				AND (? = 0 OR "request_child_offering".request_child_id NOT IN (?))
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
	`, from, from, until, until, careOfferingID, until, from, len(excludedRequestChildIDs), bun.List(excludedRequestChildIDs),
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

// CountMaxActiveByCareOfferingIDsInRange is the batched form of
// CountMaxActiveByCareOfferingInRange: one sweep-line pass over all the given
// offerings instead of a query each (#2186). Returns peak simultaneous
// occupancy per offering; offerings with no overlapping booking are absent
// from the map, so callers read a missing key as zero.
func (r *RequestChildOfferingRepository) CountMaxActiveByCareOfferingIDsInRange(
	ctx context.Context,
	phaseID int64,
	careOfferingIDs []int64,
	from, until timezone.Date,
) (map[int64]int, error) {
	result := make(map[int64]int)
	if len(careOfferingIDs) == 0 {
		return result, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("capacity range must not be empty")
	}
	var rows []struct {
		CareOfferingID int64 `bun:"care_offering_id"`
		Peak           int   `bun:"peak"`
	}
	// Same sweep-line as the single-offering variant, partitioned by
	// offering: count distinct children whose clamped interval covers each
	// interval boundary, then take the per-offering maximum.
	err := base.GetDB(ctx, r.db).NewRaw(`
		WITH intervals AS (
			SELECT
				"request_child_offering".care_offering_id AS care_offering_id,
				"request_child_offering".request_child_id AS request_child_id,
				GREATEST(COALESCE("request_child_offering".valid_from, ?), ?) AS starts_at,
				LEAST(COALESCE("request_child_offering".valid_until, ?), ?) AS ends_at
			FROM enrollment.request_child_offerings AS "request_child_offering"
			INNER JOIN enrollment.request_children AS "child"
				ON "child".id = "request_child_offering".request_child_id
			INNER JOIN enrollment.requests AS "request"
				ON "request".id = "child".request_id
			WHERE "request".phase_id = ?
				AND "request_child_offering".care_offering_id IN (?)
				AND ("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from < ?)
				AND ("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)
				AND "child".status NOT IN (?)
		), boundaries AS (
			SELECT care_offering_id, starts_at AS boundary FROM intervals
			UNION
			SELECT care_offering_id, ends_at AS boundary FROM intervals
		)
		SELECT
			boundaries.care_offering_id AS care_offering_id,
			COALESCE(MAX((
				SELECT COUNT(DISTINCT interval_row.request_child_id)
				FROM intervals AS interval_row
				WHERE interval_row.care_offering_id = boundaries.care_offering_id
					AND interval_row.starts_at <= boundaries.boundary
					AND interval_row.ends_at > boundaries.boundary
			)), 0) AS peak
		FROM boundaries
		GROUP BY boundaries.care_offering_id
	`, from, from, until, until, phaseID, bun.List(careOfferingIDs), until, from,
		bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn}),
	).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count peak active children for care offerings: %w", err)
	}
	for _, row := range rows {
		result[row.CareOfferingID] = row.Peak
	}
	return result, nil
}

// CountActiveGradeLevelsByCareOfferingIDs groups the bookings overlapping
// [from, until) by offering and by the child's target grade level, counting
// each child once per bucket. It backs the admin-side "how many existing
// bookings would this availability rule exclude" hint, so it deliberately
// mirrors the capacity gate's population: non-terminal request children
// (rejected/withdrawn excluded) whose validity interval overlaps the window.
//
// Children whose target_grade_level is NULL land in a bucket with a nil
// GradeLevel rather than being dropped: an availability rule never matches a
// missing grade, so those bookings conflict with any rule and hiding them
// would understate the hint.
func (r *RequestChildOfferingRepository) CountActiveGradeLevelsByCareOfferingIDs(
	ctx context.Context,
	phaseID int64,
	careOfferingIDs []int64,
	from, until timezone.Date,
) ([]*enrollment.CareOfferingGradeLevelCount, error) {
	result := make([]*enrollment.CareOfferingGradeLevelCount, 0)
	if len(careOfferingIDs) == 0 {
		return result, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("grade level range must not be empty")
	}
	var rows []struct {
		CareOfferingID int64  `bun:"care_offering_id"`
		GradeLevel     *int16 `bun:"grade_level"`
		Count          int    `bun:"child_count"`
	}
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT
			"request_child_offering".care_offering_id AS care_offering_id,
			"child".target_grade_level AS grade_level,
			COUNT(DISTINCT "request_child_offering".request_child_id) AS child_count
		FROM enrollment.request_child_offerings AS "request_child_offering"
		INNER JOIN enrollment.request_children AS "child"
			ON "child".id = "request_child_offering".request_child_id
		INNER JOIN enrollment.requests AS "request"
			ON "request".id = "child".request_id
		WHERE "request".phase_id = ?
			AND "request_child_offering".care_offering_id IN (?)
			AND ("request_child_offering".valid_from IS NULL OR "request_child_offering".valid_from < ?)
			AND ("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)
			AND "child".status NOT IN (?)
		GROUP BY 1, 2
		ORDER BY 1, 2
	`, phaseID, bun.List(careOfferingIDs), until, from,
		bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn}),
	).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count grade levels for care offerings: %w", err)
	}
	for _, row := range rows {
		result = append(result, &enrollment.CareOfferingGradeLevelCount{
			CareOfferingID: row.CareOfferingID,
			GradeLevel:     row.GradeLevel,
			Count:          row.Count,
		})
	}
	return result, nil
}

// ListApprovedChildrenByCareOfferingIDs returns the approved, still-relevant
// offering links for the given offerings with each child's resolved student
// and school class (#2137). Rows whose interval already ended before
// onOrAfter, children without a resolved student row, and alumni are
// excluded. Tenant isolation comes from RLS on the tenant transaction, like
// every other query in this repository.
//
// Links that only start in the future (a dated switch into the offering) are
// deliberately included — they are prospective enrollments the editor counts
// and the roster resync seeds. Callers that write dated rows MUST honour each
// link's ValidFrom/ValidUntil instead of assuming the link covers onOrAfter.
func (r *RequestChildOfferingRepository) ListApprovedChildrenByCareOfferingIDs(
	ctx context.Context,
	careOfferingIDs []int64,
	onOrAfter timezone.Date,
) ([]*enrollment.ApprovedOfferingChild, error) {
	result := make([]*enrollment.ApprovedOfferingChild, 0)
	if len(careOfferingIDs) == 0 {
		return result, nil
	}
	links := make([]*enrollment.RequestChildOffering, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&links).
		ModelTableExpr(requestChildOfferingTableExpr).
		Join(`INNER JOIN enrollment.request_children AS "child" ON "child".id = "request_child_offering".request_child_id`).
		Where(`"request_child_offering".care_offering_id IN (?)`, bun.List(careOfferingIDs)).
		Where(`"child".status = ?`, enrollment.ChildStatusApproved).
		Where(`("request_child_offering".valid_until IS NULL OR "request_child_offering".valid_until > ?)`, onOrAfter).
		OrderExpr(`"request_child_offering".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list approved children for care offerings: %w", err)
	}
	if len(links) == 0 {
		return result, nil
	}

	childIDs := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if !seen[link.RequestChildID] {
			seen[link.RequestChildID] = true
			childIDs = append(childIDs, link.RequestChildID)
		}
	}

	// Alumni are excluded by status; users.students has no soft-delete
	// column (graduation flips status to 'alumnus' instead).
	var studentRows []struct {
		ChildID     int64  `bun:"child_id"`
		StudentID   int64  `bun:"student_id"`
		SchoolClass string `bun:"school_class"`
	}
	err = base.GetDB(ctx, r.db).NewRaw(`
		SELECT
			"child".id AS child_id,
			"student".id AS student_id,
			"student".school_class
		FROM enrollment.request_children AS "child"
		INNER JOIN users.students AS "student"
			ON "student".id = COALESCE("child".created_student_id, "child".matched_student_id)
		WHERE "child".id IN (?)
		  AND "student".status <> 'alumnus'
	`, bun.List(childIDs)).Scan(ctx, &studentRows)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve students for approved offering children: %w", err)
	}

	type studentInfo struct {
		id          int64
		schoolClass string
	}
	students := make(map[int64]studentInfo, len(studentRows))
	for _, row := range studentRows {
		students[row.ChildID] = studentInfo{id: row.StudentID, schoolClass: row.SchoolClass}
	}
	for _, link := range links {
		info, ok := students[link.RequestChildID]
		if !ok {
			continue
		}
		result = append(result, &enrollment.ApprovedOfferingChild{
			Link:        link,
			StudentID:   info.id,
			SchoolClass: info.schoolClass,
		})
	}
	return result, nil
}
