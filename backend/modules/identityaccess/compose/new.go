// Package compose wires the Identity & Access module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	internalports "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = internalports.Observation

type Dependencies struct {
	RemoveErasedAvatarFile func(string) error // Nil uses rooted local avatar storage.
	DB                     *bun.DB
	Observe                func(Observation)
	// Sessions composes the account-authentication flows (#3251): tenant,
	// parent and school login, refresh, switching, logout, session validation,
	// cleanup and revocation. Compositions that only read identity facts
	// leave it nil; the flows then report ErrAccountAuthenticationUnavailable.
	Sessions *identityBindingPorts.SessionDependencies
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
	Invitations *identityBindingPorts.SchoolInvitationDependencies
	// MFA composes the account and operator second factor and, with its
	// relying-party facts, both portals' passkey ceremonies (#3331). It
	// requires Sessions; compositions without it report
	// ErrAccountMFAUnavailable and ErrOperatorMFAUnavailable.
	MFA *identityBindingPorts.MFADependencies
	// OperatorProvisioning composes the operator invitation and e-mail
	// change flows (#3332). They require Sessions and Operators;
	// compositions without it report ErrOperatorProvisioningUnavailable.
	OperatorProvisioning *OperatorProvisioningDependencies
	// Traeger composes the Träger-Büro account management (#4650): the
	// invitation into a Träger-Büro and its roles. It requires Sessions;
	// compositions without it report ErrTraegerAccountsUnavailable.
	Traeger *identityBindingPorts.TraegerAccountDependencies
	// Demo composes public demo access only in the demo environment.
	Demo *identityBindingPorts.DemoDependencies
	// TenantRuntime binds transactions; a zero value keeps the sessions' own runtime.
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
func New(dependencies Dependencies) (identityaccess.Owner, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("identity access compose: all dependencies are required")
	}
	sessions, attach := application.BindTenantRuntime(dependencies.Sessions, dependencies.TenantRuntime)
	dependencies.Sessions = sessions
	store := postgres.NewStore(dependencies.DB)
	runAdmin := newCommandTransactions(attach).RunAdmin
	service := application.New(store, store, store, newCommandTransactions(attach), tenant.FromContext, application.NewObserver(dependencies.Observe))
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
	administration, lifecycle, roles, err := newLifecycleAndRoles(service, auth, store, dependencies)
	if err != nil {
		return nil, err
	}
	resets, invitations, provisioning, err := newRegistrationFlows(auth, store, lifecycle, roles, dependencies)
	if err != nil {
		return nil, err
	}
	tokens := application.NewOperatorTokens(service, store)
	operatorAuth, accountAccess, err := newOperatorFlows(service, store, tokens, auth, dependencies.Sessions, dependencies.Operators, lifecycle, application.OperatorLoginGate(flows, dependencies.Demo))
	if err != nil {
		return nil, err
	}
	operatorPasskeys, accountPasskeys := application.NewOperatorPasskey(service, store), application.NewAccountPasskey(service, store)
	flows, err = withPasskeyFlows(flows, service, auth, operatorAuth, accountPasskeys, operatorPasskeys,
		dependencies.Sessions, dependencies.MFA)
	if err != nil {
		return nil, err
	}
	operatorProvisioning, err := newOperatorProvisioning(service, tokens, dependencies.Sessions, dependencies.Operators, dependencies.OperatorProvisioning)
	if err != nil {
		return nil, err
	}
	demoAccess, traegerAccounts, err := newPortalAccess(dependencies, auth, store, runAdmin)
	if err != nil {
		return nil, err
	}
	owner := application.NewOwner(application.OwnerParts{
		CalendarFeeds: application.NewCalendarFeeds(application.NewStaffCalendarFeeds(service, store), application.NewParentCalendarFeeds(service, store)),
		Profiles:      application.NewAccountProfiles(service, store), GuardianSchools: application.NewGuardianSchools(service, store),
		Service: service, MFA: operatorMFARecords, Tokens: tokens, RFIDCards: application.NewRFIDCards(service, store),
		Passkeys: operatorPasskeys, AccountPasskeys: accountPasskeys, AccountRoleQueries: application.NewAccountRoleQueries(service, store, postgres.NewRoleStore(store)),
		Auth: auth, OperatorAuth: operatorAuth, AccountAccess: accountAccess, Lifecycle: lifecycle, Roles: roles,
		Resets: resets, Invitations: invitations, Provisioning: provisioning, Administration: administration,
		MFAFlows: flows, OperatorProvisioning: operatorProvisioning, SignInRetention: application.NewSignInDataRetention(store, application.InvitationLogger(dependencies.Invitations), runAdmin), AccountRetention: application.NewAccountRetention(store, attach, runAdmin, dependencies.RemoveErasedAvatarFile, application.InvitationLogger(dependencies.Invitations)),
		InvitationMaintenance: application.NewSchoolInvitationMaintenance(store, application.InvitationLogger(dependencies.Invitations), newCommandTransactions(attach).RunPlatform),
		TraegerAccounts:       traegerAccounts, DemoAccess: demoAccess,
	})
	return owner, nil
}

