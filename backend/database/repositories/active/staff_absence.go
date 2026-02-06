package active

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	tableActiveStaffAbsences                   = "active.staff_absences"
	tableExprActiveStaffAbsencesAsStaffAbsence = `active.staff_absences AS "staff_absence"`
)

// StaffAbsenceRepository implements active.StaffAbsenceRepository
type StaffAbsenceRepository struct {
	*base.Repository[*active.StaffAbsence]
	db *bun.DB
}

// NewStaffAbsenceRepository creates a new StaffAbsenceRepository
func NewStaffAbsenceRepository(db *bun.DB) active.StaffAbsenceRepository {
	return &StaffAbsenceRepository{
		Repository: base.NewRepository[*active.StaffAbsence](db, tableActiveStaffAbsences, "StaffAbsence"),
		db:         db,
	}
}

// Create overrides base Create to handle validation
func (r *StaffAbsenceRepository) Create(ctx context.Context, absence *active.StaffAbsence) error {
	if absence == nil {
		return fmt.Errorf("staff absence cannot be nil")
	}

	if err := absence.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, absence)
}

// List overrides base List to use QueryOptions
func (r *StaffAbsenceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	query := r.db.NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence)

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

	return absences, nil
}

// GetByStaffAndDateRange returns absences for a staff member overlapping the given date range
func (r *StaffAbsenceRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to time.Time) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	err := r.db.NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(`"staff_absence".date_start <= ?`, to).
		Where(`"staff_absence".date_end >= ?`, from).
		OrderExpr(`"staff_absence".date_start ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get absences by staff and date range",
			Err: err,
		}
	}

	return absences, nil
}

// GetByStaffAndDate returns an absence for a staff member on a specific date, or nil
func (r *StaffAbsenceRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*active.StaffAbsence, error) {
	absence := new(active.StaffAbsence)
	err := r.db.NewSelect().
		Model(absence).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(`"staff_absence".date_start <= ?`, date).
		Where(`"staff_absence".date_end >= ?`, date).
		Limit(1).
		Scan(ctx)

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
// Priority order when multiple absences exist: sick > training > vacation > other
func (r *StaffAbsenceRepository) GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error) {
	var absences []*active.StaffAbsence
	err := r.db.NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".date_start <= CURRENT_DATE`).
		Where(`"staff_absence".date_end >= CURRENT_DATE`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today absence map",
			Err: err,
		}
	}

	// Priority: sick > training > vacation > other
	priority := map[string]int{
		active.AbsenceTypeSick:     4,
		active.AbsenceTypeTraining: 3,
		active.AbsenceTypeVacation: 2,
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

// GetByDateRange returns all absences overlapping the given date range
func (r *StaffAbsenceRepository) GetByDateRange(ctx context.Context, from, to time.Time) ([]*active.StaffAbsence, error) {
	var absences []*active.StaffAbsence
	err := r.db.NewSelect().
		Model(&absences).
		ModelTableExpr(tableExprActiveStaffAbsencesAsStaffAbsence).
		Where(`"staff_absence".date_start <= ?`, to).
		Where(`"staff_absence".date_end >= ?`, from).
		OrderExpr(`"staff_absence".date_start ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get absences by date range",
			Err: err,
		}
	}

	return absences, nil
}
