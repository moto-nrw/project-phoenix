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
	result := make(map[int64]map[calendar.Date]careplan.CareDayStatus, len(ids))
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
		days := make(map[calendar.Date]careplan.CareDayStatus)
		for date := from; !date.After(to); date = date.AddDays(1) {
			status := careplan.ResolveCareDay(facts[id][date])
			if !participating[date][id] {
				status = careplan.CareDayNotScheduled
			}
			days[date] = status
		}
		result[id] = days
	}
	return result, nil
}

var _ careplan.CareDayQuery = (*CareDayQueries)(nil)
