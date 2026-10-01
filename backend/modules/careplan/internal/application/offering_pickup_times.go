package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The write-side effects of booking-derived pickup times. The pickup
// baseline itself is projected at read time; these operations only remove
// manual overrides and refresh the materialized consumers that still depend
// on a pickup baseline.

// ReconcileOfferingPickupForStudents refreshes the remaining derived effects.
// It deliberately writes no schedule.student_pickup_schedules rows: regular
// offering pickup times are a date-aware projection of booking validity.
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
	return nil
}

// ReconcileOfferingPickupForOffering refreshes every current or future child
// affected by an offering edit. The catalog calls it after an update.
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

// ResetStudentPickupDayToOffering removes the stored row for one weekday. A
// staff row is the manual override; an old care_offering row is legacy
// materialization. The row is removed only when the read-time projection has
// an offering time for the requested date.
func (m *BookingMaterialization) ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, date calendar.Date) error {
	weekday := int(date.Weekday())
	if weekday < 1 || weekday > 5 {
		return errors.New("pickup reset date must be Monday through Friday")
	}
	if err := m.lockPickupStudents(ctx, []int64{studentID}); err != nil {
		return err
	}
	if err := m.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	offering, err := m.deps.Pickup.OfferingPickupForDate(ctx, studentID, date)
	if err != nil {
		return fmt.Errorf("load offering pickup for reset: %w", err)
	}
	if offering == nil {
		return careplan.ErrPickupResetNoOffering
	}
	removed, err := m.deletePickupWeekdayRow(ctx, studentID, weekday)
	if err != nil {
		return err
	}
	if removed && m.deps.ClearPickupWeekdayExtension != nil {
		if err := m.deps.ClearPickupWeekdayExtension(ctx, studentID, weekday); err != nil {
			return fmt.Errorf("clear pickup weekday extension: %w", err)
		}
	}
	return m.resyncPickupAutoExcusals(ctx, []int64{studentID})
}

// deletePickupWeekdayRow removes the student's stored row of the weekday and
// reports whether there was one.
func (m *BookingMaterialization) deletePickupWeekdayRow(ctx context.Context, studentID int64, weekday int) (bool, error) {
	rows, err := m.deps.Pickup.WeekdayRows(ctx, studentID)
	if err != nil {
		return false, fmt.Errorf("load pickup schedules: %w", err)
	}
	for _, row := range rows {
		if row.Weekday != weekday {
			continue
		}
		if err := m.deps.Pickup.DeleteWeekdayRow(ctx, row.ID); err != nil {
			return false, fmt.Errorf("delete pickup schedule override: %w", err)
		}
		return true, nil
	}
	return false, nil
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
