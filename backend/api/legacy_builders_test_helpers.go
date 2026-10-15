package api

import (
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// The production builders of the legacy composition that test graphs call
// through the typed root. Test packages that may not import the legacy
// composition reach them here until the slices of #2747 move them into the
// owners and the root (#4005).

// NewSchoolMembership composes the School Membership owner, unobserved.
func NewSchoolMembership(db transactionruntime.Database) (schoolmembership.Capability, error) {
	return services.NewSchoolMembership(db.Pool())
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
)

var (
	// NewRoomSnapshotPeople reads the room snapshot's students through the
	// People Directory.
	NewRoomSnapshotPeople = peopleCompose.NewRoomSnapshotPeople
	// AccountExists is the People Directory's account-reference check over
	// Identity & Access, as the root binds it.
	AccountExists = identityaccessCompose.AccountExists
)

// The legacy composition's values the Identity & Access behaviour suites
// name build through the typed root (#4006).
type FactoryConfig = services.FactoryConfig

var (
	// NewOperatorAccountDirectory is the operator account listing over
	// Identity & Access, the People Directory, School Membership and the
	// schools.
	NewOperatorAccountDirectory = services.NewOperatorAccountDirectory
)

// AccountRoleGrantPolicy returns Security Runtime's role grant decision the
// root binds to the account routes.
func AccountRoleGrantPolicy() RoleGrantPolicy {
	return RoleGrantPolicy{}
}
