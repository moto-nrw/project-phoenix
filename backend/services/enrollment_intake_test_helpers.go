package services

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

// Test bindings of Enrollment's intake and change requests (#3565): the
// same adapters the root binds, for suites that compose the flows over the
// test database.

// EnrollmentCareBookingCommands records and replaces Care Plan bookings for
// the intake and the change requests.
type EnrollmentCareBookingCommands interface {
	enrollmentOwner.CareBookingCommands
	enrollmentCompose.CareBookingChanges
}

// NewEnrollmentCareBookingCommands binds the intake's booking writes to Care
// Plan's offering bookings.
func NewEnrollmentCareBookingCommands(owner careplan.OfferingBookingCommands) EnrollmentCareBookingCommands {
	return enrollmentCareBookingCommands{owner: owner}
}

// NewEnrollmentCareOfferingRecords reads Care Plan's offerings in enrollment
// rows.
func NewEnrollmentCareOfferingRecords(carePlan enrollmentCompose.CarePlanOfferings) *enrollmentCompose.CareOfferingRecords {
	return enrollmentCompose.NewCareOfferingRecords(carePlan)
}

// PublicEnrollmentDecisions hands the decision flow out with the public
// refusal values, as the root hands it to the routes.
func PublicEnrollmentDecisions(decisions *enrollmentCompose.Decisions) enrollmentOwner.Decisions {
	return enrollmentCompose.PublicDecisions(decisions)
}
