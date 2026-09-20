// Package compose wires the Identity & Access module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

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
	// Every flow attaches the unit of work through the same reference, so
	// a root that only has its runtime after the module is composed binds
	// it once and every flow composed above sees it (#3364). The caller's
	// own attachment stays the fallback until then.
	runtime := tenant.NewRuntimeRef(nil)
	if dependencies.Sessions != nil {
		sessions := *dependencies.Sessions
		runtime = tenant.NewRuntimeRef(sessions.TenantRuntime)
		sessions.TenantRuntime = runtime.Attach
		dependencies.Sessions = &sessions
	}
	scope := func(ctx context.Context) postgres.TenantScope {
		return postgres.TenantScope{TenantID: tenant.FromContext(ctx), AdminTransaction: tenant.IsAdminTx(ctx)}
	}
	store := postgres.New(requestDatabase(dependencies.DB), scope)
	service := application.New(store, store, store, transaction{}, tenant.FromContext, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	operatorMFARecords := application.NewOperatorMFA(service, store)
	// The second factor is composed first: every login path consults its
	// gate, and the passkey ceremonies below need the sessions it gates.
	flows, err := newMFACore(service, operatorMFARecords, dependencies.Sessions, dependencies.MFA)
	if err != nil {
		return nil, err
	}
	auth, err := newAccountAuthentication(service, store, dependencies.Sessions, capabilityGate{flows.capability})
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
	operatorAuth, accountAccess, err := newOperatorFlows(service, store, tokens, auth, dependencies.Sessions, dependencies.Operators, lifecycle, operatorMFAGate{flows.operator})
	if err != nil {
		return nil, err
	}
	operatorPasskeys := application.NewOperatorPasskey(service, store)
	accountPasskeys := application.NewAccountPasskey(service, store)
	flows, err = withPasskeyFlows(flows, service, auth, operatorAuth, accountPasskeys, operatorPasskeys,
		dependencies.Sessions, dependencies.MFA)
	if err != nil {
		return nil, err
	}
	operatorProvisioning, err := newOperatorProvisioning(service, tokens, dependencies.Sessions, dependencies.Operators, dependencies.OperatorProvisioning)
	if err != nil {
		return nil, err
	}
	e := engine{
		service: service, mfa: operatorMFARecords, tokens: tokens,
		passkeys: operatorPasskeys, accountPasskeys: accountPasskeys,
		auth: auth, operatorAuth: operatorAuth, accountAccess: accountAccess, lifecycle: lifecycle, roles: roles,
		resets: resets, invitations: invitations, provisioning: provisioning, administration: administration,
		mfaFlows: flows, operatorProvisioning: operatorProvisioning,
		invitationMaintenance: application.NewSchoolInvitationMaintenance(store, invitationLogger(dependencies.Invitations)),
	}
	e.runtime = runtime.Attach
	return identityaccess.NewModule(e, runtime), nil
}

// requestDatabase resolves the caller's transaction, or the root connection
// without one.
func requestDatabase(root *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return root, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return *tx, nil
			}
			return root, nil
		default:
			return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", transaction)
		}
	}
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return fmt.Errorf("%w: %w", identityaccess.ErrTenantRequired, err)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}

// RunPlatform never opens a transaction: the operator flows open their own
// administrative transaction where several statements must commit together
// and run single statements on the root connection otherwise.
func (transaction) RunPlatform(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

type engine struct {
	service         *application.Service
	mfa             *application.OperatorMFA
	tokens          *application.OperatorTokens
	passkeys        *application.OperatorPasskey
	accountPasskeys *application.AccountPasskey
	// auth is nil when the module was composed without session dependencies.
	auth *application.AccountAuthentication
	// operatorAuth and accountAccess are nil when the module was composed
	// without operator dependencies.
	operatorAuth  *application.OperatorAuthentication
	accountAccess *application.OperatorAccountAccess
	// lifecycle is nil when the module was composed without lifecycle
	// dependencies.
	lifecycle *application.AccountLifecycle
	// roles is nil when the module was composed without lifecycle
	// dependencies.
	roles *application.RoleAdministration
	// resets is nil when the module was composed without password reset
	// dependencies.
	resets *application.PasswordReset
	// invitations is nil when the module was composed without the school
	// invitation dependencies.
	invitations *application.SchoolInvitation
	// provisioning is nil when the module was composed without lifecycle
	// dependencies.
	provisioning *application.AccountProvisioning
	// administration is nil when the module was composed without lifecycle
	// dependencies.
	administration *application.AccountAdministration
	// operatorProvisioning is nil when the module was composed without the
	// operator provisioning dependencies.
	operatorProvisioning *application.OperatorProvisioning
	// invitationMaintenance is always composed: spending a deleted school's
	// invitations and deleting expired ones need no flow dependencies.
	invitationMaintenance *application.SchoolInvitationMaintenance
	// mfaFlows carries the second factor and the passkey ceremonies. Its
	// members are nil when the module was composed without them.
	mfaFlows mfaFlows
	// runtime attaches the composed unit of work ahead of every session flow.
	runtime func(context.Context) context.Context
}

func (e engine) FindAccount(ctx context.Context, id int64) (identityaccess.Account, error) {
	value, err := e.service.FindAccount(ctx, id)
	return identityaccess.Account(value), mapError(err)
}

func (e engine) FindAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error) {
	value, err := e.service.FindAccountByEmail(ctx, email)
	return identityaccess.Account(value), mapError(err)
}

