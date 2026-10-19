package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

var (
	// ErrSupplyActive refuses a second caterer while one supplies the school.
	ErrSupplyActive = errors.New("another caterer supplies the school")
	// ErrNotSupplied refuses a caterer the school is not supplied by.
	ErrNotSupplied = errors.New("the caterer does not supply the school")
	// ErrOrderNotFound: the school has no such order.
	ErrOrderNotFound = errors.New("meal order not found")
)

// StartSupply starts the school's supply by a caterer (ADR 0074). Starting
// the running supply again changes nothing; another caterer's is refused.
func (s *Service) StartSupply(ctx context.Context, catererID int64, deliveryAddress string, accountID int64) error {
	return mapPublicError(s.run(ctx, "start_supply", func(txCtx context.Context) (domain.OperationStats, error) {
		supply, active, stats, err := s.store.FindActiveSupply(txCtx)
		if err != nil {
			return stats, err
		}
		if active {
			if supply.CatererID == catererID {
				return stats, nil
			}
			return stats, ErrSupplyActive
		}
		insertStats, err := s.store.InsertSupply(txCtx, catererID, deliveryAddress, accountID, s.now())
		return addStats(stats, insertStats), err
	}))
}

// EndSupply ends the school's supply and reports whether one ran.
func (s *Service) EndSupply(ctx context.Context, accountID int64) (ended bool, err error) {
	err = s.run(ctx, "end_supply", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		ended, stats, err = s.store.EndSupply(txCtx, accountID, s.now())
		return stats, err
	})
	return ended, mapPublicError(err)
}

// CurrentSupply reads the school's running supply.
func (s *Service) CurrentSupply(ctx context.Context) (mealplan.Supply, bool, error) {
	var supply domain.Supply
	var active bool
	err := s.run(ctx, "read_supply", func(txCtx context.Context) (stats domain.OperationStats, err error) {
		supply, active, stats, err = s.store.FindActiveSupply(txCtx)
		return stats, err
	})
	return mealplan.Supply(supply), active, mapPublicError(err)
}

// CatererOrders is the school's orders of [from, to] as its caterer reads
// them: only while the caterer supplies the school, settled like every other
// read, and cut down to what the caterer may see.
func (s *Service) CatererOrders(ctx context.Context, catererID int64, from, to string) (mealplan.OrderWeek, error) {
	var view domain.OrderView
	err := s.run(ctx, "read_caterer_orders", func(txCtx context.Context) (domain.OperationStats, error) {
		stats, supplyErr := s.requireSupplier(txCtx, catererID)
		if supplyErr != nil {
			return stats, supplyErr
		}
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return stats, rulesErr
		}
		var settleStats domain.OperationStats
		var err error
		view, settleStats, err = s.settleOrders(txCtx, rules, domain.Date(from), domain.Date(to))
		return addStats(stats, settleStats), err
	})
	if err != nil {
		return mealplan.OrderWeek{}, mapPublicError(err)
	}
	return catererWeek(publicOrderWeek(view), view.Rules), nil
}

// ConfirmOrder records that the caterer received a fixed order. It is the
// caterer's only write (ADR 0074); confirming twice keeps the first.
func (s *Service) ConfirmOrder(ctx context.Context, catererID, orderID, accountID int64) error {
	return mapPublicError(s.run(ctx, "confirm_order", func(txCtx context.Context) (domain.OperationStats, error) {
		stats, err := s.requireSupplier(txCtx, catererID)
		if err != nil {
			return stats, err
		}
		found, confirmStats, err := s.store.ConfirmOrder(txCtx, orderID, accountID, s.now())
		stats = addStats(stats, confirmStats)
		if err == nil && !found {
			err = ErrOrderNotFound
		}
		return stats, err
	}))
}

func (s *Service) requireSupplier(ctx context.Context, catererID int64) (domain.OperationStats, error) {
	supply, active, stats, err := s.store.FindActiveSupply(ctx)
	if err != nil {
		return stats, err
	}
	if !active || supply.CatererID != catererID {
		return stats, ErrNotSupplied
	}
	return stats, nil
}

// catererWeek cuts the school's view down to what its caterer may read
// (ADR 0074): no child's ID, class numbers only when the school orders by
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
	if byClass || named {
		result.SchoolClass = child.SchoolClass
	}
	return result
}
