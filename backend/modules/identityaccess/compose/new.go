// Package compose wires the Identity & Access module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application/rotation"
	internalports "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type Observation = internalports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// Sessions composes the account-authentication flows (#3251): tenant,
	// parent and school login, refresh, switching, logout, session validation,
	// cleanup and revocation. Compositions that only read identity facts
	// leave it nil; the flows then report ErrAccountAuthenticationUnavailable.
	Sessions *SessionDependencies
	// Operators composes the operator flows (#3252): operator login, the
	// MFA-proven token issue, refresh, profile and password changes, and
	// the operator-led school access of accounts. It requires Sessions;
	// compositions without it report ErrOperatorAuthenticationUnavailable.
	Operators *OperatorDependencies
	// Lifecycle composes the account lifecycle flows (#3225): staff PIN,
	// staff preview, staff offboarding, school identity, parent accounts and
	// guardian relative access, together with the role and permission
	// administration (#3314). It requires Sessions; compositions without it
	// report ErrAccountLifecycleUnavailable and
	// ErrRoleAdministrationUnavailable.
	Lifecycle *LifecycleDependencies
	// Resets composes the password reset flows (#2722). It requires Sessions;
	// compositions without it report ErrPasswordResetUnavailable.
	Resets *PasswordResetDependencies
	// Invitations composes the school invitation flows (#2722). They require
	// Sessions and Lifecycle; compositions without it report
	// ErrSchoolInvitationUnavailable.
	Invitations *SchoolInvitationDependencies
	// MFA composes the account and operator second factor and, with its
	// relying-party facts, both portals' passkey ceremonies (#3331). It
	// requires Sessions; compositions without it report
	// ErrAccountMFAUnavailable and ErrOperatorMFAUnavailable.
	MFA *MFADependencies
	// OperatorProvisioning composes the operator invitation and e-mail
	// change flows (#3332). They require Sessions and Operators;
	// compositions without it report ErrOperatorProvisioningUnavailable.
	OperatorProvisioning *OperatorProvisioningDependencies
	// Demo composes public demo access only in the demo environment.
	Demo *DemoDependencies
	// TenantRuntime is the unit of work the module opens its session,
	// lifecycle and operator transactions under. A zero value binds nothing:
	// the sessions keep their own runtime.
	TenantRuntime tenant.UnitOfWork
}

