// Package compose wires Identity & Access over the tenant runtime and database.
package compose

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/application"
	internalports "github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
	identityBindingPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-ogs/project-phoenix/tenant"
)

type Observation = internalports.Observation

type Dependencies struct {
	RemoveErasedAvatarFile func(string) error // Nil uses rooted local avatar storage.
	DB                     *bun.DB
	Observe                func(Observation)
	// Sessions binds login, refresh, switching, logout and validation. Nil
	// leaves authentication unavailable and supports read-only compositions.
	Sessions *identityBindingPorts.SessionDependencies
	// Operators requires Sessions; nil leaves operator authentication unavailable.
	Operators *OperatorDependencies
	// Lifecycle requires Sessions and binds account, guardian and role commands.
	// Nil reports ErrAccountLifecycleUnavailable / ErrRoleAdministrationUnavailable.
	Lifecycle *LifecycleDependencies
	// Resets requires Sessions; nil reports ErrPasswordResetUnavailable.
	Resets *PasswordResetDependencies
	// Invitations composes the school invitation flows (#2722); they require
	// Sessions and Lifecycle, else they report ErrSchoolInvitationUnavailable.
	Invitations *identityBindingPorts.SchoolInvitationDependencies
	// MFA composes the account and operator second factor and, with its
	// relying-party facts, both portals' passkey ceremonies (#3331). It requires
	// Sessions; without it: ErrAccountMFAUnavailable, ErrOperatorMFAUnavailable.
	MFA *identityBindingPorts.MFADependencies
	// OperatorProvisioning composes the operator invitation and e-mail change
	// flows (#3332); they require Sessions and Operators, else they report
	// ErrOperatorProvisioningUnavailable.
	OperatorProvisioning *OperatorProvisioningDependencies
	// Traeger composes the Träger-Büro invitation and roles (#4650); it requires
	// Sessions, else it reports ErrTraegerAccountsUnavailable.
	Traeger *identityBindingPorts.TraegerAccountDependencies
	// Demo composes public demo access only in the demo environment.
	Demo *identityBindingPorts.DemoDependencies
	// TenantRuntime binds transactions; a zero value keeps the sessions' own runtime.
	TenantRuntime tenant.UnitOfWork
}

