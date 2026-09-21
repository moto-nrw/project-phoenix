package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestDiffs struct {
	People   ports.RequestDiffPeople
	Arrivals ports.RequestDiffArrivals
	Pickups  ports.RequestDiffPickups
	Logger   *slog.Logger
}

func (s *RequestDiffs) Weekly(ctx context.Context, studentID int64, raw json.RawMessage) ([]carerequests.DiffEntry, error) {
	var payload carerequests.WeeklyChange
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	modes, err := s.People.DepartureModesForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load student for diff: %w", err)
	}
	arrivals, err := s.Arrivals.GetStudentArrivalSchedules(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load arrival schedules for diff: %w", err)
	}
	pickups, err := s.Pickups.GetStudentPickupSchedules(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load pickup schedules for diff: %w", err)
	}
	plan := requestWeeklyPlan(modes, arrivals, pickups)
	return carerequests.WeeklyDiff(payload.Weekdays, plan), nil
}

func requestWeeklyPlan(modes map[string][]string, arrivals []*careplan.ArrivalSchedule, pickups []*careplan.PickupSchedule) carerequests.WeeklyPlan {
	plan := carerequests.WeeklyPlan{ArrivalDays: map[int]bool{}, ArrivalTimes: map[int]string{}, PickupTimes: map[int]string{}, DepartureModes: map[string][]string{}}
	for day, values := range modes {
		if len(values) == 0 {
			plan.DepartureModes[day] = []string{"alone"}
		} else {
			plan.DepartureModes[day] = append([]string(nil), values...)
		}
	}
	for _, row := range arrivals {
		plan.ArrivalDays[row.Weekday] = true
		if !row.ExpectedArrival.IsZero() {
			plan.ArrivalTimes[row.Weekday] = row.ExpectedArrival.Format("15:04")
		}
	}
	for _, row := range pickups {
		plan.PickupTimes[row.Weekday] = row.PickupTime.Format("15:04")
	}
	return plan
}

// Snapshot is best-effort presentation. Decisions must not depend on live-diff availability.
func (s *RequestDiffs) Snapshot(ctx context.Context, request *carerequests.Request) *carerequests.DecisionSnapshot {
	var diff []carerequests.DiffEntry
	var err error
	if request.RequestKind == "pickup_change" {
		diff, err = s.pickup(ctx, request)
	} else {
		diff, err = s.Weekly(ctx, request.StudentID, request.Payload)
	}
	if err != nil {
		s.Logger.Warn("schedule: build care request decision snapshot failed",
			"request_id", request.ID,
			"error", err.Error(),
		)
		return nil
	}
	return carerequests.FreezeDecision(diff)
}

func (s *RequestDiffs) pickup(ctx context.Context, request *carerequests.Request) ([]carerequests.DiffEntry, error) {
	var payload map[string]any
	if err := json.Unmarshal(request.Payload, &payload); err != nil {
		return nil, err
	}
	date, pickupTime, _, err := carerequests.ParsePickup(request.Payload)
	if err != nil {
		return nil, err
	}
	old, _ := payload["previous_pickup_time"].(string)
	if old == "" {
		value, findErr := s.Pickups.ExceptionPickupTime(ctx, request.StudentID, careplan.Date(date))
		if findErr != nil {
			return nil, findErr
		}
		if value != nil {
			old = value.Format("15:04")
		}
	}
	if old == "" {
		value, findErr := s.Pickups.EffectivePickupTime(ctx, request.StudentID, careplan.Date(date))
		if findErr != nil {
			return nil, findErr
		}
		if value != nil {
			old = value.Format("15:04")
		}
	}
	return []carerequests.DiffEntry{{Label: date.Format("02.01.2006") + " · Abholzeit", Old: old, New: pickupTime.Format("15:04"), CareKind: carerequests.KindPickup}}, nil
}
