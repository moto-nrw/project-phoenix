package mealplan

// ErrOrderNotFound: the school has no such order.
var ErrOrderNotFound = &Fault{Code: "meal_order_not_found", Detail: "meal order not found"}
