package testutil

import (
	enrollmentFixture "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

func NewEnrollmentOwner() *enrollmentFixture.Module {
	return enrollmentFixture.New()
}

// NewApprovedOfferingProjection wires the same owner projection used by the
// server while keeping composition out of individual API test packages.
func NewApprovedOfferingProjection(db *bun.DB, selections services.ApprovedSelectionTestReader) (*services.ApprovedOfferingTestProjection, error) {
	return services.NewApprovedOfferingTestProjection(db, selections)
}
