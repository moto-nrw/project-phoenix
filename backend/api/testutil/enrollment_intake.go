package testutil

import "github.com/moto-nrw/project-phoenix/services"

// Enrollment's intake and change requests (#3565), composed over the sources
// the server binds, so suites drive them without importing the composition
// root.
type (
	EnrollmentIntakeSources        = services.EnrollmentIntakeSources
	EnrollmentIntakeSettings       = services.EnrollmentIntakeSettings
	EnrollmentChangeRequestSources = services.EnrollmentChangeRequestSources
	EnrollmentReviewerPersons      = services.EnrollmentReviewerPersons
	EnrollmentCareBookingCommands  = services.EnrollmentCareBookingCommands
)

var (
	// NewEnrollmentIntake composes the intake over the sources.
	NewEnrollmentIntake = services.NewEnrollmentIntake
	// NewEnrollmentChangeRequests composes the change requests over the
	// sources.
	NewEnrollmentChangeRequests = services.NewEnrollmentChangeRequests
	// NewEnrollmentCareBookingCommands binds the intake's booking writes to
	// Care Plan.
	NewEnrollmentCareBookingCommands = services.NewEnrollmentCareBookingCommands
	// NewEnrollmentCareOfferingRecords reads Care Plan's offerings in
	// enrollment rows.
	NewEnrollmentCareOfferingRecords = services.NewEnrollmentCareOfferingRecords
	// PublicEnrollmentDecisions hands the decision flow out with the public
	// refusal values.
	PublicEnrollmentDecisions = services.PublicEnrollmentDecisions
)
