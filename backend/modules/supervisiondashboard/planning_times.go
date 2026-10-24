package supervisiondashboard

import (
	"context"
	"fmt"
)

// The planning times (#2703): the effective pickup and arrival of each child
// in the projection, behind the planning gate.

// PickupTime / ArrivalTime keep the field names of the bulk endpoints they
// replace so the frontend mapping stays identical.
type PickupTime struct {
	StudentID   int64     `json:"student_id,string"`
	Date        string    `json:"date"`
	WeekdayName string    `json:"weekday_name"`
	PickupTime  *string   `json:"pickup_time,omitempty"`
	IsException bool      `json:"is_exception"`
	Notes       string    `json:"notes,omitempty"`
	DayNotes    []DayNote `json:"day_notes,omitempty"`

	// DayDepartureMode is the Abholart set for this day only (#3946).
	DayDepartureMode string `json:"day_departure_mode,omitempty"`
}

type ArrivalTime struct {
	StudentID       int64     `json:"student_id,string"`
	Date            string    `json:"date"`
	WeekdayName     string    `json:"weekday_name"`
	ExpectedArrival *string   `json:"expected_arrival,omitempty"`
	IsException     bool      `json:"is_exception"`
	Notes           string    `json:"notes,omitempty"`
	DayNotes        []DayNote `json:"day_notes,omitempty"`
}

// loadPlanningTimes mirrors the former bulk pickup/arrival endpoints, which
// were gated on users:read; without it the sections stay deterministically
// empty. Times are additionally scoped to full-access callers, matching the
// actual arrival/pickup gate on the visit rows.
func (s *service) loadPlanningTimes(ctx context.Context, projection *Projection, caller Caller, studentIDs []int64, fullAccess bool, businessDay Date) error {
	if !planningVisible(caller, studentIDs, fullAccess) {
		return nil
	}
	pickups, err := s.deps.Planning.Pickups(ctx, studentIDs, businessDay)
	if err != nil {
		return fmt.Errorf("load pickup times: %w", err)
	}
	arrivals, err := s.deps.Planning.Arrivals(ctx, studentIDs, businessDay)
	if err != nil {
		return fmt.Errorf("load arrival times: %w", err)
	}

	for _, id := range studentIDs {
		if effective, ok := pickups[id]; ok {
			projection.PickupTimes = append(projection.PickupTimes, pickupTimeFromEffective(id, effective))
		}
		if effective, ok := arrivals[id]; ok {
			projection.ArrivalTimes = append(projection.ArrivalTimes, arrivalTimeFromEffective(id, effective))
		}
	}
	return nil
}

// planningVisible is the planning gate: users:read and full access, for at
// least one child. The planning times and today's hints of the Lehrkraft
// (#4795) share it.
func planningVisible(caller Caller, studentIDs []int64, fullAccess bool) bool {
	return len(studentIDs) > 0 && fullAccess && caller.CanReadStudents
}

func pickupTimeFromEffective(studentID int64, effective Pickup) PickupTime {
	return PickupTime{
		StudentID: studentID, Date: string(effective.Date), WeekdayName: effective.WeekdayName,
		PickupTime: effective.PickupTime, IsException: effective.IsException, Notes: effective.Notes,
		DayNotes: effective.DayNotes, DayDepartureMode: effective.DayDepartureMode,
	}
}

func arrivalTimeFromEffective(studentID int64, effective Arrival) ArrivalTime {
	return ArrivalTime{
		StudentID: studentID, Date: string(effective.Date), WeekdayName: effective.WeekdayName,
		ExpectedArrival: effective.ArrivalTime, IsException: effective.IsException, Notes: effective.Notes,
		DayNotes: effective.DayNotes,
	}
}
