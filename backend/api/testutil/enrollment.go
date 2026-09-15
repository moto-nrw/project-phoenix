package testutil

import (
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

func NewEnrollmentOwner() services.EnrollmentBookingFixture {
	return services.NewEnrollmentBookingFixture()
}

// NewApprovedOfferingProjection wires the same owner projection used by the
// server while keeping composition out of individual API test packages.
func NewApprovedOfferingProjection(db *bun.DB, selections services.ApprovedSelectionTestReader) (*services.ApprovedOfferingTestProjection, error) {
	return services.NewApprovedOfferingTestProjection(db, selections)
}
