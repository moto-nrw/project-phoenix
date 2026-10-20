package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// EngineParts are the application services the engine serves the Identity &
// Access capability over. A nil member reports its flows as unavailable.
type EngineParts struct {
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
}

// NewEngine serves the Identity & Access capability over parts (#4084).
func NewEngine(parts EngineParts) identityaccess.Engine {
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
	}
}

type engine struct {
	CalendarFeeds
	// AccountRetention is always composed like signInRetention (#4043).
	*AccountRetention
	*RFIDCards
	*AccountRoleQueries
	*GuardianSchools
	*AccountProfiles
	*Service
	// The records, the second factor and the ceremonies are embedded through
	// their public capabilities: their own steps share names with each other.
	identityaccess.OperatorMFARecords
	identityaccess.OperatorTokens
	identityaccess.OperatorPasskeyRecords
	identityaccess.AccountPasskeyRecords
	// AccountAuthentication serves its public capabilities itself and reports
	// them unavailable when the module was composed without session
	// dependencies.
	*AccountAuthentication
	// The operator flows serve their public capabilities themselves and
	// report them unavailable when the module was composed without operator
	// dependencies.
	*OperatorAuthentication
	*OperatorAccountAccess
	// AccountLifecycle serves its public capability itself and reports it
	// unavailable when the module was composed without lifecycle
	// dependencies. lifecycle reaches the same flows for the engine
	// operations the transaction register keys (#4469).
	*AccountLifecycle
	lifecycle *AccountLifecycle
	// roles is nil when the module was composed without lifecycle
	// dependencies.
	roles *RoleAdministration
	// PasswordReset serves its public capability itself and reports it
	// unavailable when the module was composed without password reset
	// dependencies.
	*PasswordReset
	// SchoolInvitation serves its public capability itself and reports it
	// unavailable when the module was composed without the school invitation
	// dependencies.
	*SchoolInvitation
	// AccountProvisioning serves its public capability itself and reports
	// it unavailable when the module was composed without lifecycle
	// dependencies.
	*AccountProvisioning
	// AccountAdministration serves its public capability itself and reports
	// it unavailable when the module was composed without lifecycle
	// dependencies.
	*AccountAdministration
	// OperatorProvisioning serves its public capability itself and reports
	// it unavailable when the module was composed without the operator
	// provisioning dependencies.
	*OperatorProvisioning
	// SchoolInvitationMaintenance is always composed: spending a deleted
	// school's invitations and deleting expired ones need no flow
	// dependencies.
	*SchoolInvitationMaintenance
	// SignInDataRetention is always composed: the fixed retention of
	// short-lived sign-in data needs no flow dependencies (#3954).
	*SignInDataRetention
	// The second factor and the passkey ceremonies serve their public
	// capabilities themselves and report them unavailable when the module
	// was composed without them.
	identityaccess.OperatorMFAFlows
	identityaccess.OperatorPasskeyFlows
	identityaccess.AccountPasskeyFlows
	// accountMFACapability serves the account second factor.
	accountMFACapability
	// runtime attaches the composed unit of work ahead of every session flow.
	runtime func(context.Context) context.Context
}

func MapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAccountNotFound):
		return identityaccess.ErrAccountNotFound
	case errors.Is(err, domain.ErrRoleNotFound):
		return identityaccess.ErrRoleNotFound
	case errors.Is(err, domain.ErrTenantRequired):
		return identityaccess.ErrTenantRequired
	case errors.Is(err, domain.ErrGuardianRoleMissing):
		return identityaccess.ErrGuardianRoleMissing
	case errors.Is(err, domain.ErrGuardianStudentAccessTenantMismatch):
		return fmt.Errorf("%w: %w", identityaccess.ErrGuardianStudentAccessTenantMismatch, err)
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return identityaccess.ErrOperatorNotFound
	case errors.Is(err, identityaccess.ErrOperatorSessionNotFound):
		return identityaccess.ErrOperatorSessionNotFound
	case errors.Is(err, identityaccess.ErrOperatorSessionRotated):
		return identityaccess.ErrOperatorSessionRotated
	case errors.Is(err, identityaccess.ErrAccountSessionNotFound):
		return identityaccess.ErrAccountSessionNotFound
	case errors.Is(err, identityaccess.ErrAccountSessionRotated):
		return identityaccess.ErrAccountSessionRotated
	case errors.Is(err, identityaccess.ErrOperatorMFACredentialNotFound):
		return identityaccess.ErrOperatorMFACredentialNotFound
	case errors.Is(err, identityaccess.ErrOperatorMFAChallengeNotFound):
		return identityaccess.ErrOperatorMFAChallengeNotFound
	case errors.Is(err, identityaccess.ErrOperatorMFAChallengeStateChanged):
		return identityaccess.ErrOperatorMFAChallengeStateChanged
	case errors.Is(err, identityaccess.ErrOperatorTrustedDeviceNotFound):
		return identityaccess.ErrOperatorTrustedDeviceNotFound
	case errors.Is(err, identityaccess.ErrOperatorInvitationNotFound):
		return identityaccess.ErrOperatorInvitationNotFound
	case errors.Is(err, identityaccess.ErrOperatorEmailChangeNotFound):
		return identityaccess.ErrOperatorEmailChangeNotFound
	case errors.Is(err, identityaccess.ErrOperatorPasskeyNotFound):
		return identityaccess.ErrOperatorPasskeyNotFound
	case errors.Is(err, identityaccess.ErrOperatorPasskeySessionNotFound):
		return identityaccess.ErrOperatorPasskeySessionNotFound
	case errors.Is(err, identityaccess.ErrAccountPasskeyNotFound):
		return identityaccess.ErrAccountPasskeyNotFound
	case errors.Is(err, identityaccess.ErrAccountPasskeySessionNotFound):
		return identityaccess.ErrAccountPasskeySessionNotFound
	default:
		return err
	}
}

// attach hands every session flow the unit of work it was composed with so
// the reads outside a transaction resolve the tenant runtime too.
func (e engine) attach(ctx context.Context) context.Context {
	if e.runtime == nil {
		return ctx
	}
	return e.runtime(ctx)
}
