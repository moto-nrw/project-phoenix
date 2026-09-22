package organizationtenancy

import (
	"context"
	"errors"
	"time"
)

// BillingReport is the operator billing report (#2791). Billing counts the
// actively managed children and the active terminals of every school on one
// key date per month. The figures of a month are captured once, on the key
// date, and never change afterwards: the live tables cannot answer for a
// past date, because resuming care rewrites the enrollment dates, a hard
// delete removes the student row and devices keep no status history.
//
// What counts is decided by the owners: School Membership counts a live
// membership in status active, Device Fleet a live, non-virtual device in
// status active. Every call runs in the administrative transaction.
type BillingReport interface {
	// BillingKeyDay reads the key day that applies to every school.
	BillingKeyDay(ctx context.Context) (BillingKeyDay, error)
	// SetBillingKeyDay changes the key day. It applies from the next capture
	// on; a month already captured keeps its key date.
	SetBillingKeyDay(ctx context.Context, keyDay int, operatorID int64) (BillingKeyDay, error)
	// ListBillingKeyDateCounts returns every captured row, newest month
	// first.
	ListBillingKeyDateCounts(ctx context.Context) ([]BillingKeyDateCount, error)
	// RecordDueBillingKeyDates captures the current month's figures when its
	// key date is due at now and a school has no row yet. It is idempotent
	// and returns the number of rows written.
	RecordDueBillingKeyDates(ctx context.Context, now time.Time) (int, error)
}

// ErrInvalidBillingKeyDay rejects a key day outside 1..28. The key day stops
// at 28 so every month has it.
var ErrInvalidBillingKeyDay = errors.New("billing key day must be between 1 and 28")

// BillingKeyDay is the key day and the next key date (YYYY-MM-DD) it
// produces.
type BillingKeyDay struct {
	Day                 int
	NextKeyDate         string
	UpdatedAt           time.Time
	UpdatedByOperatorID *int64
}

// BillingKeyDateCount is one school's captured figures of one month. The
// names are those the school and its organisation had on capture, so a
// later rename does not change an issued invoice. Period (the month's first
// day) and KeyDate are calendar days as YYYY-MM-DD. RecordedAt is on the
// Berlin clock; a day later than KeyDate means the capture ran late.
type BillingKeyDateCount struct {
	SchoolID         int64
	SchoolName       string
	OrganizationName string
	Period           string
	KeyDate          string
	ActiveStudents   int
	ActiveTerminals  int
	RecordedAt       time.Time
}
