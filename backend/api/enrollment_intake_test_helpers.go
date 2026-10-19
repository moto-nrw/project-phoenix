//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
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
	return owners.EnrollmentCareBookingsBinding{Owner: owner}
}
