package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *WeeklyApprovals) applyCareDays(ctx context.Context, studentID int64, changes map[int]bool) error {
	if len(changes) == 0 {
		return nil
	}
	arrivals, err := s.Arrivals.GetStudentArrivalSchedules(ctx, studentID)
	if err != nil {
		return fmt.Errorf("apply care days: load arrivals: %w", err)
	}
	pickups, err := s.Pickups.GetStudentPickupSchedules(ctx, studentID)
	if err != nil {
		return fmt.Errorf("apply care days: load pickups: %w", err)
	}
	if err := s.checkCareDayBookings(ctx, studentID, changes); err != nil {
		return err
	}
	for weekday, active := range changes {
		if !active {
			if err := s.deleteCareDay(ctx, weekday, arrivals, pickups); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *WeeklyApprovals) checkCareDayBookings(ctx context.Context, studentID int64, changes map[int]bool) error {
	for weekday, active := range changes {
		if active {
			continue
		}
		managed, err := s.Pickups.HasBookedOfferingPickupForWeekday(ctx, studentID, weekday)
		if err != nil {
			return fmt.Errorf("apply care days: check booked offering: %w", err)
		}
		if managed {
			return carerequests.ErrCareDayManagedByBooking
		}
	}
	return nil
}

func (s *WeeklyApprovals) deleteCareDay(ctx context.Context, weekday int, arrivals []*careplan.ArrivalSchedule, pickups []*careplan.PickupSchedule) error {
	for _, row := range arrivals {
		if row.Weekday == weekday {
			if err := s.Arrivals.DeleteStudentArrivalSchedule(ctx, row.ID); err != nil {
				return fmt.Errorf("apply care day weekday %d: delete arrival: %w", weekday, err)
			}
		}
	}
	for _, row := range pickups {
		if row.Weekday == weekday {
			if err := s.Pickups.DeleteStudentPickupSchedule(ctx, row.ID); err != nil {
				return fmt.Errorf("apply care day weekday %d: delete pickup: %w", weekday, err)
			}
		}
	}
	return nil
}

func (s *WeeklyApprovals) applyArrivalTimes(ctx context.Context, studentID, staffID int64, changes map[int]string) error {
	for weekday, hhmm := range changes {
		clock, err := parseWallClock(hhmm)
		if err != nil {
			return fmt.Errorf("apply arrival weekday %d: %w", weekday, err)
		}
		row := &careplan.ArrivalSchedule{StudentID: studentID, Weekday: weekday, ExpectedArrival: clock, CreatedBy: staffID}
		if err := s.Arrivals.UpsertStudentArrivalSchedule(ctx, row); err != nil {
			return fmt.Errorf("apply arrival weekday %d: %w", weekday, err)
		}
	}
	return nil
}

func (s *WeeklyApprovals) applyPickupTimes(ctx context.Context, studentID, staffID int64, changes map[int]string) error {
	for weekday, hhmm := range changes {
		clock, err := parseWallClock(hhmm)
		if err != nil {
			return fmt.Errorf("apply pickup weekday %d: %w", weekday, err)
		}
		row := &careplan.PickupSchedule{StudentID: studentID, Weekday: weekday, PickupTime: clock, CreatedBy: staffID}
		if err := s.Pickups.UpsertStudentPickupSchedule(ctx, row); err != nil {
			return fmt.Errorf("apply pickup weekday %d: %w", weekday, err)
		}
	}
	return nil
}

// parseWallClock reads an HH:MM value as a wall-clock time for a TIME column.
func parseWallClock(hhmm string) (time.Time, error) {
	clock, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return calendar.NormalizeWallClock(clock), nil
}
