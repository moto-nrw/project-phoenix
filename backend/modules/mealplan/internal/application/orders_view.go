package application

import (
	"context"
	"maps"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// Orders returns the school's orders of [from, to] (#3873). Reading settles
// them first: every due order is fixed and every change since is numbered,
// so school, caterer and export always see the same state.
func (s *Service) Orders(ctx context.Context, from, to domain.Date) (view domain.OrderView, err error) {
	err = s.run(ctx, "read_orders", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		var stats domain.OperationStats
		view, stats, err = s.settleOrders(txCtx, rules, from, to)
		return stats, err
	})
	return view, err
}

// orderWindow is everything settling a window needs: the calendar of its
// periods and the orders in them.
type orderWindow struct {
	days  domain.CalendarDays
	state domain.OrderState
}

// lockedWindow takes the order lock, reads the window's periods and fixes
// their due orders, re-reading after a new one.
func (s *Service) lockedWindow(ctx context.Context, rules domain.OrderingRules, from, to domain.Date, now time.Time) (orderWindow, domain.OperationStats, error) {
	stats, err := s.store.LockOrders(ctx)
	if err != nil {
		return orderWindow{}, stats, err
	}
	periodStart, _ := domain.OrderPeriod(rules.Rhythm, from)
	_, periodEnd := domain.OrderPeriod(rules.Rhythm, to)
	days, calendarStats, err := s.calendar.Days(ctx, periodStart, periodEnd)
	stats = addStats(stats, calendarStats)
	if err != nil {
		return orderWindow{}, stats, err
	}
	state, findStats, err := s.store.FindOrders(ctx, periodStart, periodEnd)
	stats = addStats(stats, findStats)
	if err != nil {
		return orderWindow{}, stats, err
	}
	fixed, fixStats, err := s.fixDueOrders(ctx, rules, state, days, periodStart, periodEnd, now)
	stats = addStats(stats, fixStats)
	if err != nil || fixed == 0 {
		return orderWindow{days: days, state: state}, stats, err
	}
	state, findStats, err = s.store.FindOrders(ctx, periodStart, periodEnd)
	return orderWindow{days: days, state: state}, addStats(stats, findStats), err
}

// settleOrders fixes the due orders of the window, records the changes from
// today on and lays out the weekdays.
func (s *Service) settleOrders(ctx context.Context, rules domain.OrderingRules, from, to domain.Date) (domain.OrderView, domain.OperationStats, error) {
	now := s.now()
	window, stats, err := s.lockedWindow(ctx, rules, from, to, now)
	if err != nil {
		return domain.OrderView{}, stats, err
	}
	today := domain.DateFromInstant(now)
	current := make(map[domain.Date]dayChildren)
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Before(today) || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		children, childStats, childErr := s.childrenOn(ctx, date, rules.AbsenceCutoff(date), window.days)
		stats = addStats(stats, childStats)
		if childErr != nil {
			return domain.OrderView{}, stats, childErr
		}
		current[date] = children
	}
	numbered, insertStats, err := s.store.InsertOrderChanges(ctx, recordChanges(window.state, current, today), now)
	stats = addStats(stats, insertStats)
	if err != nil {
		return domain.OrderView{}, stats, err
	}
	window.state.Changes = append(window.state.Changes, numbered...)
	view := buildOrderView(rules, window, current, from, to)
	names, nameStats, err := s.orderNames(ctx, view, current)
	view.Names = names
	for index := range view.Days {
		view.Days[index].Name(names)
	}
	return view, addStats(stats, nameStats), err
}

// buildOrderView lays out the weekdays of [from, to].
func buildOrderView(rules domain.OrderingRules, window orderWindow, current map[domain.Date]dayChildren, from, to domain.Date) domain.OrderView {
	view := domain.OrderView{Rules: rules}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		day := domain.OrderDay{Date: date, Closed: window.days.Closed[date], Deadline: rules.FreezeAt(date), Children: current[date].classes}
		if order, ordered := window.state.OrderFor(date); ordered {
			day.Order, day.Deadline, day.Children = &order, order.DeadlineAt, window.state.OrderedChildren(date)
		}
		for _, change := range window.state.Changes {
			if change.Date == date {
				day.Changes = append(day.Changes, change)
			}
		}
		if day.Children == nil {
			day.Children = map[int64]string{}
		}
		view.Days = append(view.Days, day)
	}
	return view
}

// orderNames names every child of the view: from today's registration, and
// for children of past orders from the directory.
func (s *Service) orderNames(ctx context.Context, view domain.OrderView, current map[domain.Date]dayChildren) (map[int64]domain.DailyCandidate, domain.OperationStats, error) {
	names := make(map[int64]domain.DailyCandidate)
	for _, children := range current {
		maps.Copy(names, children.candidates)
	}
	missing := unnamedChildren(view, names)
	if len(missing) == 0 {
		return names, domain.OperationStats{}, nil
	}
	found, stats, err := s.directory.FindStudentNames(ctx, missing)
	maps.Copy(names, found)
	return names, stats, err
}

// unnamedChildren lists the children and changed children of the view
// without a name yet, each once.
func unnamedChildren(view domain.OrderView, names map[int64]domain.DailyCandidate) []int64 {
	seen := make(map[int64]bool)
	var missing []int64
	add := func(studentID int64) {
		if _, named := names[studentID]; !named && !seen[studentID] {
			missing, seen[studentID] = append(missing, studentID), true
		}
	}
	for _, day := range view.Days {
		for studentID := range day.Children {
			add(studentID)
		}
		for _, change := range day.Changes {
			add(change.StudentID)
		}
	}
	return missing
}
