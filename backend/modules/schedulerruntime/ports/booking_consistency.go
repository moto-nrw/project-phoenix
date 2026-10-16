package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookingConsistencyReport counts the booking drift of one school on one day.
// TotalFindings is the number of inconsistent rows or child-days; a child in
// an optional-offering phase is reported for review but is not counted.
type BookingConsistencyReport struct {
	TenantID                        int64
	AuditDate                       calendar.Date
	PickupProjectionMissingDays     int
	ApprovedWithoutRequiredOffering int
	ApprovedWithoutOptionalOffering int
	TotalFindings                   int
}

// BookingConsistencyAudit evaluates the booking-derived planning data of the
// school in ctx on day. The root binds the Audit Platform's evaluation.
type BookingConsistencyAudit func(ctx context.Context, day calendar.Date) (*BookingConsistencyReport, error)