func (e engine) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (identityaccess.GuardianTenantAccess, error) {
	value, err := e.service.GrantGuardianTenantAccess(ctx, accountID)
	return identityaccess.GuardianTenantAccess(value), mapError(err)
}

func (e engine) FindOperator(ctx context.Context, id int64) (identityaccess.Operator, error) {
	value, err := e.service.FindOperator(ctx, id)
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) FindOperatorForUpdate(ctx context.Context, id int64) (identityaccess.Operator, error) {
	value, err := e.service.FindOperatorForUpdate(ctx, id)
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) FindOperatorByEmail(ctx context.Context, email string) (identityaccess.Operator, error) {
	value, err := e.service.FindOperatorByEmail(ctx, email)
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) ListOperators(ctx context.Context) ([]identityaccess.Operator, error) {
	values, err := e.service.ListOperators(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.Operator, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.Operator(value))
	}
	return result, nil
}

func (e engine) CreateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	value, err := e.service.CreateOperator(ctx, domain.Operator(operator))
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) UpdateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	value, err := e.service.UpdateOperator(ctx, domain.Operator(operator))
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) DeleteOperator(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteOperator(ctx, id))
}

func (e engine) RecordOperatorLogin(ctx context.Context, id int64) error {
	return mapError(e.service.RecordOperatorLogin(ctx, id))
}

func (e engine) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (identityaccess.OperatorMFAAttempts, error) {
	value, err := e.service.IncrementOperatorMFAAttempts(ctx, id, threshold, lockout)
	return identityaccess.OperatorMFAAttempts(value), mapError(err)
}

func (e engine) ResetOperatorMFAAttempts(ctx context.Context, id int64) error {
	return mapError(e.service.ResetOperatorMFAAttempts(ctx, id))
}

func (e engine) FindOperatorSessionForUpdate(ctx context.Context, token string) (identityaccess.OperatorSession, error) {
	value, err := e.service.FindOperatorSessionForUpdate(ctx, token)
	return identityaccess.OperatorSession(value), mapError(err)
}

func (e engine) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (identityaccess.OperatorSession, error) {
	value, err := e.service.LatestOperatorSessionInFamily(ctx, familyID)
	return identityaccess.OperatorSession(value), mapError(err)
}

func (e engine) CreateOperatorSession(ctx context.Context, session identityaccess.OperatorSession) (identityaccess.OperatorSession, error) {
	value, err := e.service.CreateOperatorSession(ctx, domain.OperatorSession(session))
	return identityaccess.OperatorSession(value), mapError(err)
}

func (e engine) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return mapError(e.service.MarkOperatorSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt))
}

func (e engine) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error {
	return mapError(e.service.DeleteExpiredRotatedOperatorSessions(ctx, familyID, now))
}

func (e engine) DeleteOperatorSession(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteOperatorSession(ctx, id))
}

func (e engine) RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]identityaccess.OperatorSession, error) {
	values, err := e.service.RevokeOperatorSessions(ctx, operatorID)
	return operatorSessions(values), mapError(err)
}

func (e engine) RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]identityaccess.OperatorSession, error) {
	values, err := e.service.RevokeOperatorSessionFamily(ctx, familyID)
	return operatorSessions(values), mapError(err)
}

func (e engine) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error) {
	deleted, err := e.service.DeleteExpiredOperatorSessions(ctx, now)
	return deleted, mapError(err)
}

func (e engine) FindAccountSession(ctx context.Context, token string) (identityaccess.AccountSession, error) {
	value, err := e.service.FindAccountSession(ctx, token)
	return identityaccess.AccountSession(value), mapError(err)
}

func (e engine) FindAccountSessionForUpdate(ctx context.Context, token string) (identityaccess.AccountSession, error) {
	value, err := e.service.FindAccountSessionForUpdate(ctx, token)
	return identityaccess.AccountSession(value), mapError(err)
}

func (e engine) LatestAccountSessionInFamily(ctx context.Context, familyID string) (identityaccess.AccountSession, error) {
	value, err := e.service.LatestAccountSessionInFamily(ctx, familyID)
	return identityaccess.AccountSession(value), mapError(err)
}

func (e engine) ListAccountSessions(ctx context.Context, filter identityaccess.AccountSessionFilter) ([]identityaccess.AccountSession, error) {
	values, err := e.service.ListAccountSessions(ctx, domain.AccountSessionFilter{
		AccountID: filter.AccountID, FamilyID: filter.FamilyID, Mobile: filter.Mobile, Liveness: domain.AccountSessionLiveness(filter.Liveness),
	})
	return accountSessions(values), mapError(err)
}

