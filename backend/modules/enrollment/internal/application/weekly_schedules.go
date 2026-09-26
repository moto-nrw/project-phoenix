package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// scheduleWeekdays maps the form's weekday keys onto the ISO weekday numbers
// of the weekly schedules.
var scheduleWeekdays = map[string]int{
	"mon": 1,
	"tue": 2,
	"wed": 3,
	"thu": 4,
	"fri": 5,
}

// dispatchWeekdaySchedule inserts one pickup or arrival schedule row per
// non-empty weekday entry. isPickup=true targets the pickup schedules,
// false the arrival schedules; an unbound schedule port is a no-op.
func (d *Decisions) dispatchWeekdaySchedule(ctx context.Context, raw any, studentID int64, reviewedBy int64, isPickup bool) error {
	if (isPickup && d.deps.PickupSchedules == nil) || (!isPickup && d.deps.ArrivalSchedules == nil) {
		return nil
	}
	var sched enrollment.WeekdaySchedule
	if err := decodeStructured(raw, &sched); err != nil {
		return fmt.Errorf("decode weekday_schedule: %w", err)
	}
	if err := sched.Validate(); err != nil {
		return err
	}
	createdBy, err := d.resolveReviewerStaffID(ctx, reviewedBy)
	if err != nil {
		return err
	}
	for day, hhmm := range sched {
		hhmm = strings.TrimSpace(hhmm)
		if hhmm == "" {
			continue
		}
		if err := d.writeWeekdaySchedule(ctx, studentID, day, hhmm, createdBy, isPickup); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decisions) writeWeekdaySchedule(ctx context.Context, studentID int64, day, hhmm string, createdBy int64, isPickup bool) error {
	if !isPickup {
		if err := d.deps.ArrivalSchedules.CreateArrivalSchedule(ctx, studentID, scheduleWeekdays[day], createdBy); err != nil {
			return fmt.Errorf("create arrival %s: %w", day, err)
		}
		return nil
	}
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return fmt.Errorf("parse %s time %q: %w", day, hhmm, err)
	}
	if err := d.deps.PickupSchedules.UpsertPickupSchedule(ctx, studentID, scheduleWeekdays[day], calendar.NormalizeWallClock(t), createdBy); err != nil {
		return fmt.Errorf("upsert pickup %s: %w", day, err)
	}
	return nil
}

// resolveReviewerStaffID returns the staff id of the reviewing account, the
// author of the schedule rows the approval writes.
func (d *Decisions) resolveReviewerStaffID(ctx context.Context, reviewerAccountID int64) (int64, error) {
	if reviewerAccountID <= 0 {
		return 0, fmt.Errorf("reviewer account id is required")
	}
	people := d.deps.People
	if people.Persons == nil || people.Staff == nil {
		return 0, fmt.Errorf("reviewer staff lookup is unavailable")
	}
	person, err := people.Persons.PersonByAccount(ctx, reviewerAccountID)
	if err != nil {
		return 0, fmt.Errorf("find reviewer person: %w", err)
	}
	if person == nil {
		return 0, fmt.Errorf("reviewer account %d has no linked person", reviewerAccountID)
	}
	staffID, err := people.Staff.StaffIDByPerson(ctx, person.ID)
	if err != nil {
		if d.deps.Runtime.NotFound(err) {
			return 0, fmt.Errorf("reviewer account %d has no linked staff", reviewerAccountID)
		}
		return 0, fmt.Errorf("find reviewer staff: %w", err)
	}
	if staffID <= 0 {
		return 0, fmt.Errorf("reviewer account %d has no linked staff", reviewerAccountID)
	}
	return staffID, nil
}

// An approved weekly Gehzeit plan changes the same pickup baseline a staff
// weekly edit does. These hooks take the student lock first, compare the
// plan before and after, and re-derive the coupled auto excusals in the
// caller's transaction. They are nil-safe for focused tests.

func (d *Decisions) lockPickupStudents(ctx context.Context, studentIDs []int64) error {
	if d.deps.Pickups.LockStudents == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := d.deps.Pickups.LockStudents(ctx, studentIDs); err != nil {
		return fmt.Errorf("lock students for pickup reset: %w", err)
	}
	return nil
}

func (d *Decisions) resyncPickupAutoExcusals(ctx context.Context, studentIDs []int64) error {
	if d.deps.Pickups.ResyncExcusal == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := d.deps.Pickups.ResyncExcusal(ctx, studentIDs); err != nil {
		return fmt.Errorf("resync pickup auto excusals: %w", err)
	}
	return nil
}

func (d *Decisions) snapshotPickupWeekdayChanges(ctx context.Context, studentID int64, date calendar.Date) (map[int]string, error) {
	if d.deps.Pickups.Snapshot == nil {
		return nil, nil
	}
	before, err := d.deps.Pickups.Snapshot(ctx, studentID, date)
	if err != nil {
		return nil, fmt.Errorf("snapshot pickup weekday changes: %w", err)
	}
	return before, nil
}

func (d *Decisions) recordPickupWeekdayChanges(ctx context.Context, studentID int64, date calendar.Date, before map[int]string) error {
	if d.deps.Pickups.RecordChanges == nil || before == nil {
		return nil
	}
	if err := d.deps.Pickups.RecordChanges(ctx, studentID, date, before); err != nil {
		return fmt.Errorf("record pickup weekday changes: %w", err)
	}
	return nil
}