// NewGuardianInvitationReads composes the guardian invitation lists over the
// database alone, for a root that needs them before it composes the module.
// They run on the caller's ambient transaction like the module's reads.
func NewGuardianInvitationReads(db *bun.DB) (identityaccess.GuardianInvitationReads, error) {
	if db == nil {
		return nil, errors.New("identity access compose: the guardian invitation reads need the database")
	}
	return application.NewGuardianInvitationReads(postgres.NewStore(db), newCommandTransactions(nil).RunRead), nil
}

func newCommandTransactions(attach func(context.Context) context.Context) application.CommandTransactions {
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewCommandTransactions(attach, tenant.TransactionFromContext, tenant.FromContext, tenant.WithinCurrentTenant, tenant.WithinAdmin)
}

func newLifecycleAndRoles(service *application.Service, auth *application.AccountAuthentication, store *postgres.Store, dependencies Dependencies) (*application.AccountAdministration, *application.AccountLifecycle, *application.RoleAdministration, error) {
	administration, err := newAccountAdministration(store, auth, dependencies.Sessions, dependencies.Lifecycle)
	if err != nil {
		return nil, nil, nil, err
	}
	lifecycle, roles, err := newAccountLifecycle(service, auth, store, dependencies.Sessions, dependencies.Lifecycle, administration)
	if err != nil {
		return nil, nil, nil, err
	}
	return administration, lifecycle, roles, nil
}

func newRegistrationFlows(auth *application.AccountAuthentication, store *postgres.Store, lifecycle *application.AccountLifecycle, roles *application.RoleAdministration, dependencies Dependencies) (*application.PasswordReset, *application.SchoolInvitation, *application.AccountProvisioning, error) {
	resets, err := newPasswordReset(auth, store, dependencies.Sessions, dependencies.Resets)
	if err != nil {
		return nil, nil, nil, err
	}
	invitations, err := newSchoolInvitation(store, roles, lifecycle, dependencies.Sessions, dependencies.Lifecycle, dependencies.Invitations)
	if err != nil {
		return nil, nil, nil, err
	}
	provisioning, err := newAccountProvisioning(store, lifecycle, dependencies.Sessions, dependencies.Lifecycle)
	if err != nil {
		return nil, nil, nil, err
	}
	return resets, invitations, provisioning, nil
}

// newPortalAccess composes the public demo access and the Träger-Büro
// account management (#4650) over the global administrative unit; each is
// nil without its dependencies. The account management requires Sessions:
// their password verifier proves an existing account at acceptance.
func newPortalAccess(dependencies Dependencies, auth *application.AccountAuthentication, store *postgres.Store, runAdmin func(context.Context, func(context.Context) error) error) (*identityaccess.DemoAccess, *application.TraegerAccounts, error) {
	demoAccess, err := composeDemoAccess(dependencies.DB, auth, dependencies.Demo)
	deps := dependencies.Traeger
	if err != nil || deps == nil {
		return demoAccess, nil, err
	}
	if dependencies.Sessions == nil || dependencies.Sessions.Passwords == nil {
		return nil, nil, errors.New("identity access compose: the Träger account management requires the session dependencies")
	}
	traegerAccounts, err := application.NewTraegerAccounts(application.TraegerAccountsDependencies{
		Store: store, Memberships: deps.Memberships, Delivery: deps.Delivery, Audit: deps.Audit, Passwords: deps.Passwords,
		Verifier: dependencies.Sessions.Passwords, Unit: runAdmin, Expiry: deps.Expiry, Logger: deps.Logger,
	})
	return demoAccess, traegerAccounts, err
}
