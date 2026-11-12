package application

import (
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

// Engine is the Identity & Access owner value compose fills. It stays an
// alias: the command-effect inventory keys its reviewed bindings, the two
// cross-owner coordination exceptions of AcceptSchoolInvitation included, by
// the type's name (application.engine). The base comparison counts a renamed
// exception as a new one that needs an approval ADR
// (internal/architecture/command_effect_base.go, effectBaseFailures).
type Engine = engine

// engine is the Identity & Access owner value. Each
// embedded service serves its capability itself and reports it unavailable
// when the owner was composed without its dependencies. The records, the
// second factor and the passkey ceremonies are embedded through their
// interfaces because their own steps share names with each other.
type engine struct {
	CalendarFeeds
	*AccountRetention
	*RFIDCards
	*AccountRoleQueries
	*GuardianSchools
	*AccountProfiles
	*Service
	ports.OperatorMFARecords
	ports.OperatorTokens
	ports.OperatorPasskeyRecords
	ports.AccountPasskeyRecords
	*AccountAuthentication
	*OperatorAuthentication
	*OperatorAccountAccess
	*AccountLifecycle
	*RoleAdministration
	*PasswordReset
	*SchoolInvitation
	*AccountProvisioning
	*AccountAdministration
	*OperatorProvisioning
	*SchoolInvitationMaintenance
	*SignInDataRetention
	*TraegerAccounts
	identityaccess.OperatorMFAFlows
	identityaccess.OperatorPasskeyFlows
	identityaccess.AccountPasskeyFlows
	AccountMFACapability
	// Demo is the demo-only capability; nil outside the demo environment.
	Demo *identityaccess.DemoAccess
	// CatererFlows are the caterer flows; nil without their dependencies.
	CatererFlows *identityaccess.CatererAccess
}

// DemoAccess returns the demo-only capability when the owner was composed in
// the demo environment.
func (e engine) DemoAccess() *identityaccess.DemoAccess { return e.Demo }

// Caterers returns the caterer flows (ADR 0077), nil when the owner was
// composed without them.
func (e engine) Caterers() *identityaccess.CatererAccess { return e.CatererFlows }

var (
	_ identityaccess.Owner = Engine{}
	_ ports.Records        = Engine{}
)
