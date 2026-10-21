package mealplan

import "time"

var (
	// ErrSupplyActive refuses a second caterer while one supplies the school.
	ErrSupplyActive = &Fault{Code: "meal_supply_active", Detail: "another caterer supplies the school"}
	// ErrNotSupplied refuses a caterer the school is not supplied by.
	ErrNotSupplied = &Fault{Code: "meal_not_supplied", Detail: "the caterer does not supply the school"}
	// ErrOrderNotFound: the school has no such order.
	ErrOrderNotFound = &Fault{Code: "meal_order_not_found", Detail: "meal order not found"}
)

// Supply is a school's supply by its caterer (ADR 0077).
type Supply struct {
	CatererID       int64
	DeliveryAddress string
	StartedAt       time.Time
}
