package enrollmenttest

import "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

// The ports of Enrollment's decision flow, rollover and deletions (#3564) a
// behavior test binds its doubles to.
type (
	DecisionBookings          = compose.DecisionBookings
	DecisionChildren          = compose.DecisionChildren
	DecisionGuardianAccess    = compose.DecisionGuardianAccess
	DecisionStudentEnrollment = compose.DecisionStudentEnrollment
	DecisionPhases            = compose.DecisionPhases
	DecisionSchemas           = compose.DecisionSchemas
	CareWithdrawalReconciler  = compose.CareWithdrawalReconciler
	WeeklyPickupHooks         = compose.WeeklyPickupHooks
	RolloverCatalogCloner     = compose.RolloverCatalogCloner
	RolloverDecider           = compose.RolloverDecider
	Decisions                 = compose.Decisions
	DirectoryGuardian         = compose.DirectoryGuardian
	GuardianDirectory         = compose.GuardianDirectory
	RejectedRequestCleaner    = compose.RejectedRequestCleaner
)
