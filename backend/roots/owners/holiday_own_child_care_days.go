package owners

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewHolidayOwnChildCareDays binds the booked days of a child of the hosting
// school itself (#6678, ADR 0120 addendum) to Care Plan: a dated arrival and
// pickup exception for each booked day with the times of the booked time
// model, so the check-in expects the child on those days.
//
// Such a child keeps every plan the team or its parents wrote: Enrollment
// owns only the exceptions whose reason it wrote
// (enrollment.HolidayOwnChildCareReason with the booked times). A day that
// carries another exception is left alone, and so is one of its own days the
// team edited: the reason still names the booked times, so the binding sees
// that the booking did not change and keeps the team's time.
func NewHolidayOwnChildCareDays(care HolidayGuestCarePlan, staff HolidayGuestStaff) enrollmentOwner.HolidayOwnChildCareDays {
	if care == nil || staff == nil {
		return nil
	}
	return holidayOwnChildCareDays{care: care, staff: staff, logger: slog.Default().With("service", "holiday-own-children")}
}

type holidayOwnChildCareDays struct {
	care   HolidayGuestCarePlan
	staff  HolidayGuestStaff
	logger *slog.Logger
}

type holidayOwnChildDayKey struct {
	studentID int64
	day       calendar.Date
}

// holidayOwnChildReason is the reason of a planned day: the marker and the
// booked times, which tell a changed booking from a time the team edited.
func holidayOwnChildReason(day enrollmentOwner.HolidayOwnChildDay) string {
	return fmt.Sprintf("%s (%s bis %s Uhr)", enrollmentOwner.HolidayOwnChildCareReason, day.Arrival, day.Pickup)
}

func isHolidayOwnChildReason(reason *string) bool {
	return reason != nil && strings.HasPrefix(*reason, enrollmentOwner.HolidayOwnChildCareReason)
}

func (c holidayOwnChildCareDays) SyncHolidayOwnChildDays(ctx context.Context, from, today calendar.Date, wanted []enrollmentOwner.HolidayOwnChildDay) error {
	byKey := make(map[holidayOwnChildDayKey]enrollmentOwner.HolidayOwnChildDay, len(wanted))
	for _, day := range wanted {
		key := holidayOwnChildDayKey{day.StudentID, day.Day}
		if _, ok := byKey[key]; !ok && !day.Day.Before(from) {
			byKey[key] = day
		}
	}
	filter := careplan.StudentScheduleFilter{From: careplan.Date(from)}
	arrivals, err := c.care.ListArrivalExceptions(ctx, filter)
	if err != nil {
		return err
	}
	pickups, err := c.care.ListPickupExceptions(ctx, filter)
	if err != nil {
		return err
	}
	arrivalDone, err := c.settleArrivals(ctx, arrivals, byKey)
	if err != nil {
		return err
	}
	pickupDone, err := c.settlePickups(ctx, pickups, byKey)
	if err != nil {
		return err
	}
	return c.planMissing(ctx, today, wanted, byKey, arrivalDone, pickupDone)
}

// settleArrivals removes the planned arrivals whose booking went or changed
// and returns the days that need no arrival written: they carry a plan of
// the team or the parents, or the planned one still fits.
func (c holidayOwnChildCareDays) settleArrivals(ctx context.Context, existing []careplan.ArrivalException, wanted map[holidayOwnChildDayKey]enrollmentOwner.HolidayOwnChildDay) (map[holidayOwnChildDayKey]bool, error) {
	done := map[holidayOwnChildDayKey]bool{}
	for _, exception := range existing {
		key := holidayOwnChildDayKey{exception.StudentID, calendar.Date(exception.ExceptionDate)}
		day, booked := wanted[key]
		if !isHolidayOwnChildReason(exception.Reason) || booked && *exception.Reason == holidayOwnChildReason(day) {
			done[key] = true
			continue
		}
		if err := c.care.DeleteArrivalException(ctx, exception.ID); err != nil {
			return nil, err
		}
	}
	return done, nil
}

// settlePickups does the same for the planned pickups. A planned pickup the
// team added an excused time to stays: it is the team's entry now.
func (c holidayOwnChildCareDays) settlePickups(ctx context.Context, existing []careplan.PickupException, wanted map[holidayOwnChildDayKey]enrollmentOwner.HolidayOwnChildDay) (map[holidayOwnChildDayKey]bool, error) {
	done := map[holidayOwnChildDayKey]bool{}
	for _, exception := range existing {
		key := holidayOwnChildDayKey{exception.StudentID, calendar.Date(exception.ExceptionDate)}
		day, booked := wanted[key]
		if !isHolidayOwnChildReason(exception.Reason) || exception.ExcusedFrom != nil || booked && *exception.Reason == holidayOwnChildReason(day) {
			done[key] = true
			continue
		}
		if err := c.care.DeletePickupException(ctx, exception.ID); err != nil {
			return nil, err
		}
	}
	return done, nil
}

// planMissing writes the booked days from today on that carry no plan yet,
// under the staff member of the account that picked the week's holiday
// group. A week without a group, or an account without a staff record,
// plans nothing new; the next group choice does.
func (c holidayOwnChildCareDays) planMissing(ctx context.Context, today calendar.Date, wanted []enrollmentOwner.HolidayOwnChildDay, byKey map[holidayOwnChildDayKey]enrollmentOwner.HolidayOwnChildDay, arrivalDone, pickupDone map[holidayOwnChildDayKey]bool) error {
	authors := map[int64]int64{}
	for _, candidate := range wanted {
		key := holidayOwnChildDayKey{candidate.StudentID, candidate.Day}
		day, ok := byKey[key]
		if !ok || day.Day.Before(today) || day.ActorAccountID <= 0 || arrivalDone[key] && pickupDone[key] {
			continue
		}
		delete(byKey, key)
		staffID, known := authors[day.ActorAccountID]
		if !known {
			resolved, err := c.staff.ResolveStaffIDByAccountID(ctx, day.ActorAccountID)
			if err != nil {
				c.logger.Warn("own holiday children left unplanned: the account that picked the group has no staff record",
					"account_id", day.ActorAccountID,
					"error", err,
				)
			}
			staffID = resolved
			authors[day.ActorAccountID] = staffID
		}
		if staffID <= 0 {
			continue
		}
		if err := c.planDay(ctx, day, staffID, !arrivalDone[key], !pickupDone[key]); err != nil {
			return err
		}
	}
	return nil
}

func (c holidayOwnChildCareDays) planDay(ctx context.Context, day enrollmentOwner.HolidayOwnChildDay, staffID int64, arrival, pickup bool) error {
	arrivalAt, err := holidayGuestClock(day.Arrival)
	if err != nil {
		return err
	}
	pickupAt, err := holidayGuestClock(day.Pickup)
	if err != nil {
		return err
	}
	if arrival {
		reason := holidayOwnChildReason(day)
		if _, err := c.care.CreateArrivalException(ctx, careplan.ArrivalException{
			StudentID: day.StudentID, ExceptionDate: careplan.Date(day.Day), ExpectedArrival: &arrivalAt, Reason: &reason,
			Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
		}); err != nil {
			return err
		}
	}
	if pickup {
		reason := holidayOwnChildReason(day)
		if _, err := c.care.CreatePickupException(ctx, careplan.PickupException{
			StudentID: day.StudentID, ExceptionDate: careplan.Date(day.Day), PickupTime: &pickupAt, Reason: &reason,
			Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
		}); err != nil {
			return err
		}
	}
	return nil
}
