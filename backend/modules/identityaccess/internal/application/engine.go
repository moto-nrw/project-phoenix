package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// OwnerParts are the application services that serve the Identity & Access
// capability. A nil member reports its flows as unavailable.
type OwnerParts struct {
	CalendarFeeds         CalendarFeeds
	RFIDCards             *RFIDCards
	AccountRoleQueries    *AccountRoleQueries
	GuardianSchools       *GuardianSchools
	Profiles              *AccountProfiles
	Service               *Service
	MFA                   *OperatorMFA
	Tokens                *OperatorTokens
	Passkeys              *OperatorPasskey
	AccountPasskeys       *AccountPasskey
	Auth                  *AccountAuthentication
	OperatorAuth          *OperatorAuthentication
	AccountAccess         *OperatorAccountAccess
	Lifecycle             *AccountLifecycle
	Roles                 *RoleAdministration
	Resets                *PasswordReset
	Invitations           *SchoolInvitation
	Provisioning          *AccountProvisioning
	Administration        *AccountAdministration
	OperatorProvisioning  *OperatorProvisioning
	InvitationMaintenance *SchoolInvitationMaintenance
	SignInRetention       *SignInDataRetention
	AccountRetention      *AccountRetention
	MFAFlows              MFAFlows
	// Runtime attaches the composed unit of work ahead of every session flow.
	Runtime func(context.Context) context.Context
	// DemoAccess is the demo-only capability; nil outside the demo
	// environment.
	DemoAccess *identityaccess.DemoAccess
}

// NewOwner serves the Identity & Access capability from its services
// (#4452). The services serve their public capabilities themselves; the
// engine keeps only the lifecycle and role operations whose transaction
// boundaries the register keys on it until #4469, and the attach they use.
func NewOwner(parts OwnerParts) identityaccess.Owner {
	return engine{
		CalendarFeeds:               parts.CalendarFeeds,
		RFIDCards:                   parts.RFIDCards,
		AccountRoleQueries:          parts.AccountRoleQueries,
		GuardianSchools:             parts.GuardianSchools,
		AccountProfiles:             parts.Profiles,
		Service:                     parts.Service,
		OperatorMFARecords:          parts.MFA,
		OperatorTokens:              parts.Tokens,
		OperatorPasskeyRecords:      parts.Passkeys,
		AccountPasskeyRecords:       parts.AccountPasskeys,
		AccountAuthentication:       parts.Auth,
		OperatorAuthentication:      parts.OperatorAuth,
		OperatorAccountAccess:       parts.AccountAccess,
		lifecycle:                   parts.Lifecycle,
		AccountLifecycle:            parts.Lifecycle,
		roles:                       parts.Roles,
		RoleAdministration:          parts.Roles,
		PasswordReset:               parts.Resets,
		SchoolInvitation:            parts.Invitations,
		AccountProvisioning:         parts.Provisioning,
		AccountAdministration:       parts.Administration,
		OperatorProvisioning:        parts.OperatorProvisioning,
		SchoolInvitationMaintenance: parts.InvitationMaintenance,
		SignInDataRetention:         parts.SignInRetention,
		AccountRetention:            parts.AccountRetention,
		OperatorMFAFlows:            parts.MFAFlows.Operator,
		OperatorPasskeyFlows:        parts.MFAFlows.OperatorPasskey,
		AccountPasskeyFlows:         parts.MFAFlows.AccountPasskey,
		accountMFACapability:        newAccountMFACapability(parts.MFAFlows.Capability),
		runtime:                     parts.Runtime,
		demoAccess:                  parts.DemoAccess,
	}
}

// engine is the Identity & Access owner value. Each embedded service serves
// its public capability itself and reports it unavailable when the owner was
// composed without its dependencies. The records, the second factor and the
// passkey ceremonies are embedded through their public capabilities because
// their own steps share names with each other. The engine's own methods are
// only the lifecycle and role operations whose transaction boundaries the
// register keys on it (#4469); lifecycle and roles reach the same services
// for them.
type engine struct {
	CalendarFeeds
	*AccountRetention
	*RFIDCards
	*AccountRoleQueries
	*GuardianSchools
	*AccountProfiles
	*Service
	identityaccess.OperatorMFARecords
	identityaccess.OperatorTokens
	identityaccess.OperatorPasskeyRecords
	identityaccess.AccountPasskeyRecords
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
	identityaccess.OperatorMFAFlows
	identityaccess.OperatorPasskeyFlows
	identityaccess.AccountPasskeyFlows
	accountMFACapability
	lifecycle *AccountLifecycle
	roles     *RoleAdministration
	// runtime attaches the composed unit of work ahead of every engine
	// operation.
	runtime func(context.Context) context.Context
	// demoAccess is the demo-only capability; nil outside the demo
	// environment.
	demoAccess *identityaccess.DemoAccess
}

// attach hands every session flow the unit of work it was composed with so
// the reads outside a transaction resolve the tenant runtime too.
func (e engine) attach(ctx context.Context) context.Context {
	if e.runtime == nil {
		return ctx
	}
	return e.runtime(ctx)
}

// DemoAccess returns the demo-only capability when the owner was composed in
// the demo environment.
func (e engine) DemoAccess() *identityaccess.DemoAccess { return e.demoAccess }
