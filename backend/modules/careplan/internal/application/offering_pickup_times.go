package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The write-side effects of booking-derived pickup times. The pickup
// baseline itself is projected at read time; these operations only remove
// manual overrides and refresh the materialized consumers that still depend
// on a pickup baseline.

// ReconcileOfferingPickupForStudents runs after every booking write of the
// students (adjustment, change request, approval, offering edit) in the same
// transaction. Besides the pickup effects it announces the booking change to
// the staff views whose hints read the bookings (#5498).
func (m *BookingMaterialization) ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error {
	studentIDs = sortedPositiveIDs(studentIDs)
	if len(studentIDs) == 0 {
		return nil
	}
	if m.deps.ResyncPickupAutoExcusals != nil {
		if err := m.deps.ResyncPickupAutoExcusals(ctx, studentIDs); err != nil {
			return fmt.Errorf("resync pickup auto excusals: %w", err)
		}
	}
	if m.deps.AnnouncePickupChange != nil {
		m.deps.AnnouncePickupChange(ctx, studentIDs)
	}
	if m.deps.PlanningStaffing != nil {
		m.deps.PlanningStaffing.AnnounceBookingChange(ctx)
	}
	return nil
}

func (m *BookingMaterialization) ReconcileOfferingPickupForOffering(ctx context.Context, offeringID int64) error {
	if offeringID <= 0 {
		return fmt.Errorf("%w: offering id is required", careplan.ErrCareOfferingConfigInvalid)
	}
	studentIDs, err := m.offeringStudentIDs(ctx, offeringID)
	if err != nil {
		return err
	}
	return m.ReconcileOfferingPickupForStudents(ctx, studentIDs)
}

// offeringStudentIDs lists the students booked into the offering today or
// later.
func (m *BookingMaterialization) offeringStudentIDs(ctx context.Context, offeringID int64) ([]int64, error) {
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, []int64{offeringID}, calendar.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("list approved offering children: %w", err)
	}
	studentIDs := make([]int64, 0, len(children))
	for _, child := range children {
		studentIDs = append(studentIDs, child.StudentID)
	}
	return studentIDs, nil
}

// ResetStudentPickupDayToOffering removes the staff pickup row of the date's
// weekday so the booked pickup time applies. The row has no validity period:
// removing it applies to every remaining day of the weekday, also to those
// before the date. A future date is refused while such an earlier day still
// needs the stored time (#5643).
func (m *BookingMaterialization) ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, date calendar.Date) error {
	first, err := m.pickupResetFirstDay(date)
	if err != nil {
		return err
	}
	weekday := int(date.Weekday())
	// Match care end: class -> recurrence before the student and pickup rows.
	if err := m.LockOfferingDerivedWrites(ctx); err != nil {
		return err
	}
	if err := m.lockPickupStudents(ctx, []int64{studentID}); err != nil {
		return err
	}
	projection, err := m.deps.Pickup.OfferingPickupsBetween(ctx, studentID, min(first, date), date)
	if err != nil {
		return fmt.Errorf("load offering pickup for reset: %w", err)
	}
	if projection.OfferingForDate(studentID, date) == nil {
		return careplan.ErrPickupResetNoOffering
	}
	row, err := m.pickupWeekdayRow(ctx, studentID, weekday)
	if err != nil {
		return err
	}
	if row == nil {
		return m.resyncPickupAutoExcusals(ctx, []int64{studentID})
	}
	// A legacy care_offering row is no override: the projection ignores it,
	// so removing it changes no earlier day.
	if row.Manual {
		if until, needed := lastDayNeedingStoredPickup(projection, studentID, *row, first, date); needed {
			return &careplan.PickupResetStillNeededError{NeededUntil: until, PossibleFrom: until.AddDays(1)}
		}
	}
	if err := m.deps.Pickup.DeleteWeekdayRow(ctx, row.ID); err != nil {
		return fmt.Errorf("delete pickup schedule override: %w", err)
	}
	if m.deps.ClearPickupWeekdayExtension != nil {
		if err := m.deps.ClearPickupWeekdayExtension(ctx, studentID, weekday); err != nil {
			return fmt.Errorf("clear pickup weekday extension: %w", err)
		}
	}
	return m.resyncPickupAutoExcusals(ctx, []int64{studentID})
}

// pickupResetFirstDay validates the reset date and returns the first day of
// its weekday the reset reaches: the weekday's next date from today. The
// reset projects every day from there to the date, so a date beyond the
// horizon is refused before any lock is taken.
func (m *BookingMaterialization) pickupResetFirstDay(date calendar.Date) (calendar.Date, error) {
	weekday := int(date.Weekday())
	if weekday < 1 || weekday > 5 {
		return "", errors.New("pickup reset date must be Monday through Friday")
	}
	today := m.todayDate()
	if date.After(today.AddDays(pickupResetHorizonDays)) {
		return "", careplan.ErrPickupResetBeyondHorizon
	}
	return nextWeekdayFrom(today, weekday), nil
}

// pickupResetHorizonDays is how far ahead a reset date may lie: a year covers
// every booking change staff can plan in the care plan editor.
const pickupResetHorizonDays = 366

// pickupWeekdayRow is the student's stored row of the weekday; nil when there
// is none.
func (m *BookingMaterialization) pickupWeekdayRow(ctx context.Context, studentID int64, weekday int) (*ports.PickupWeekdayRow, error) {
	rows, err := m.deps.Pickup.WeekdayRows(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("load pickup schedules: %w", err)
	}
	for i := range rows {
		if rows[i].Weekday == weekday {
			return &rows[i], nil
		}
	}
	return nil, nil
}

// lastDayNeedingStoredPickup is the last day from day up to (excluding)
// until on which the bookings do not set the stored time, so that removing
// the row would change that day; false when there is none.
func lastDayNeedingStoredPickup(
	projection *careplan.PickupBaselineProjection, studentID int64, row ports.PickupWeekdayRow, day, until calendar.Date,
) (calendar.Date, bool) {
	var last calendar.Date
	for ; day.Before(until); day = day.AddDays(7) {
		if offeringPickupClock(projection, studentID, day) != row.PickupTime {
			last = day
		}
	}
	return last, !last.IsZero()
}

// lockPickupStudents takes the students' care locks BEFORE a weekly pickup
// row changes. Staff weekly editors lock the student first and schedule rows
// second; the reset must acquire in the same order or a concurrent staff edit
// can deadlock against it (#2360 review).
func (m *BookingMaterialization) lockPickupStudents(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if err := m.deps.Students.LockStudents(ctx, studentIDs); err != nil {
		return fmt.Errorf("lock students for pickup reset: %w", err)
	}
	return nil
}

func (m *BookingMaterialization) resyncPickupAutoExcusals(ctx context.Context, studentIDs []int64) error {
	if m.deps.ResyncPickupAutoExcusals == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := m.deps.ResyncPickupAutoExcusals(ctx, studentIDs); err != nil {
		return fmt.Errorf("resync pickup auto excusals: %w", err)
	}
	return nil
}

func sortedPositiveIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}
