package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableActiveStaffAbsences                   = "active.staff_absences"
	tableExprActiveStaffAbsencesAsStaffAbsence = `active.staff_absences AS "staff_absence"`
)

var effectiveStaffAbsenceStatuses = []string{
	active.AbsenceStatusReported,
	active.AbsenceStatusApproved,
}

var staffAbsencePriority = map[string]int{
	active.AbsenceTypeSick:     5,
	active.AbsenceTypeTraining: 4,
	active.AbsenceTypeVacation: 3,
	active.AbsenceTypeCompTime: 2,
	active.AbsenceTypeOther:    1,
}

// StaffAbsenceRepository implements active.StaffAbsenceRepository
type StaffAbsenceRepository struct {
	*base.Repository[*active.StaffAbsence]
	db *bun.DB
}

// NewStaffAbsenceRepository creates a new StaffAbsenceRepository
func NewStaffAbsenceRepository(db *bun.DB) active.StaffAbsenceRepository {
	repo := base.NewRepository[*active.StaffAbsence](db, tableActiveStaffAbsences, "StaffAbsence")
	repo.TenantScoped = true
	return &StaffAbsenceRepository{
		Repository: repo,
		db:         db,
	}
}

// List overrides base List to use QueryOptions
func (r *StaffAbsenceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffAbsence, error) {
	return r.ListWithOptions(ctx, options)
}

// LockStaffAbsenceWrites serializes overlap-sensitive absence writes for one
// tenant/staff pair. It takes the shared balance lock first because effective
// absence mutations also change the Stundenkonto. The transaction-scoped
// locks stay held through any sick plan cascade or reversal.
func (r *StaffAbsenceRepository) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if err := lockStaffBalanceWrites(ctx, r.db, staffID); err != nil {
		return err
	}
	key := fmt.Sprintf("staff-absence:%d:%d", tenantID, staffID)
	if err := base.AcquireXactLock(ctx, r.db, key); err != nil {
		return fmt.Errorf("lock staff absence writes: %w", err)
	}
	return nil
}

