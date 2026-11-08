package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BillingMonthStore keeps the released months of the school in the
// caller's school transaction and answers the Träger's read in its Träger
// transaction.
type BillingMonthStore interface {
	// ReleasedMonths reads the latest release of each month of [from, to]
	// without children.
	ReleasedMonths(ctx context.Context, from, to calendar.Date) ([]domain.StoredBillingMonth, error)
	// ReleasedMonth reads the latest release of a month with its children
	// and every version; false when the month is not released.
	ReleasedMonth(ctx context.Context, month calendar.Date) (domain.StoredBillingMonth, bool, error)
	// SchoolOrganization names the Träger of the school; false without one.
	SchoolOrganization(ctx context.Context) (int64, bool, error)
	// InsertRelease stores a release as version month.Version with its
	// reason; a version another release took is
	// mealplan.ErrBillingMonthReleased for the first, else
	// mealplan.ErrBillingMonthChanged.
	InsertRelease(ctx context.Context, organizationID, accountID int64, month domain.StoredBillingMonth) (time.Time, error)
	// TraegerMonths reads the released months up to until of the Träger in
	// its Träger transaction.
	TraegerMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]domain.ReleasedMealRow, error)
}
