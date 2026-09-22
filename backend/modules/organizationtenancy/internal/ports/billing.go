package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// BillingStore keeps the billing key day and the captured counts (#2791).
// Every call runs on the caller's administrative transaction.
type BillingStore interface {
	// UseRepeatableReadSnapshot makes the capture's school and owner reads see
	// one database snapshot before any of those reads begin.
	UseRepeatableReadSnapshot(context.Context) error
	BillingSettings(context.Context) (domain.BillingSettings, error)
	UpdateBillingKeyDay(ctx context.Context, keyDay int, operatorID int64) (domain.BillingSettings, error)
	// SchoolsMissingBillingPeriod lists the non-deleted schools that existed
	// on keyDate and have no captured row for its month yet.
	SchoolsMissingBillingPeriod(ctx context.Context, keyDate string) ([]domain.BillingSchool, error)
	// InsertBillingKeyDateCounts writes the rows and skips a school whose
	// month is already captured. It returns the number of rows written.
	InsertBillingKeyDateCounts(ctx context.Context, counts []domain.BillingKeyDateCount) (int, error)
	ListBillingKeyDateCounts(context.Context) ([]domain.BillingKeyDateCount, error)
}

// BillingCounts are the live figures of every school, each answered by its
// owner: School Membership for active students, Device Fleet for active
// terminals. A school missing from a map has none.
type BillingCounts interface {
	CountActiveStudentsByTenant(context.Context) (map[int64]int, error)
	CountActiveTerminalsByTenant(context.Context) (map[int64]int, error)
}

// BillingClock supplies the current instant.
type BillingClock func() time.Time
