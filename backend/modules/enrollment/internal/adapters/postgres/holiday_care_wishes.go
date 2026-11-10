package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The holiday care wish (#4703). The approved answer is read in the tenant
// transaction of the child's school, the office's decision written in the
// Träger transaction. Both name their tenant once more in the filter, below
// RLS.

// approvedHolidayCareWishQuery reads the answer of the newest approved
// enrollment of the student, new or re-enrolled, in a school-year phase of
// the school whose service window holds the day.
const approvedHolidayCareWishQuery = `SELECT child.holiday_care_wish
	FROM enrollment.request_children AS child
	JOIN enrollment.requests AS request ON request.tenant_id = child.tenant_id AND request.id = child.request_id
	JOIN enrollment.phases AS phase ON phase.tenant_id = request.tenant_id AND phase.id = request.phase_id
	WHERE child.tenant_id = ? AND child.status = ?
		AND (child.created_student_id = ? OR child.matched_student_id = ?)
		AND phase.kind = ? AND phase.service_start_date <= ?::date AND phase.service_end_date >= ?::date
		AND child.holiday_care_wish IS NOT NULL
	ORDER BY phase.service_start_date DESC, child.reviewed_at DESC NULLS LAST, child.id DESC
	LIMIT 1`

// ApprovedHolidayCareWish is the holiday care wish of the student's newest
// approved enrollment for the school year that holds on; nil when there is
// none or its form did not ask.
func (r *Store) ApprovedHolidayCareWish(ctx context.Context, studentID int64, on calendar.Date) (*bool, error) {
	if studentID <= 0 || on.IsZero() {
		return nil, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var wish bool
	err = db.NewRaw(approvedHolidayCareWishQuery, tenantID, enrollment.ChildStatusApproved, studentID, studentID,
		enrollment.PhaseKindSchoolYear, on.String(), on.String()).Scan(ctx, &wish)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday care wish: %w", err)
	}
	return &wish, nil
}

const setHolidayChildWishQuery = `UPDATE enrollment.holiday_registration_children
	SET holiday_wish = ?, holiday_wish_source = ?, updated_at = statement_timestamp()
	WHERE organization_id = ? AND holiday_registration_id = ? AND id = ? AND home_school_id IS NOT NULL`

// SetHolidayChildWish writes the wish of a child of a school of the Träger
// and reports whether the registration has such a child.
func (s *HolidayRegistrationStore) SetHolidayChildWish(ctx context.Context, organizationID, registrationID, childID int64, wish, source string) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	result, err := db.NewRaw(setHolidayChildWishQuery, wish, source, organizationID, registrationID, childID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: write holiday wish: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}
