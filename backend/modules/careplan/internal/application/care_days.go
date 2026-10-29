package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type CareDayQueries struct {
	source ports.CareDaySource
}

func NewCareDayQueries(source ports.CareDaySource) *CareDayQueries {
	if source == nil {
		panic("care plan care-day queries: source is required")
	}
	return &CareDayQueries{source: source}
}

func (q *CareDayQueries) ResolveForDate(ctx context.Context, ids []int64, date calendar.Date) (map[int64]careplan.CareDayStatus, error) {
	byStudent, err := q.ResolveForRange(ctx, ids, date, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]careplan.CareDayStatus, len(byStudent))
	for id, days := range byStudent {
		result[id] = days[date]
	}
	return result, nil
}

func (q *CareDayQueries) ResolveForRange(ctx context.Context, ids []int64, from, to calendar.Date) (map[int64]map[calendar.Date]careplan.CareDayStatus, error) {
	planned, err := q.ResolvePlannedForRange(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[calendar.Date]careplan.CareDayStatus, len(planned))
	for id, byDate := range planned {
		days := make(map[calendar.Date]careplan.CareDayStatus, len(byDate))
		for date, day := range byDate {
			days[date] = day.Status
		}
		result[id] = days
	}
	return result, nil
}

// ResolvePlannedForRange resolves the verdict per child and day together with
// the planned arrival and pickup times. A child who is not expected carries no
// times: the plan's clock values only mean something on a day they come.
func (q *CareDayQueries) ResolvePlannedForRange(ctx context.Context, ids []int64, from, to calendar.Date) (map[int64]map[calendar.Date]careplan.PlannedCareDay, error) {
	result := make(map[int64]map[calendar.Date]careplan.PlannedCareDay, len(ids))
	if len(ids) == 0 || to.Before(from) {
		return result, nil
	}
	facts, err := q.source.LoadCareDayFacts(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	participating, err := q.source.ParticipatingStudentIDsByDate(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		days := make(map[calendar.Date]careplan.PlannedCareDay)
		for date := from; !date.After(to); date = date.AddDays(1) {
			days[date] = plannedCareDay(facts[id][date], participating[date][id])
		}
		result[id] = days
	}
	return result, nil
}

func plannedCareDay(facts careplan.CareDayFacts, participating bool) careplan.PlannedCareDay {
	day := careplan.PlannedCareDay{Status: careplan.ResolveCareDay(facts)}
	if !participating {
		day.Status = careplan.CareDayNotScheduled
	}
	if !day.Status.Expected() {
		return day
	}
	if facts.Arrival != nil {
		day.Arrival = facts.Arrival.ArrivalTime
	}
	if facts.Pickup != nil {
		day.Pickup = facts.Pickup.PickupTime
	}
	return day
}

var _ careplan.CareDayQuery = (*CareDayQueries)(nil)