// GetByStaffAndDateRange returns absences for a staff member overlapping the given date range
func (r *StaffAbsenceRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(`"staff_absence".date_start <= ?`, to).
		Where(`"staff_absence".date_end >= ?`, from).
		OrderExpr(`"staff_absence".date_start ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get absences by staff and date range",
			Err: base.TranslateNotFound(err),
		}
	}

	return absences, nil
}

// GetByStaffAndDate returns an absence for a staff member on a specific date, or nil
func (r *StaffAbsenceRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*active.StaffAbsence, error) {
	absence := new(active.StaffAbsence)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(absence).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(`"staff_absence".date_start <= ?`, date).
		Where(`"staff_absence".date_end >= ?`, date).
		Where(`"staff_absence".status IN (?)`, bun.List(effectiveStaffAbsenceStatuses)).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	err := query.Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "get absence by staff and date",
			Err: base.TranslateNotFound(err),
		}
	}

	return absence, nil
}

// GetAbsenceMapForDate returns a map of staff IDs to their absence type for the given date.
// Priority order when multiple absences exist:
// sick > training > vacation > comp_time > other.
func (r *StaffAbsenceRepository) GetAbsenceMapForDate(ctx context.Context, date timezone.Date) (map[int64]string, error) {
	absences, err := r.effectiveAbsencesForDate(ctx, date)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]string, len(absences))
	for _, a := range absences {
		existing, exists := result[a.StaffID]
		if !exists || staffAbsencePriority[a.AbsenceType] > staffAbsencePriority[existing] {
			result[a.StaffID] = a.AbsenceType
		}
	}

	return result, nil
}

// GetAbsenceTypeIDMapForDate returns staff ID -> the school-defined
// Abwesenheitsart of the absence that wins GetAbsenceMapForDate's priority on
// that date (#2403). Only staff whose winning absence carries one appear.
//
// Kept additive rather than folded into GetAbsenceMapForDate: that map's value
// is compared against the canonical type constants all over the codebase, and
// widening it would have every one of those call sites decide again what to do
// with a name. Both maps use the same ordered query, including ID as the
// stable tie-breaker, so they select the same absence under equal priority.
func (r *StaffAbsenceRepository) GetAbsenceTypeIDMapForDate(ctx context.Context, date timezone.Date) (map[int64]int64, error) {
	absences, err := r.effectiveAbsencesForDate(ctx, date)
	if err != nil {
		return nil, err
	}

	winner := make(map[int64]*active.StaffAbsence, len(absences))
	for _, a := range absences {
		existing, exists := winner[a.StaffID]
		if !exists || staffAbsencePriority[a.AbsenceType] > staffAbsencePriority[existing.AbsenceType] {
			winner[a.StaffID] = a
		}
	}

	result := make(map[int64]int64, len(winner))
	for staffID, a := range winner {
		if a.AbsenceTypeID != nil {
			result[staffID] = *a.AbsenceTypeID
		}
	}
	return result, nil
}

// effectiveAbsencesForDate returns candidates in canonical priority order.
// ID resolves equal-priority overlaps deterministically. Both absence maps
// must use this exact ordering because callers combine their values to render
// a canonical status and its optional school-defined label.
func (r *StaffAbsenceRepository) effectiveAbsencesForDate(ctx context.Context, date timezone.Date) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".date_start <= ?`, date).
		Where(`"staff_absence".date_end >= ?`, date).
		Where(`"staff_absence".status IN (?)`, bun.List(effectiveStaffAbsenceStatuses)).
		OrderExpr(`"staff_absence".staff_id ASC`).
		OrderExpr(`CASE "staff_absence".absence_type WHEN 'sick' THEN 5 WHEN 'training' THEN 4 WHEN 'vacation' THEN 3 WHEN 'comp_time' THEN 2 WHEN 'other' THEN 1 ELSE 0 END DESC`).
		OrderExpr(`"staff_absence".id ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_absence")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get effective absences for date",
			Err: base.TranslateNotFound(err),
		}
	}
	return absences, nil
}

// ListByStatuses returns all absences whose status is in the given set,
// ordered by requested_at ASC (oldest request first).
func (r *StaffAbsenceRepository) ListByStatuses(ctx context.Context, statuses []string) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".status IN (?)`, bun.List(statuses)).
		OrderExpr(`"staff_absence".requested_at ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list absences by statuses", Err: base.TranslateNotFound(err)}
	}
	return absences, nil
}

// ListRequests returns absence requests with the subject and decider names the
// Anfragen module shows (#2433). Both name joins are LEFT joins so a request
// stays visible after its staff or person row is gone; the name is empty then.
func (r *StaffAbsenceRepository) ListRequests(ctx context.Context, filter active.AbsenceRequestFilter) ([]*active.AbsenceRequestRow, error) {
	if len(filter.Statuses) == 0 {
		return nil, errors.New("at least one status is required")
	}
	var rows []*active.AbsenceRequestRow
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		ColumnExpr(`"staff_absence".*`).
		ColumnExpr(`COALESCE("subject_person".first_name || ' ' || "subject_person".last_name, '') AS staff_name`).
		ColumnExpr(`COALESCE("decider_person".first_name || ' ' || "decider_person".last_name, '') AS decided_by_name`).
		Join(`LEFT JOIN users.staff AS "subject_staff" ON "subject_staff".id = "staff_absence".staff_id`).
		Join(`LEFT JOIN users.persons AS "subject_person" ON "subject_person".id = "subject_staff".person_id`).
		Join(`LEFT JOIN users.staff AS "decider_staff" ON "decider_staff".id = "staff_absence".approved_by`).
		Join(`LEFT JOIN users.persons AS "decider_person" ON "decider_person".id = "decider_staff".person_id`).
		Where(`"staff_absence".status IN (?)`, bun.List(filter.Statuses))

	if len(filter.Types) > 0 {
		query = query.Where(`"staff_absence".absence_type IN (?)`, bun.List(filter.Types))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		// Escape the LIKE metacharacters so a typed % or _ stays a literal
		// character of the name instead of silently matching everything.
		escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search)
		pattern := "%" + escaped + "%"
		query = query.Where(
			`COALESCE("subject_person".first_name || ' ' || "subject_person".last_name, '') ILIKE ?`,
			pattern,
		)
	}
	if filter.Decided {
		query = query.OrderExpr(`COALESCE("staff_absence".approved_at, "staff_absence".updated_at) DESC, "staff_absence".id DESC`)
	} else {
		query = query.OrderExpr(`"staff_absence".requested_at ASC, "staff_absence".id ASC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list absence requests", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// ListNonHistoricalByStaffID returns the rows that staff offboarding removes,
// preserving their full values for deletion tombstones.
func (r *StaffAbsenceRepository) ListNonHistoricalByStaffID(ctx context.Context, staffID int64, from timezone.Date) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(
			`("staff_absence".status IN (?, ?) OR "staff_absence".date_end >= ?)`,
			active.AbsenceStatusRequested,
			active.AbsenceStatusQuestion,
			from,
		).
		OrderExpr(`"staff_absence".id ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list non-historical absences by staff id",
			Err: base.TranslateNotFound(err),
		}
	}
	return absences, nil
}

// DeleteNonHistoricalByStaffID hard-deletes absences that are still pending
// ('requested' or 'question') or not yet over (date_end >= from). Past decided
// absences stay as history. Used by staff offboarding so offboarded staff no
// longer appear in absence request lists and date maps. Returns the number of
// deleted rows.
func (r *StaffAbsenceRepository) DeleteNonHistoricalByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*active.StaffAbsence)(nil)).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(
			`("staff_absence".status IN (?, ?) OR "staff_absence".date_end >= ?)`,
			active.AbsenceStatusRequested,
			active.AbsenceStatusQuestion,
			from,
		)

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete non-historical absences by staff id",
			Err: base.TranslateNotFound(err),
		}
	}

	return result.RowsAffected()
}

// GetByStaffIDsAndDateRange is GetByStaffAndDateRange for many staff members in
// one round trip, keyed by staff ID. A batched IN-lookup the generic filter API
// cannot express as a single query. It keeps the OVERLAP predicate of its
// single-staff twin: the month math needs absences that start before `from`.
func (r *StaffAbsenceRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*active.StaffAbsence, error) {
	result := make(map[int64][]*active.StaffAbsence, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var absences []*active.StaffAbsence
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"staff_absence".date_start <= ?`, to).
		Where(`"staff_absence".date_end >= ?`, from).
		OrderExpr(`"staff_absence".staff_id ASC, "staff_absence".date_start ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get absences by staff IDs and date range",
			Err: base.TranslateNotFound(err),
		}
	}
	for _, absence := range absences {
		result[absence.StaffID] = append(result[absence.StaffID], absence)
	}
	return result, nil
}