func (e engine) CountExpiredAccountSessions(ctx context.Context) (int, error) {
	count, err := e.service.CountExpiredAccountSessions(ctx)
	return count, mapError(err)
}

func (e engine) ListInactiveAccountIDsWithLiveSessions(ctx context.Context) ([]int64, error) {
	ids, err := e.service.ListInactiveAccountIDsWithLiveSessions(ctx)
	return ids, mapError(err)
}

func (e engine) HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error) {
	exists, err := e.service.HasLiveAccountSessionsCreatedAfter(ctx, accountID, since)
	return exists, mapError(err)
}

func (e engine) CreateAccountSession(ctx context.Context, session identityaccess.AccountSession) (identityaccess.AccountSession, error) {
	value, err := e.service.CreateAccountSession(ctx, domain.AccountSession(session))
	return identityaccess.AccountSession(value), mapError(err)
}

func (e engine) MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return mapError(e.service.MarkAccountSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt))
}

func (e engine) DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error {
	return mapError(e.service.DeleteExpiredRotatedAccountSessions(ctx, accountID, now))
}

func (e engine) RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error {
	return mapError(e.service.RetireAccountSessionFamily(ctx, accountID, familyID, expiry))
}

func (e engine) EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int) ([]identityaccess.AccountSession, error) {
	values, err := e.service.EnforceAccountSessionCap(ctx, accountID, portalScope, keep)
	return accountSessions(values), mapError(err)
}

func (e engine) DeleteAccountSession(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteAccountSession(ctx, id))
}

func (e engine) RevokeAccountSessionFamily(ctx context.Context, familyID string) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAccountSessionFamily(ctx, familyID)
	return accountSessions(values), mapError(err)
}

func (e engine) RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAccountSessionsInTenant(ctx, accountID)
	return accountSessions(values), mapError(err)
}

func (e engine) RevokeAllAccountSessions(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAllAccountSessions(ctx, accountID)
	return accountSessions(values), mapError(err)
}

func (e engine) RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAccountSessionsCreatedAtOrBefore(ctx, accountID, cutoff)
	return accountSessions(values), mapError(err)
}

func (e engine) RevokeTenantAccountSessions(ctx context.Context, tenantID int64) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeTenantAccountSessions(ctx, tenantID)
	return accountSessions(values), mapError(err)
}

func (e engine) DeleteExpiredAccountSessions(ctx context.Context) (int, error) {
	deleted, err := e.service.DeleteExpiredAccountSessions(ctx)
	return deleted, mapError(err)
}

func accountSessions(values []domain.AccountSession) []identityaccess.AccountSession {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.AccountSession, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.AccountSession(value))
	}
	return result
}

func operatorSessions(values []domain.OperatorSession) []identityaccess.OperatorSession {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.OperatorSession, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.OperatorSession(value))
	}
	return result
}

func mapError(err error) error {
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
	case errors.Is(err, domain.ErrOperatorNotFound):
		return identityaccess.ErrOperatorNotFound
	case errors.Is(err, domain.ErrOperatorSessionNotFound):
		return identityaccess.ErrOperatorSessionNotFound
	case errors.Is(err, domain.ErrOperatorSessionRotated):
		return identityaccess.ErrOperatorSessionRotated
	case errors.Is(err, domain.ErrAccountSessionNotFound):
		return identityaccess.ErrAccountSessionNotFound
	case errors.Is(err, domain.ErrAccountSessionRotated):
		return identityaccess.ErrAccountSessionRotated
	case errors.Is(err, domain.ErrOperatorMFACredentialNotFound):
		return identityaccess.ErrOperatorMFACredentialNotFound
	case errors.Is(err, domain.ErrOperatorMFAChallengeNotFound):
		return identityaccess.ErrOperatorMFAChallengeNotFound
	case errors.Is(err, domain.ErrOperatorMFAChallengeStateChanged):
		return identityaccess.ErrOperatorMFAChallengeStateChanged
	case errors.Is(err, domain.ErrOperatorTrustedDeviceNotFound):
		return identityaccess.ErrOperatorTrustedDeviceNotFound
	case errors.Is(err, domain.ErrOperatorInvitationNotFound):
		return identityaccess.ErrOperatorInvitationNotFound
	case errors.Is(err, domain.ErrOperatorEmailChangeNotFound):
		return identityaccess.ErrOperatorEmailChangeNotFound
	case errors.Is(err, domain.ErrOperatorPasskeyNotFound):
		return identityaccess.ErrOperatorPasskeyNotFound
	case errors.Is(err, domain.ErrOperatorPasskeySessionNotFound):
		return identityaccess.ErrOperatorPasskeySessionNotFound
	case errors.Is(err, domain.ErrAccountPasskeyNotFound):
		return identityaccess.ErrAccountPasskeyNotFound
	case errors.Is(err, domain.ErrAccountPasskeySessionNotFound):
		return identityaccess.ErrAccountPasskeySessionNotFound
	default:
		return err
	}
}
