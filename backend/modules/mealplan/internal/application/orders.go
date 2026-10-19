package application

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// orderReachDays is how far ahead a new weekly pattern can change ordered
// days: the four weeks parents reach.
const orderReachDays = 27

// dayChildren are the children who eat on a day as the registration stands
// now, with their class and the account of the day's exception.
type dayChildren struct {
	classes    map[int64]string
	changedBy  map[int64]*int64
	candidates map[int64]domain.DailyCandidate
}

// childrenOn resolves who eats on date, counting absences reported up to
// cutoff. A closed day has nobody.
func (s *Service) childrenOn(ctx context.Context, date domain.Date, cutoff time.Time, days domain.CalendarDays) (dayChildren, domain.OperationStats, error) {
	result := dayChildren{classes: map[int64]string{}, changedBy: map[int64]*int64{}, candidates: map[int64]domain.DailyCandidate{}}
	if days.Closed[date] {
		return result, domain.OperationStats{}, nil
	}
	candidates, stats, err := s.directory.FindDailyCandidates(ctx, date)
	if err != nil {
		return result, stats, err
	}
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.StudentID)
		result.candidates[candidate.StudentID] = candidate
	}
	states, participationStats, err := s.store.FindDailyParticipation(ctx, ids, date, cutoff)
	stats = addStats(stats, participationStats)
	if err != nil {
		return result, stats, err
	}
	for _, candidate := range candidates {
		state := states[candidate.StudentID]
		day := dayState{holiday: days.Holiday[date], scheduled: state.Regular, override: state.Override, absences: state.Absences}
		result.changedBy[candidate.StudentID] = state.OverrideBy
		if resolveDay(day, cutoff).participating {
			result.classes[candidate.StudentID] = candidate.SchoolClass
		}
	}
	return result, stats, nil
}

// fixDueOrders fixes every order of [from, to] whose deadline has passed and
// that does not exist yet (#3873). The caller holds the order lock. A weekly
// order fixes all open days of its week at the earliest deadline among them;
// a day that already belongs to an order is never ordered again.
func (s *Service) fixDueOrders(ctx context.Context, rules domain.OrderingRules, state domain.OrderState, days domain.CalendarDays, from, to domain.Date, now time.Time) (int, domain.OperationStats, error) {
	var stats domain.OperationStats
	fixed := 0
	for start := from; !start.After(to); {
		periodStart, periodEnd := domain.OrderPeriod(rules.Rhythm, start)
		open, deadline := openDays(rules, state, periodStart, periodEnd)
		if len(open) > 0 && now.After(deadline) {
			orderStats, err := s.fixOrder(ctx, open, deadline, days, now)
			stats = addStats(stats, orderStats)
			if err != nil {
				return fixed, stats, err
			}
			fixed++
		}
		start = nextPeriod(rules.Rhythm, periodStart, periodEnd)
	}
	return fixed, stats, nil
}

// nextPeriod is the first day after a period: the next day, or the next
// Monday of a weekly order.
func nextPeriod(rhythm domain.Rhythm, periodStart, periodEnd domain.Date) domain.Date {
	if rhythm == domain.RhythmWeekly {
		return periodStart.AddDays(7)
	}
	return periodEnd.AddDays(1)
}

// openDays lists the weekdays of a period no order holds yet and the
// earliest deadline among them.
func openDays(rules domain.OrderingRules, state domain.OrderState, from, to domain.Date) ([]domain.Date, time.Time) {
	var open []domain.Date
	var deadline time.Time
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		if _, ordered := state.OrderFor(date); ordered {
			continue
		}
		open = append(open, date)
		if freeze := rules.FreezeAt(date); deadline.IsZero() || freeze.Before(deadline) {
			deadline = freeze
		}
	}
	return open, deadline
}

// fixOrder stores the order of the open days as it stood at the deadline:
// absences count when they were reported by then.
func (s *Service) fixOrder(ctx context.Context, open []domain.Date, deadline time.Time, days domain.CalendarDays, now time.Time) (domain.OperationStats, error) {
	order := domain.NewOrder{PeriodStart: open[0], PeriodEnd: open[len(open)-1], DeadlineAt: deadline, Days: open}
	var stats domain.OperationStats
	for _, date := range open {
		children, childStats, err := s.childrenOn(ctx, date, deadline, days)
		stats = addStats(stats, childStats)
		if err != nil {
			return stats, err
		}
		for studentID, schoolClass := range children.classes {
			order.Lines = append(order.Lines, domain.OrderLine{Date: date, StudentID: studentID, SchoolClass: schoolClass})
		}
	}
	_, insertStats, err := s.store.InsertOrder(ctx, order, now)
	return addStats(stats, insertStats), err
}

// recordChanges compares each ordered day from today on with who eats now
// and stores the difference as numbered changes. A past day keeps its order:
// what reaches the history later (an archived absence) changes no bill.
func recordChanges(state domain.OrderState, current map[domain.Date]dayChildren, today domain.Date) []domain.OrderChange {
	dates := make([]domain.Date, 0, len(current))
	for date := range current {
		dates = append(dates, date)
	}
	slices.SortFunc(dates, func(a, b domain.Date) int { return a.Compare(b) })
	var changes []domain.OrderChange
	for _, date := range dates {
		children := current[date]
		order, ordered := state.OrderFor(date)
		if !ordered || date.Before(today) {
			continue
		}
		for _, change := range domain.DiffOrder(date, state.OrderedChildren(date), children.classes) {
			change.OrderID, change.ChangedBy = order.ID, children.changedBy[change.StudentID]
			changes = append(changes, change)
		}
	}
	return changes
}
