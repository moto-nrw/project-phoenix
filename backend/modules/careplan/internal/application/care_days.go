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
	times, err := q.ResolveCareTimes(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[calendar.Date]careplan.CareDayStatus, len(times))
	for id, byDate := range times {
		days := make(map[calendar.Date]careplan.CareDayStatus, len(byDate))
		for date, careTime := range byDate {
			days[date] = careTime.Status
		}
		result[id] = days
	}
	return result, nil
}

// ResolveCareTimes is ResolveForRange with the effective times each verdict
// was derived from (#4774), so both read one derivation.
func (q *CareDayQueries) ResolveCareTimes(ctx context.Context, ids []int64, from, to calendar.Date) (map[int64]map[calendar.Date]careplan.CareTime, error) {
	result := make(map[int64]map[calendar.Date]careplan.CareTime, len(ids))
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
		days := make(map[calendar.Date]careplan.CareTime)
		for date := from; !date.After(to); date = date.AddDays(1) {
			days[date] = careTimeOf(facts[id][date], participating[date][id])
		}
		result[id] = days
	}
	return result, nil
}

// careTimeOf is the verdict of one child-day with the times it rests on; a
// child outside care participation is not scheduled whatever the plan says.
func careTimeOf(fact careplan.CareDayFacts, participating bool) careplan.CareTime {
	status := careplan.ResolveCareDay(fact)
	if !participating {
		status = careplan.CareDayNotScheduled
	}
	careTime := careplan.CareTime{Status: status}
	if fact.Arrival != nil {
		careTime.Arrival = fact.Arrival.ArrivalTime
	}
	if fact.Pickup != nil {
		careTime.Pickup = fact.Pickup.PickupTime
	}
	return careTime
}

var (
	_ careplan.CareDayQuery  = (*CareDayQueries)(nil)
	_ careplan.CareTimeQuery = (*CareDayQueries)(nil)
)
