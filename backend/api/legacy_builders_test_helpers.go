package api

import (
	"context"
	"time"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// The production builders of the legacy composition that test graphs call
// through the typed root. Test packages that may not import the legacy
// composition reach them here until the slices of #2747 move them into the
// owners and the root (#4005).

// NewOrganizationTenancy composes the Organization & Tenancy owner.
func NewOrganizationTenancy(db transactionruntime.Database) (organizationtenancy.Capability, error) {
	return services.NewOrganizationTenancy(db.Pool())
}

// NewPeopleDirectory composes the People Directory owner, unobserved.
func NewPeopleDirectory(db transactionruntime.Database) (peopledirectory.Capability, error) {
	return services.NewPeopleDirectory(db.Pool())
}

// MustNewPeopleDirectory is NewPeopleDirectory for graphs that cannot return
// an error.
func MustNewPeopleDirectory(db transactionruntime.Database) peopledirectory.Capability {
	return services.MustNewPeopleDirectory(db.Pool())
}

// NewSchoolMembership composes the School Membership owner, unobserved.
func NewSchoolMembership(db transactionruntime.Database) (schoolmembership.Capability, error) {
	return services.NewSchoolMembership(db.Pool())
}

// NewDeviceFleet composes the Device Fleet owner, with the online windows the
// suite pins.
func NewDeviceFleet(db transactionruntime.Database, onlineWindows ...func(context.Context) time.Duration) (devicefleet.Capability, error) {
	return services.NewDeviceFleet(db.Pool(), onlineWindows...)
}

// NewSessionRecords composes the Student Presence session records over the
// given owners.
func NewSessionRecords(db transactionruntime.Database, owners services.SessionRecordOwners) *presenceCompose.SessionRecords {
	return services.NewSessionRecords(db.Pool(), owners)
}

// NewRetainedTimetableRepositories builds the retained Timetable adapters over
// the given dependencies.
func NewRetainedTimetableRepositories(db transactionruntime.Database, deps services.TimetableDependencies) services.TimetableRepositories {
	return services.NewTimetableRepositories(db.Pool(), deps)
}

// The enrollment flows and their sources, as the root composes them.
type (
	EnrollmentChangeRequestSources = services.EnrollmentChangeRequestSources
	EnrollmentConsentAuditor       = services.EnrollmentConsentAuditor
	EnrollmentDecisionSettings     = services.EnrollmentDecisionSettings
	EnrollmentDecisionSources      = services.EnrollmentDecisionSources
	EnrollmentDeletionOwner        = services.EnrollmentDeletionOwner
	EnrollmentDeletionSources      = services.EnrollmentDeletionSources
	EnrollmentIntakeSettings       = services.EnrollmentIntakeSettings
	EnrollmentIntakeSources        = services.EnrollmentIntakeSources
	EnrollmentRolloverSources      = services.EnrollmentRolloverSources
	SessionRecordOwners            = services.SessionRecordOwners
)

var (
	NewEnrollmentChangeRequests   = services.NewEnrollmentChangeRequests
	NewEnrollmentDecisions        = services.NewEnrollmentDecisions
	NewEnrollmentDeletionModule   = services.NewEnrollmentDeletionModule
	NewEnrollmentIntake           = services.NewEnrollmentIntake
	NewEnrollmentOfferingCapacity = services.NewEnrollmentOfferingCapacity
	NewEnrollmentRollovers        = services.NewEnrollmentRollovers
	// NewRoomSnapshotPeople reads the room snapshot's students through the
	// People Directory.
	NewRoomSnapshotPeople = services.NewRoomSnapshotPeople
	// AccountExists is the People Directory's account-reference check over
	// Identity & Access, as the root binds it.
	AccountExists = identityaccessCompose.AccountExists
	// NewWorkforceAdminCapabilities adapts the retained workforce services to
	// the public Workforce contracts, as the root does.
	NewWorkforceAdminCapabilities = services.NewWorkforceAdminCapabilities
)

// The legacy composition's values the Identity & Access behaviour suites
// name build through the typed root (#4006).
type (
	FactoryConfig              = services.FactoryConfig
	MembershipTestRepositories = services.MembershipTestRepositories
	ParentRequestReviewPolicy  = services.ParentRequestReviewPolicy
)

var (
	// NewParentRequestReviewPolicy and ReviewPermissions are the parent
	// request review decision the caller context serves.
	NewParentRequestReviewPolicy = services.NewParentRequestReviewPolicy
	ReviewPermissions            = services.ReviewPermissions
	// NewOperatorAccountDirectory is the operator account listing over
	// Identity & Access, the People Directory, School Membership and the
	// schools.
	NewOperatorAccountDirectory = services.NewOperatorAccountDirectory
	// ClassifyGuardianInvitationFailure classifies the guardian routes'
	// invitation refusals.
	ClassifyGuardianInvitationFailure = services.ClassifyGuardianInvitationFailure
)

// GuardianFailureConflict is the conflict a guardian invitation refusal is
// classified as.
const GuardianFailureConflict = services.GuardianFailureConflict

// AccountRoleGrantPolicy returns Security Runtime's role grant decision the
// root binds to the account routes.
func AccountRoleGrantPolicy() RoleGrantPolicy {
	return RoleGrantPolicy{}
}
