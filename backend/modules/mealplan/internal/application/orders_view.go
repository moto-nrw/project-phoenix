package application

import (
	"context"
	"errors"
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

// SettleOrders fixes the school's due orders and records every change of an
// ordered day since (#3873). The worker runs it every few minutes, so an order
// is fixed close to its deadline and a change nobody reads is still recorded
// before its day ends. A school without meal registration has nothing to
// settle.
func (s *Service) SettleOrders(ctx context.Context) error {
	return s.run(ctx, "settle_orders", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, err := s.requireRegistration(txCtx)
		if errors.Is(err, ErrDisabled) || errors.Is(err, ErrRegistrationDisabled) {
			return domain.OperationStats{}, nil
		}
		if err != nil {
			return domain.OperationStats{}, err
		}
		now := s.now()
		today := domain.DateFromInstant(now)
		window, stats, err := s.lockedWindow(txCtx, rules, today, today.AddDays(orderReachDays), now)
		if err != nil {
			return stats, err
		}
		_, _, recordStats, err := s.recordWindow(txCtx, rules, window, today, today.AddDays(orderReachDays), now, nil, false)
		return addStats(stats, recordStats), err
	})
}

// settleWrite runs a registration write between fixing the due orders of
// [from, to], as they stood before it, and recording what it changed in
// them. The changes of the written child name its author.
func (s *Service) settleWrite(ctx context.Context, rules domain.OrderingRules, from, to domain.Date, author changeAuthor, write func() (domain.OperationStats, error)) (domain.OperationStats, error) {
	now := s.now()
	window, stats, err := s.lockedWindow(ctx, rules, from, to, now)
	if err != nil {
		return stats, err
	}
	writeStats, err := write()
	stats = addStats(stats, writeStats)
	if err != nil {
		return stats, err
	}
	_, _, recordStats, err := s.recordWindow(ctx, rules, window, from, to, now, &author, false)
	return addStats(stats, recordStats), err
}

// recordWindow resolves who eats on each weekday of [from, to] from today on
// and records the changes of the ordered days. With all it resolves the open
// days too, for the view.
func (s *Service) recordWindow(ctx context.Context, rules domain.OrderingRules, window orderWindow, from, to domain.Date, now time.Time, author *changeAuthor, all bool) (map[domain.Date]dayChildren, []domain.OrderChange, domain.OperationStats, error) {
	today := domain.DateFromInstant(now)
	current := make(map[domain.Date]dayChildren)
	var stats domain.OperationStats
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Before(today) || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		if _, ordered := window.state.OrderFor(date); !ordered && !all {
			continue
		}
		children, childStats, err := s.childrenOn(ctx, date, rules.AbsenceCutoff(date), window.days)
		stats = addStats(stats, childStats)
		if err != nil {
			return nil, nil, stats, err
		}
		current[date] = children
	}
	numbered, insertStats, err := s.store.InsertOrderChanges(ctx, recordChanges(window.state, current, today, author), now)
	return current, numbered, addStats(stats, insertStats), err
}

// settleOrders fixes the due orders of the window, records the changes from
// today on and lays out the weekdays.
func (s *Service) settleOrders(ctx context.Context, rules domain.OrderingRules, from, to domain.Date) (domain.OrderView, domain.OperationStats, error) {
	now := s.now()
	window, stats, err := s.lockedWindow(ctx, rules, from, to, now)
	if err != nil {
		return domain.OrderView{}, stats, err
	}
	current, numbered, recordStats, err := s.recordWindow(ctx, rules, window, from, to, now, nil, true)
	stats = addStats(stats, recordStats)
	if err != nil {
		return domain.OrderView{}, stats, err
	}
	window.state.Changes = append(window.state.Changes, numbered...)
	view := buildOrderView(rules, window, current, from, to, domain.DateFromInstant(now))
	names, nameStats, err := s.orderNames(ctx, view, current)
	view.Names = names
	for index := range view.Days {
		view.Days[index].Name(names)
	}
	return view, addStats(stats, nameStats), err
}

// buildOrderView lays out the weekdays of [from, to].
func buildOrderView(rules domain.OrderingRules, window orderWindow, current map[domain.Date]dayChildren, from, to, today domain.Date) domain.OrderView {
	view := domain.OrderView{Rules: rules}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		day := domain.OrderDay{Date: date, Closed: window.days.Closed[date], Deadline: rules.FreezeAt(date), Children: current[date].classes}
		if order, ordered := window.state.OrderFor(date); ordered {
			day.Order, day.Deadline, day.Children = &order, order.DeadlineAt, window.state.OrderedChildren(date)
		} else if date.Before(today) {
			day.Unordered = true
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