// New composes Identity & Access. Guardian writes join the caller's transaction
// or open one for its tenant, committing with the approval that requested them.
// Operator/session operations join an ambient transaction or use the root
// connection. Login, refresh, switch and logout open an administrative unit
// before a tenant is known; school statements apply the runtime's tenant filter.
func New(dependencies Dependencies) (identityaccess.Owner, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("identity access compose: all dependencies are required")
	}
	sessions, attach := application.BindTenantRuntime(dependencies.Sessions, dependencies.TenantRuntime)
	dependencies.Sessions = sessions
	runtime := tenantRuntime{attach: attach, runner: tenant.NewTransactionRunner()}
	store := postgres.NewStore(dependencies.DB)
	runAdmin := newCommandTransactions(attach).RunAdmin
	service := application.New(store, store, store, newCommandTransactions(attach), tenant.FromContext, application.NewObserver(dependencies.Observe))
	operatorMFARecords := application.NewOperatorMFA(service, store)
	// Compose MFA first: login and passkey ceremonies depend on its gate.
	flows, err := newMFACore(service, operatorMFARecords, postgres.NewAccountMFARecords(store), dependencies.Sessions, dependencies.MFA, runtime)
	if err != nil {
		return nil, err
	}
	auth, err := newAccountAuthentication(service, store, store, dependencies.Sessions, application.CapabilityGate{Capability: flows.Capability}, runtime)
	if err != nil {
		return nil, err
	}
	administration, lifecycle, roles, err := newLifecycleAndRoles(service, auth, store, dependencies, runtime)
	if err != nil {
		return nil, err
	}
	resets, invitations, provisioning, err := newRegistrationFlows(auth, store, lifecycle, roles, dependencies, runtime)
	if err != nil {
		return nil, err
	}
	tokens := application.NewOperatorTokens(service, store)
	operatorAuth, accountAccess, err := newOperatorFlows(service, store, tokens, auth, dependencies.Sessions, dependencies.Operators, lifecycle, application.OperatorLoginGate(flows, dependencies.Demo), runtime)
	if err != nil {
		return nil, err
	}
	operatorPasskeys, accountPasskeys := application.NewOperatorPasskey(service, store), application.NewAccountPasskey(service, store)
	flows, err = withPasskeyFlows(flows, service, auth, operatorAuth, accountPasskeys, operatorPasskeys,
		dependencies.Sessions, dependencies.MFA, runtime)
	if err != nil {
		return nil, err
	}
	operatorProvisioning, err := newOperatorProvisioning(service, tokens, dependencies.Sessions, dependencies.Operators, dependencies.OperatorProvisioning, runtime)
	if err != nil {
		return nil, err
	}
	demoAccess, traegerAccounts, caterers, err := composeBesideEngine(dependencies, auth, store, runAdmin, flows, runtime)
	if err != nil {
		return nil, err
	}
	return application.Engine{
		CalendarFeeds:   application.NewCalendarFeeds(application.NewStaffCalendarFeeds(service, store), application.NewParentCalendarFeeds(service, store)),
		AccountProfiles: application.NewAccountProfiles(service, store), GuardianSchools: application.NewGuardianSchools(service, store),
		Service: service, OperatorMFARecords: operatorMFARecords, OperatorTokens: tokens, RFIDCards: application.NewRFIDCards(service, store),
		OperatorPasskeyRecords: operatorPasskeys, AccountPasskeyRecords: accountPasskeys, AccountRoleQueries: application.NewAccountRoleQueries(service, store, postgres.NewRoleStore(store)),
		AccountAuthentication: auth, OperatorAuthentication: operatorAuth, OperatorAccountAccess: accountAccess, AccountLifecycle: lifecycle, RoleAdministration: roles,
		PasswordReset: resets, SchoolInvitation: invitations, AccountProvisioning: provisioning, AccountAdministration: administration, OperatorProvisioning: operatorProvisioning,
		OperatorMFAFlows: flows.Operator, OperatorPasskeyFlows: flows.OperatorPasskey, AccountPasskeyFlows: flows.AccountPasskey, AccountMFACapability: application.NewAccountMFACapability(flows.Capability),
		SignInDataRetention: application.NewSignInDataRetention(store, application.InvitationLogger(dependencies.Invitations), runAdmin), AccountRetention: application.NewAccountRetention(store, attach, runAdmin, dependencies.RemoveErasedAvatarFile, application.InvitationLogger(dependencies.Invitations)),
		SchoolInvitationMaintenance: application.NewSchoolInvitationMaintenance(store, application.InvitationLogger(dependencies.Invitations), newCommandTransactions(attach).RunPlatform), TraegerAccounts: traegerAccounts, Demo: demoAccess, CatererFlows: caterers,
	}, nil
}

// composeBesideEngine composes the capabilities the owner carries beside its
// services: the demo access, the Träger accounts and the caterer flows.
func composeBesideEngine(dependencies Dependencies, auth *application.AccountAuthentication, store *postgres.Store, runAdmin func(context.Context, func(context.Context) error) error, flows application.MFAFlows, runtime tenantRuntime) (*identityaccess.DemoAccess, *application.TraegerAccounts, *identityaccess.CatererAccess, error) {
	demoAccess, traegerAccounts, err := newPortalAccess(dependencies, auth, store, runAdmin)
	if err != nil {
		return nil, nil, nil, err
	}
	caterers, err := newCatererAccess(auth, store, dependencies.Sessions, dependencies.Lifecycle, flows, runtime)
	return demoAccess, traegerAccounts, caterers, err
}

// NewGuardianInvitationReads joins ambient transactions without the full owner.
func NewGuardianInvitationReads(db *bun.DB) (identityaccess.GuardianInvitationReads, error) {
	if db == nil {
		return nil, errors.New("identity access compose: the guardian invitation reads need the database")
	}
	return application.NewGuardianInvitationReads(postgres.NewStore(db), newCommandTransactions(nil).RunRead), nil
}

func NewGuardianInvitationInvalidation(db *bun.DB) (identityaccess.GuardianInvitationInvalidation, error) {
	if db == nil {
		return nil, errors.New("identity access compose: guardian invitation invalidation needs the database")
	}
	return application.NewGuardianInvitationInvalidation(postgres.NewStore(db)), nil
}

