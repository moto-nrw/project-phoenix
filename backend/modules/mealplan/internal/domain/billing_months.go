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

// ReleasedMealChild is a child of a release as the Träger reads it.
type ReleasedMealChild struct {
	StudentID int64
	FirstName string
	LastName  string
	MealDays  int
}

// ReleasedMealRow is one row of the Träger's read: a released month of a
// school and one of its children, nil for a release without children.
type ReleasedMealRow struct {
	SchoolID   int64
	Month      calendar.Date
	ReleasedAt time.Time
	Child      *ReleasedMealChild
}
