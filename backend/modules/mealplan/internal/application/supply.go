package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// ErrOrderNotFound: the school has no such order.
var ErrOrderNotFound = errors.New("meal order not found")

// catererOrders is the school's orders of [from, to] as its caterer reads
// them: laid out like every other read, without a write or a lock, the
// delivery days from the start of its supply on, and cut down to what the
// caterer may see. Who supplies the school is no Meal Plan data (ADR 0077);
// the caterer portal checks it in the same school transaction and hands in
// when the supply started.
func (s *Service) catererOrders(ctx context.Context, since time.Time, from, to string) (mealplan.OrderWeek, error) {
	var view domain.OrderView
	err := s.run(ctx, "read_caterer_orders", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		var stats domain.OperationStats
		var err error
		view, stats, err = s.provisionalOrders(txCtx, rules, domain.Date(from), domain.Date(to))
		return stats, err
	})
	if err != nil {
		return mealplan.OrderWeek{}, mapPublicError(err)
	}
	view.Days = daysSince(view.Days, since)
	return catererWeek(publicOrderWeek(view), view.Rules), nil
}

// daysSince keeps the delivery days from the start of the supply on (ADR
// 0077): a caterer never reads a day its predecessor delivered, but reads
// every day it delivers, even one whose order was fixed before it started.
// A confirmation from before the start is its predecessor's and does not
// count for it.
func daysSince(days []domain.OrderDay, start time.Time) []domain.OrderDay {
	first := domain.DateFromInstant(start)
	kept := days[:0:0]
	for _, day := range days {
		if day.Date.Before(first) {
			continue
		}
		if day.Order != nil && day.Order.ConfirmedAt != nil && day.Order.ConfirmedAt.Before(start) {
			order := *day.Order
			order.ConfirmedAt = nil
			day.Order = &order
		}
		kept = append(kept, day)
	}
	return kept
}

// confirmOrder records that the caterer received a fixed order with a day of
// its supply. It is the caterer's only write (ADR 0077); confirming twice
// keeps the first, a predecessor's confirmation gives way to the caterer's
// own. A due order the worker has not stored yet has no ID, so it cannot be
// confirmed before it is stored: confirming never fixes an order.
func (s *Service) confirmOrder(ctx context.Context, orderID int64, since time.Time, accountID int64) error {
	return mapPublicError(s.run(ctx, "confirm_order", func(txCtx context.Context) (domain.OperationStats, error) {
		found, stats, err := s.store.ConfirmOrder(txCtx, orderID, accountID, domain.DateFromInstant(since), since, s.now())
		if err == nil && !found {
			err = ErrOrderNotFound
		}
		return stats, err
	}))
}

// catererWeek cuts the school's view down to what its caterer may read
// (ADR 0077): no child's ID, class numbers only when the school orders by
// class, and names only while the school shows them to the caterer.
func catererWeek(week mealplan.OrderWeek, rules domain.OrderingRules) mealplan.OrderWeek {
	byClass := rules.Detail == domain.DetailByClass
	for index := range week.Days {
		day := &week.Days[index]
		if !byClass {
			day.Classes = nil
		}
		if rules.CatererNames {
			day.Children = catererChildren(day.Children, byClass)
		} else {
			day.Children = nil
		}
		for change := range day.Changes {
			child := &day.Changes[change].Child
			*child = catererChild(*child, rules.CatererNames, byClass)
		}
	}
	return week
}

func catererChildren(children []mealplan.OrderChild, byClass bool) []mealplan.OrderChild {
	result := make([]mealplan.OrderChild, 0, len(children))
	for _, child := range children {
		result = append(result, catererChild(child, true, byClass))
	}
	return result
}

func catererChild(child mealplan.OrderChild, named, byClass bool) mealplan.OrderChild {
	result := mealplan.OrderChild{}
	if named {
		result.FirstName, result.LastName = child.FirstName, child.LastName
	}
	if byClass {
		result.SchoolClass = child.SchoolClass
	}
	return result
}