// NewSchoolMembershipLock binds the parent write guard's membership lock (#6529).
func NewSchoolMembershipLock(db *bun.DB) identityaccess.SchoolMembershipLock {
	return application.NewSchoolMembershipLock(postgres.NewStore(db), tenant.FromContext, tenant.TransactionFromContext)
}

func NewGuardianInvitationAccountLock(db *bun.DB) identityaccess.GuardianInvitationAccountLock {
	return application.NewGuardianInvitationAccountLock(postgres.NewStore(db), tenant.FromContext, tenant.TransactionFromContext)
}

func newCommandTransactions(attach func(context.Context) context.Context) application.CommandTransactions {
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewCommandTransactions(attach, tenant.TransactionFromContext, tenant.FromContext, tenant.WithinCurrentTenant, tenant.WithinAdmin)
}

func newLifecycleAndRoles(service *application.Service, auth *application.AccountAuthentication, store *postgres.Store, dependencies Dependencies, runtime tenantRuntime) (*application.AccountAdministration, *application.AccountLifecycle, *application.RoleAdministration, error) {
	administration, err := newAccountAdministration(store, auth, dependencies.Sessions, dependencies.Lifecycle, runtime)
	if err != nil {
		return nil, nil, nil, err
	}
	lifecycle, roles, err := newAccountLifecycle(service, auth, store, dependencies.Sessions, dependencies.Lifecycle, administration, runtime)
	if err != nil {
		return nil, nil, nil, err
	}
	return administration, lifecycle, roles, nil
}

func newRegistrationFlows(auth *application.AccountAuthentication, store *postgres.Store, lifecycle *application.AccountLifecycle, roles *application.RoleAdministration, dependencies Dependencies, runtime tenantRuntime) (*application.PasswordReset, *application.SchoolInvitation, *application.AccountProvisioning, error) {
	resets, err := newPasswordReset(auth, store, dependencies.Sessions, dependencies.Resets, runtime)
	if err != nil {
		return nil, nil, nil, err
	}
	invitations, err := newSchoolInvitation(store, roles, lifecycle, dependencies.Sessions, dependencies.Lifecycle, dependencies.Invitations, runtime)
	if err != nil {
		return nil, nil, nil, err
	}
	provisioning, err := newAccountProvisioning(store, lifecycle, dependencies.Sessions, dependencies.Lifecycle, runtime)
	if err != nil {
		return nil, nil, nil, err
	}
	return resets, invitations, provisioning, nil
}

// newPortalAccess composes the public demo access and the Träger-Büro
// account management (#4650) over the global administrative unit; each is
// nil without its dependencies. The account management requires Sessions:
// the login's password check proves an existing account at acceptance.
func newPortalAccess(dependencies Dependencies, auth *application.AccountAuthentication, store *postgres.Store, runAdmin func(context.Context, func(context.Context) error) error) (*identityaccess.DemoAccess, *application.TraegerAccounts, error) {
	demoAccess, err := composeDemoAccess(dependencies.DB, auth, dependencies.Demo)
	deps := dependencies.Traeger
	if err != nil || deps == nil {
		return demoAccess, nil, err
	}
	if dependencies.Sessions == nil || auth == nil {
		return nil, nil, errors.New("identity access compose: the Träger account management requires the session dependencies")
	}
	traegerAccounts, err := application.NewTraegerAccounts(application.TraegerAccountsDependencies{
		Store: store, Memberships: deps.Memberships, Delivery: deps.Delivery, Audit: deps.Audit, Passwords: deps.Passwords,
		Credentials: auth.CheckPasswordLogin, Unit: runAdmin, InAdminUnit: auth.InAdminUnit, Expiry: deps.Expiry, Logger: deps.Logger,
	})
	return demoAccess, traegerAccounts, err
}

// NewAccountBatchQuery exposes a read-sized capability without extending the
// login/command Owner contract or constructing its unrelated session flows.
func NewAccountBatchQuery(db *bun.DB) (identityaccess.AccountBatchQuery, error) {
	if db == nil {
		return nil, errors.New("identity access: account batch needs the database")
	}
	store := postgres.NewStore(db)
	return application.NewAccountBatch(store, application.NewObserver(func(Observation) {})), nil
}
