package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type WeeklyApprovals struct {
	People   ports.WeeklyApprovalPeople
	Arrivals ports.WeeklyApprovalArrivals
	Pickups  ports.WeeklyApprovalPickups
}

func (s *WeeklyApprovals) ApplyWeekly(ctx context.Context, request *carerequests.Request, actorID int64) (bool, error) {
	changes, err := carerequests.ParseWeekly(request.Payload)
	if err != nil {
		return false, err
	}
	if changes.IsEmpty() {
		return false, errors.New("schedule: no weekdays to apply")
	}
	ctx, companionsChanged := s.People.TrackCompanionChanges(ctx)
	staffID, err := s.People.ActingStaffID(ctx)
	if err != nil {
		return false, err
	}
	modes, err := s.People.LockDepartureModes(ctx, request.StudentID)
	if err != nil {
		return false, err
	}
	if len(changes.Modes) != 0 || len(changes.Scheduled) != 0 {
		merged := mergeDepartureModes(modes, changes)
		if err := s.People.SaveDepartureModes(ctx, request.StudentID, merged); err != nil {
			return false, err
		}
	}
	if err := s.applyCareDays(ctx, request.StudentID, changes.Scheduled); err != nil {
		return false, err
	}
	if err := s.applyArrivalTimes(ctx, request.StudentID, staffID, changes.Arrivals); err != nil {
		return false, err
	}
	if err := s.applyPickupTimes(ctx, request.StudentID, staffID, changes.Pickups); err != nil {
		return false, err
	}
	if err := s.People.AuditDepartureModes(ctx, request.StudentID, actorID); err != nil {
		return false, fmt.Errorf("schedule: audit care request departure: %w", err)
	}
	return companionsChanged(), nil
}

func mergeDepartureModes(current map[string][]string, changes carerequests.WeeklyChanges) map[string][]string {
	merged := make(map[string][]string, len(current))
	for day, modes := range current {
		merged[day] = append([]string(nil), modes...)
	}
	for day, mode := range changes.Modes {
		merged[day] = []string{mode}
	}
	days := [...]string{"mon", "tue", "wed", "thu", "fri"}
	for weekday, active := range changes.Scheduled {
		if !active {
			delete(merged, days[weekday-1])
		}
	}
	return merged
}
