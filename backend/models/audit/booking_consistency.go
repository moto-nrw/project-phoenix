package audit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// BookingConsistencyReport counts actionable booking configuration drift for
// one tenant. Counts contain no person names or other direct identifiers.
type BookingConsistencyReport struct {
	TenantID                        int64         `bun:"tenant_id"`
	AuditDate                       timezone.Date `bun:"audit_date"`
	PickupProjectionMissingDays     int           `bun:"pickup_projection_missing_days"`
	ApprovedWithoutRequiredOffering int           `bun:"approved_without_required_offering"`
	ApprovedWithoutOptionalOffering int           `bun:"approved_without_optional_offering"`
}

// TotalFindings returns the number of inconsistent rows or child-days. An
// approved child in an optional-offering phase is reported for review but is
// not itself inconsistent (#2491).
func (r BookingConsistencyReport) TotalFindings() int {
	return r.PickupProjectionMissingDays +
		r.ApprovedWithoutRequiredOffering
}

// BookingConsistencyRepository evaluates booking-derived planning data for
// the tenant in ctx on auditDate.
type BookingConsistencyRepository interface {
	Audit(ctx context.Context, auditDate timezone.Date) (*BookingConsistencyReport, error)
}
