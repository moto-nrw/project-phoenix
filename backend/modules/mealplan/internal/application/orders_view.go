package application

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// orderView returns the school's orders of [from, to] (#3873). A read
// writes nothing and takes no lock: it lays out the due orders the worker
// has not stored yet and the changes nobody recorded yet as they will be
// stored, so school, caterer and export see the same numbers.
func (s *Service) orderView(ctx context.Context, from, to domain.Date) (view domain.OrderView, err error) {
	err = s.run(ctx, "read_orders", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		var stats domain.OperationStats
		view, stats, err = s.provisionalOrders(txCtx, rules, from, to)
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

// window reads the calendar and the orders of the periods [from, to]
// touches.
func (s *Service) window(ctx context.Context, rules domain.OrderingRules, from, to domain.Date) (orderWindow, domain.Date, domain.Date, domain.OperationStats, error) {
	periodStart, _ := domain.OrderPeriod(rules.Rhythm, from)
	_, periodEnd := domain.OrderPeriod(rules.Rhythm, to)
	days, stats, err := s.calendar.Days(ctx, periodStart, periodEnd)
	if err != nil {
		return orderWindow{}, periodStart, periodEnd, stats, err
	}
	state, findStats, err := s.store.FindOrders(ctx, periodStart, periodEnd)
	return orderWindow{days: days, state: state}, periodStart, periodEnd, addStats(stats, findStats), err
}

// lockedWindow takes the order lock, reads the window's periods and fixes
// their due orders, re-reading after a new one. Only the worker and a
// registration write call it.
func (s *Service) lockedWindow(ctx context.Context, rules domain.OrderingRules, from, to domain.Date, now time.Time) (orderWindow, domain.OperationStats, error) {
	stats, err := s.store.LockOrders(ctx)
	if err != nil {
		return orderWindow{}, stats, err
	}
	window, periodStart, periodEnd, windowStats, err := s.window(ctx, rules, from, to)
	stats = addStats(stats, windowStats)
	if err != nil {
		return orderWindow{}, stats, err
	}
	fixed, fixStats, err := s.fixDueOrders(ctx, rules, window.state, window.days, periodStart, periodEnd, now)
	stats = addStats(stats, fixStats)
	if err != nil || fixed == 0 {
		return window, stats, err
	}
	window.state, fixStats, err = s.store.FindOrders(ctx, periodStart, periodEnd)
	return window, addStats(stats, fixStats), err
}

// provisionalWindow reads the window's periods and adds in memory what the
// worker would store now: every due order without an ID yet.
func (s *Service) provisionalWindow(ctx context.Context, rules domain.OrderingRules, from, to domain.Date, now time.Time, seen registrations) (orderWindow, domain.OperationStats, error) {
	window, periodStart, periodEnd, stats, err := s.window(ctx, rules, from, to)
	if err != nil {
		return window, stats, err
	}
	due, dueStats, err := s.dueOrders(ctx, rules, window.state, window.days, periodStart, periodEnd, now, seen)
	stats = addStats(stats, dueStats)
	for _, order := range due {
		window.state.Orders = append(window.state.Orders, domain.Order{PeriodStart: order.PeriodStart, PeriodEnd: order.PeriodEnd, DeadlineAt: order.DeadlineAt, FrozenAt: now, Days: order.Days})
		window.state.Lines = append(window.state.Lines, order.Lines...)
	}
	return window, stats, err
}

// settleDueOrders fixes the school's due orders and records every change of an
// ordered day since (#3873). The worker runs it every few minutes, so an order
// is fixed close to its deadline and a change nobody reads is still recorded
// before its day ends. A school without meal registration has nothing to
// settle.
func (s *Service) settleDueOrders(ctx context.Context) error {
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

// settleWrite runs a registration write between settling the orders of
// [from, to] as they stood before it and recording what it changed in them.
// Settling first records what changed before the write (a sick note since
// the last run) without an author, so only the write's own changes name it.
func (s *Service) settleWrite(ctx context.Context, rules domain.OrderingRules, from, to domain.Date, author changeAuthor, write func() (domain.OperationStats, error)) (domain.OperationStats, error) {
	now := s.now()
	window, stats, err := s.lockedWindow(ctx, rules, from, to, now)
	if err != nil {
		return stats, err
	}
	_, pending, pendingStats, err := s.recordWindow(ctx, rules, window, from, to, now, nil, false)
	stats = addStats(stats, pendingStats)
	if err != nil {
		return stats, err
	}
	window.state.Changes = append(window.state.Changes, pending...)
	writeStats, err := write()
	stats = addStats(stats, writeStats)
	if err != nil {
		return stats, err
	}
	_, _, recordStats, err := s.recordWindow(ctx, rules, window, from, to, now, &author, false)
	return addStats(stats, recordStats), err
}

// recordWindow resolves who eats on the ordered days of [from, to] from
// today on and records their changes.
func (s *Service) recordWindow(ctx context.Context, rules domain.OrderingRules, window orderWindow, from, to domain.Date, now time.Time, author *changeAuthor, all bool) (map[domain.Date]dayChildren, []domain.OrderChange, domain.OperationStats, error) {
	today := domain.DateFromInstant(now)
	current, stats, err := s.currentChildren(ctx, rules, window, from, to, today, all, nil)
	if err != nil {
		return nil, nil, stats, err
	}
	numbered, insertStats, err := s.store.InsertOrderChanges(ctx, recordChanges(window.state, current, today, author), now)
	return current, numbered, addStats(stats, insertStats), err
}

// currentChildren resolves who eats on each weekday of [from, to] from today
// on as the registration stands now: the ordered days, and with all the
// open days too, for the view.
func (s *Service) currentChildren(ctx context.Context, rules domain.OrderingRules, window orderWindow, from, to, today domain.Date, all bool, seen registrations) (map[domain.Date]dayChildren, domain.OperationStats, error) {
	current := make(map[domain.Date]dayChildren)
	var stats domain.OperationStats
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Before(today) || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		if _, ordered := window.state.OrderFor(date); !ordered && !all {
			continue
		}
		children, childStats, err := s.childrenOn(ctx, rules, date, rules.AbsenceCutoff(date), window.days, seen)
		stats = addStats(stats, childStats)
		if err != nil {
			return nil, stats, err
		}
		current[date] = children
	}
	return current, stats, nil
}

// provisionalOrders lays out the weekdays of [from, to] without writing: the
// due orders not stored yet and the changes since the last recording are
// derived in memory, the changes without a number.
func (s *Service) provisionalOrders(ctx context.Context, rules domain.OrderingRules, from, to domain.Date) (domain.OrderView, domain.OperationStats, error) {
	now := s.now()
	today := domain.DateFromInstant(now)
	seen := registrations{}
	window, stats, err := s.provisionalWindow(ctx, rules, from, to, now, seen)
	if err != nil {
		return domain.OrderView{}, stats, err
	}
	current, currentStats, err := s.currentChildren(ctx, rules, window, from, to, today, true, seen)
	stats = addStats(stats, currentStats)
	if err != nil {
		return domain.OrderView{}, stats, err
	}
	for _, change := range recordChanges(window.state, current, today, nil) {
		change.DetectedAt = now
		window.state.Changes = append(window.state.Changes, change)
	}
	view := buildOrderView(rules, window, current, from, to, today)
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
			day.FrozenChildren = window.state.FrozenChildren(date)
		} else if date.Before(today) {
			day.Unordered = true
		} else {
			// A weekly order freezes all remaining days at their earliest
			// deadline, even when the view shows only part of the week.
			start, end := domain.OrderPeriod(rules.Rhythm, date)
			_, day.Deadline = openDays(rules, window.state, start, end, today)
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
