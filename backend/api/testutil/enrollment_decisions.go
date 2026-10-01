package testutil

import "github.com/moto-nrw/project-phoenix/services"

// Enrollment's decision flow, rollover, admin deletion, retention cleanup and
// capacity gate (#3564), composed over the sources the server binds, so
// suites drive them without importing the composition root.
type (
	EnrollmentDecisionSources  = services.EnrollmentDecisionSources
	EnrollmentDecisionSettings = services.EnrollmentDecisionSettings
	EnrollmentConsentAuditor   = services.EnrollmentConsentAuditor
	EnrollmentRolloverSources  = services.EnrollmentRolloverSources
	EnrollmentRolloverSettings = services.EnrollmentRolloverSettings
	EnrollmentDeletionSources  = services.EnrollmentDeletionSources
	EnrollmentDeletionModule   = services.EnrollmentDeletionModule
	EnrollmentDeletionOwner    = services.EnrollmentDeletionOwner
)

var (
	// NewEnrollmentDecisions composes the decision flow over the sources.
	NewEnrollmentDecisions = services.NewEnrollmentDecisions
	// NewEnrollmentRollovers composes the rollover over the sources.
	NewEnrollmentRollovers = services.NewEnrollmentRollovers
	// NewEnrollmentDeletionModule composes the admin deletion and the
	// retention cleanup over one impact preview.
	NewEnrollmentDeletionModule = services.NewEnrollmentDeletionModule
	// NewEnrollmentOfferingCapacity composes the capacity gate the
	// submissions share with the restore.
	NewEnrollmentOfferingCapacity = services.NewEnrollmentOfferingCapacity
)