// New composes the Identity & Access module. Guardian operations run on the
// caller's ambient transaction when one exists and otherwise open one for
// the tenant in context, so the guardian-access writes commit with the
// approval that requested them. Operator and account-session operations join
// an ambient transaction and otherwise run on the root connection: operators
// and their sessions are platform-wide rows without a tenant, and the
// account-session flows (login, refresh, switch, logout) open the
// administrative transaction their rotation and audit evidence must commit
// in before a tenant is known. Account-session statements apply the tenant
// filter the runtime scoped the caller to.
func New(dependencies Dependencies) (*identityaccess.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("identity access compose: all dependencies are required")
	}
	dependencies, attach := bindTenantRuntime(dependencies)
	store := newStore(dependencies.DB)
	service := application.New(store, store, store, newCommandTransactions(attach), tenant.FromContext, func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	})
	operatorMFARecords := application.NewOperatorMFA(service, store)
	// The second factor is composed first: every login path consults its
	// gate, and the passkey ceremonies below need the sessions it gates.
	flows, err := newMFACore(service, operatorMFARecords, postgres.NewAccountMFARecords(store), dependencies.Sessions, dependencies.MFA)
	if err != nil {
		return nil, err
	}
	auth, err := newAccountAuthentication(service, store, dependencies.Sessions, application.CapabilityGate{Capability: flows.Capability})
	if err != nil {
		return nil, err
	}
	administration, err := newAccountAdministration(store, auth, dependencies.Sessions, dependencies.Lifecycle)
	if err != nil {
		return nil, err
	}
	lifecycle, roles, err := newAccountLifecycle(service, auth, store, dependencies.Sessions, dependencies.Lifecycle, administration)
	if err != nil {
		return nil, err
	}
	resets, err := newPasswordReset(auth, store, dependencies.Sessions, dependencies.Resets)
	if err != nil {
		return nil, err
	}
	invitations, err := newSchoolInvitation(store, roles, lifecycle, dependencies.Sessions, dependencies.Lifecycle, dependencies.Invitations)
	if err != nil {
		return nil, err
	}
	provisioning, err := newAccountProvisioning(store, lifecycle, dependencies.Sessions, dependencies.Lifecycle)
	if err != nil {
		return nil, err
	}
	tokens := application.NewOperatorTokens(service, store)
	operatorAuth, accountAccess, err := newOperatorFlows(service, store, tokens, auth, dependencies.Sessions, dependencies.Operators, lifecycle, operatorLoginGate(flows, dependencies.Demo))
	if err != nil {
		return nil, err
	}
	operatorPasskeys, accountPasskeys := newPasskeyRecords(service, store)
	flows, err = withPasskeyFlows(flows, service, auth, operatorAuth, accountPasskeys, operatorPasskeys,
		dependencies.Sessions, dependencies.MFA)
	if err != nil {
		return nil, err
	}
	operatorProvisioning, err := newOperatorProvisioning(service, tokens, dependencies.Sessions, dependencies.Operators, dependencies.OperatorProvisioning)
	if err != nil {
		return nil, err
	}
	demoAccess, err := composeDemoAccess(dependencies.DB, auth, dependencies.Demo)
	if err != nil {
		return nil, err
	}
	e := application.NewEngine(application.EngineParts{
		CalendarFeeds: newCalendarFeeds(service, store),
		Profiles:      application.NewAccountProfiles(service, store), GuardianSchools: application.NewGuardianSchools(service, store),
		Service: service, MFA: operatorMFARecords, Tokens: tokens, RFIDCards: application.NewRFIDCards(service, store),
		Passkeys: operatorPasskeys, AccountPasskeys: accountPasskeys, AccountRoleQueries: application.NewAccountRoleQueries(service, store, postgres.NewRoleStore(store)),
		Auth: auth, OperatorAuth: operatorAuth, AccountAccess: accountAccess, Lifecycle: lifecycle, Roles: roles,
		Resets: resets, Invitations: invitations, Provisioning: provisioning, Administration: administration,
		MFAFlows: flows, OperatorProvisioning: operatorProvisioning, SignInRetention: application.NewSignInDataRetention(store, invitationLogger(dependencies.Invitations), newCommandTransactions(attach).RunAdmin),
		InvitationMaintenance: application.NewSchoolInvitationMaintenance(store, invitationLogger(dependencies.Invitations), newCommandTransactions(attach).RunPlatform),
		Runtime:               attach, WithRecoveryProof: rotation.WithRecoveryProof,
	})
	return identityaccess.NewDemoModule(e, demoAccess), nil
}

// NewGuardianInvitationReads composes the guardian invitation lists over the
// database alone, for a root that needs them before it composes the module.
// They run on the caller's ambient transaction like the module's reads.
func NewGuardianInvitationReads(db *bun.DB) (identityaccess.GuardianInvitationReads, error) {
	if db == nil {
		return nil, errors.New("identity access compose: the guardian invitation reads need the database")
	}
	return application.NewGuardianInvitationReads(newStore(db), newCommandTransactions(nil).RunRead), nil
}

// bindTenantRuntime is the runtime the module opens its transactions under:
// Dependencies.TenantRuntime when set, otherwise the sessions' own runtime,
// otherwise the context's. The sessions attach through it.
func bindTenantRuntime(dependencies Dependencies) (Dependencies, func(context.Context) context.Context) {
	attach := func(ctx context.Context) context.Context { return ctx }
	if dependencies.Sessions != nil && dependencies.Sessions.TenantRuntime != nil {
		attach = dependencies.Sessions.TenantRuntime
	}
	if unit := dependencies.TenantRuntime; !unit.IsZero() {
		attach = func(ctx context.Context) context.Context { return tenant.WithUnitOfWork(ctx, unit) }
	}
	if dependencies.Sessions != nil {
		sessions := *dependencies.Sessions
		sessions.TenantRuntime = attach
		dependencies.Sessions = &sessions
	}
	return dependencies, attach
}

func newPasskeyRecords(service *application.Service, store *postgres.Store) (*application.OperatorPasskey, *application.AccountPasskey) {
	return application.NewOperatorPasskey(service, store), application.NewAccountPasskey(service, store)
}

func newCommandTransactions(attach func(context.Context) context.Context) application.CommandTransactions {
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewCommandTransactions(attach, tenant.TransactionFromContext, tenant.FromContext, tenant.WithinCurrentTenant, tenant.WithinAdmin)
}
