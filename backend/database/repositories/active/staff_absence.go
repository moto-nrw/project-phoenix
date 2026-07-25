package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
			Err: err,
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
			Err: err,
		}
	}

	return absence, nil
}

// GetTodayAbsenceMap returns a map of staff IDs to their absence type for today.
// Priority order when multiple absences exist:
// sick > training > vacation > comp_time > other.
func (r *StaffAbsenceRepository) GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error) {
	var absences []*active.StaffAbsence
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".date_start <= CURRENT_DATE`).
		Where(`"staff_absence".date_end >= CURRENT_DATE`).
		Where(`"staff_absence".status IN (?)`, bun.List(effectiveStaffAbsenceStatuses))

	query = base.WithTenantFilter(ctx, query, "staff_absence")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today absence map",
			Err: err,
		}
	}

	// Priority: sick > training > vacation > comp_time > other.
	priority := map[string]int{
		active.AbsenceTypeSick:     5,
		active.AbsenceTypeTraining: 4,
		active.AbsenceTypeVacation: 3,
		active.AbsenceTypeCompTime: 2,
		active.AbsenceTypeOther:    1,
	}

	result := make(map[int64]string, len(absences))
	for _, a := range absences {
		existing, exists := result[a.StaffID]
		if !exists || priority[a.AbsenceType] > priority[existing] {
			result[a.StaffID] = a.AbsenceType
		}
	}

	return result, nil
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
		return nil, &modelBase.DatabaseError{Op: "list absences by statuses", Err: err}
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
			Err: err,
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
			Err: err,
		}
	}
	for _, absence := range absences {
		result[absence.StaffID] = append(result[absence.StaffID], absence)
	}
	return result, nil
}
