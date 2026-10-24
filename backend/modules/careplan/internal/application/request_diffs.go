package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	switch request.RequestKind {
	case "pickup_change":
		diff, err = s.pickup(ctx, request)
	case carerequests.KindPickupPermission:
		diff, err = storedPermissionDiff(request)
	default:
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

// storedPermissionDiff freezes a pickup permission request (#4780) with the
// days the person held at submission.
func storedPermissionDiff(request *carerequests.Request) ([]carerequests.DiffEntry, error) {
	permission, err := carerequests.ParsePickupPermission(request.Payload)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Previous string `json:"previous_label"`
	}
	_ = json.Unmarshal(request.Payload, &payload)
	return carerequests.PickupPermissionEntries(permission, payload.Previous), nil
}

func (s *RequestDiffs) pickup(ctx context.Context, request *carerequests.Request) ([]carerequests.DiffEntry, error) {
	var payload map[string]any
	if err := json.Unmarshal(request.Payload, &payload); err != nil {
		return nil, err
	}
	change, err := carerequests.ParsePickup(request.Payload)
	if err != nil {
		return nil, err
	}
	old := ""
	if change.PickupTime != nil {
		if old, err = s.oldPickup(ctx, request.StudentID, change.Date, payload); err != nil {
			return nil, err
		}
	}
	// The Abholart before the request was recorded at submission (#3945);
	// the snapshot freezes that, not a plan the decision itself changes.
	return carerequests.PickupChangeEntries(change, old, carerequests.StoredPickupTerms(request.Payload).PreviousDepartureModes), nil
}

func (s *RequestDiffs) oldPickup(ctx context.Context, studentID int64, date calendar.Date, payload map[string]any) (string, error) {
	if old, _ := payload["previous_pickup_time"].(string); old != "" {
		return old, nil
	}
	value, err := s.Pickups.ExceptionPickupTime(ctx, studentID, careplan.Date(date))
	if err != nil {
		return "", err
	}
	if value == nil {
		if value, err = s.Pickups.EffectivePickupTime(ctx, studentID, careplan.Date(date)); err != nil {
			return "", err
		}
	}
	if value == nil {
		return "", nil
	}
	return value.Format("15:04"), nil
}
