package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BillingChild is a child of a released Mensa month (#4686).
type BillingChild struct {
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
	RegisteredDays int
	MealDays       int
}

// StoredBillingMonth is a released Mensa month as the store keeps it.
type StoredBillingMonth struct {
	Month      calendar.Date
	ReleasedAt time.Time
	ChildCount int
	MealCount  int
	Children   []BillingChild
}
