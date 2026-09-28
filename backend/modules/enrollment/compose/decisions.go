package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
)

// Ports the composition root binds for Enrollment's decision flow, restore,
// rollover and deletions (#3564).
type (
	DecisionDependencies      = application.DecisionDependencies
	DecisionRequests          = application.DecisionRequests
	DecisionChildren          = application.DecisionChildren
	DecisionGuardians         = application.DecisionGuardians
	DecisionLateInvites       = application.DecisionLateInvites
	DecisionPhases            = application.DecisionPhases
	DecisionSchemas           = application.DecisionSchemas
	CareOfferingCatalog       = application.CareOfferingCatalog
	DecisionBookings          = application.DecisionBookings
	DecisionGuardianAccess    = application.DecisionGuardianAccess
	DecisionStudentEnrollment = application.DecisionStudentEnrollment
	DecisionSettings          = application.DecisionSettings
	OfferingAdjustmentTrail   = application.OfferingAdjustmentTrail
	RestorationAudit          = application.RestorationAudit
	Restoration               = application.Restoration
	PayerAudit                = application.PayerAudit
	PayerChange               = application.PayerChange
	StudentAudit              = application.StudentAudit
	StudentConsents           = application.StudentConsents
	CareWithdrawalReconciler  = application.CareWithdrawalReconciler
	StudentPlanBroadcasts     = application.StudentPlanBroadcasts
	WeeklyPickupHooks         = application.WeeklyPickupHooks
	WeeklyPickupSchedules     = application.WeeklyPickupSchedules
	WeeklyArrivalSchedules    = application.WeeklyArrivalSchedules
	DepartureCompanions       = application.DepartureCompanions
	CompanionEdge             = application.CompanionEdge
	PeopleDirectory           = application.PeopleDirectory
	PeopleRules               = application.PeopleRules
	Persons                   = application.Persons
	ReviewerStaff             = application.ReviewerStaff
	StudentRecords            = application.StudentRecords
	GuardianProfiles          = application.GuardianProfiles
	StudentGuardians          = application.StudentGuardians
	GuardianPhones            = application.GuardianPhones
	Student                   = application.Student
	Person                    = application.Person
	GuardianProfile           = application.GuardianProfile
	StudentGuardian           = application.StudentGuardian
	GuardianPhone             = application.GuardianPhone
	GuardianRole              = application.GuardianRole
	DeparturePlanSnapshot     = application.DeparturePlanSnapshot

	RolloverDependencies  = application.RolloverDependencies
	RolloverPhases        = application.RolloverPhases
	RolloverRequests      = application.RolloverRequests
	RolloverChildren      = application.RolloverChildren
	RolloverCatalogCloner = application.RolloverCatalogCloner
	PhaseEligibilityGuard = application.PhaseEligibilityGuard
	RolloverDecider       = application.RolloverDecider

	DeletionQueries             = application.DeletionQueries
	DirectoryGuardian           = application.DirectoryGuardian
	GuardianDirectory           = application.GuardianDirectory
	DeletionPreviewDependencies = application.DeletionPreviewDependencies
	DeletionPreview             = application.DeletionPreview
	DeletionDependencies        = application.DeletionDependencies
	DeletionRequests            = application.DeletionRequests
	DeletionChildren            = application.DeletionChildren
	EnrollmentDeletionDelivery  = application.EnrollmentDeletionDelivery
	EnrollmentDeletionAudit     = application.EnrollmentDeletionAudit
	EnrollmentDeletionEvent     = application.EnrollmentDeletionEvent
	DeletionActor               = application.DeletionActor
	DeletionScope               = application.DeletionScope

	RejectedCleanupDependencies = application.RejectedCleanupDependencies
	RejectedRequestCleaner      = application.RejectedRequestCleaner
	RejectedChildren            = application.RejectedChildren
	UsedLateInviteCleaner       = application.UsedLateInviteCleaner
	RetentionSettings           = application.RetentionSettings

	CapacityOfferings = application.CapacityOfferings
	CapacityPeaks     = application.CapacityPeaks
)

// Role presets, deletion actors and scopes the root binding maps onto the
// stored values.
const (
	GuardianRolePrimary    = application.GuardianRolePrimary
	GuardianRoleEmergency  = application.GuardianRoleEmergency
	GuardianRolePickupOnly = application.GuardianRolePickupOnly
	GuardianRoleCustom     = application.GuardianRoleCustom

	DeletionActorAdmin   = application.DeletionActorAdmin
	DeletionActorSystem  = application.DeletionActorSystem
	DeletionScopeRequest = application.DeletionScopeRequest
	DeletionScopeChild   = application.DeletionScopeChild
)

// Decisions is Enrollment's decision flow: the public Decisions and
// ApprovedChildChanges capabilities in one value.
type Decisions = application.Decisions

// NewDecisions composes the decision flow in the ambient tenant transaction.
func NewDecisions(deps DecisionDependencies) *Decisions {
	deps.Runtime = tenantRuntime()
	return application.NewDecisions(deps)
}

// NewRollovers composes the rollover in the ambient tenant transaction.
func NewRollovers(deps RolloverDependencies) enrollment.Rollovers {
	deps.Runtime = tenantRuntime()
	return application.NewRollovers(deps)
}

// NewDeletionPreview composes the impact preview of a deletion for the
// tenant in context.
func NewDeletionPreview(deps DeletionPreviewDependencies) *DeletionPreview {
	deps.Runtime = tenantRuntime()
	return application.NewDeletionPreview(deps)
}

// NewDeletions composes the admin deletion of requests and children.
func NewDeletions(deps DeletionDependencies) enrollment.EnrollmentDeletions {
	deps.Runtime = tenantRuntime()
	return application.NewDeletions(deps)
}

// NewRejectedCleanup composes the retention worker of rejected enrollments.
func NewRejectedCleanup(deps RejectedCleanupDependencies) enrollment.RejectedEnrollmentCleaner {
	deps.Runtime = tenantRuntime()
	return application.NewRejectedCleanup(deps)
}

// NewOfferingCapacity composes the capacity gate a submission shares with
// the restore. A nil waitlistEnabled fails a waitlist-mode phase with a
// configuration error.
func NewOfferingCapacity(offerings CapacityOfferings, peaks CapacityPeaks, waitlistEnabled func(context.Context) (bool, error)) enrollment.OfferingCapacity {
	return application.NewOfferingCapacity(offerings, peaks, waitlistEnabled)
}
