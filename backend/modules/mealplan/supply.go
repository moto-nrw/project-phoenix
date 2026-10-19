package mealplan

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	// ErrSupplyActive refuses a second caterer while one supplies the school.
	ErrSupplyActive = &Fault{Code: "meal_supply_active", Detail: "another caterer supplies the school"}
	// ErrNotSupplied refuses a caterer the school is not supplied by.
	ErrNotSupplied = &Fault{Code: "meal_not_supplied", Detail: "the caterer does not supply the school"}
	// ErrOrderNotFound: the school has no such order.
	ErrOrderNotFound = &Fault{Code: "meal_order_not_found", Detail: "meal order not found"}
)

// Supply is a school's supply by its caterer (ADR 0074).
type Supply struct {
	CatererID       int64
	DeliveryAddress string
	StartedAt       time.Time
}

// StartSupply starts the school's supply by a caterer in the caller's unit of
// work. Starting the running supply again changes nothing.
func (m *Module) StartSupply(ctx context.Context, catererID int64, deliveryAddress string, accountID int64) error {
	if catererID <= 0 {
		return ErrInvalidParticipation
	}
	return m.engine.StartSupply(ctx, catererID, strings.TrimSpace(deliveryAddress), accountID)
}

// EndSupply ends the school's supply and reports whether one ran.
func (m *Module) EndSupply(ctx context.Context, accountID int64) (bool, error) {
	return m.engine.EndSupply(ctx, accountID)
}

// CurrentSupply reads the school's running supply.
func (m *Module) CurrentSupply(ctx context.Context) (Supply, bool, error) {
	return m.engine.CurrentSupply(ctx)
}

// CatererOrders returns the school's orders of [from, to] as its caterer
// reads them: numbers by class only when the school orders by class, and the
// children's names only while the school shows them to the caterer. A
// caterer that does not supply the school gets ErrNotSupplied.
func (m *Module) CatererOrders(ctx context.Context, catererID int64, from, to Date) (OrderWeek, error) {
	start, startErr := calendar.ParseDate(string(from))
	end, endErr := calendar.ParseDate(string(to))
	if startErr != nil || endErr != nil || end.Before(start) || start.DaysUntil(end) > maxOrderWindowDays {
		return OrderWeek{}, ErrInvalidParticipation
	}
	return m.engine.CatererOrders(ctx, catererID, start.String(), end.String())
}

// ConfirmOrder records that the caterer received a fixed order; confirming
// twice keeps the first.
func (m *Module) ConfirmOrder(ctx context.Context, catererID, orderID, accountID int64) error {
	if orderID <= 0 {
		return ErrOrderNotFound
	}
	return m.engine.ConfirmOrder(ctx, catererID, orderID, accountID)
}
