package application

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	// WithRecoveryProof binds a refresh-token recovery proof to the request
	// context, where the rotation policy reads its hash.
	WithRecoveryProof func(ctx context.Context, proof string) context.Context
}

// NewEngine serves the Identity & Access capability over parts (#4084).
func NewEngine(parts EngineParts) identityaccess.Engine {
	return engine{
		CalendarFeeds:         parts.CalendarFeeds,
		rfidCards:             parts.RFIDCards,
		accountRoleQueries:    parts.AccountRoleQueries,
		guardianSchools:       parts.GuardianSchools,
		profiles:              parts.Profiles,
		service:               parts.Service,
		mfa:                   parts.MFA,
		tokens:                parts.Tokens,
		passkeys:              parts.Passkeys,
		accountPasskeys:       parts.AccountPasskeys,
		auth:                  parts.Auth,
		operatorAuth:          parts.OperatorAuth,
		accountAccess:         parts.AccountAccess,
		lifecycle:             parts.Lifecycle,
		roles:                 parts.Roles,
		resets:                parts.Resets,
		invitations:           parts.Invitations,
		provisioning:          parts.Provisioning,
		administration:        parts.Administration,
		operatorProvisioning:  parts.OperatorProvisioning,
		invitationMaintenance: parts.InvitationMaintenance,
		signInRetention:       parts.SignInRetention,
		AccountRetention:      parts.AccountRetention,
		mfaFlows:              parts.MFAFlows,
		runtime:               parts.Runtime,
		withRecoveryProof:     parts.WithRecoveryProof,
	}
}

type engine struct {
	CalendarFeeds
	// AccountRetention is always composed like signInRetention (#4043).
	*AccountRetention
	rfidCards          *RFIDCards
	accountRoleQueries *AccountRoleQueries
	guardianSchools    *GuardianSchools
	profiles           *AccountProfiles
	service            *Service
	mfa                *OperatorMFA
	tokens             *OperatorTokens
	passkeys           *OperatorPasskey
	accountPasskeys    *AccountPasskey
	// auth is nil when the module was composed without session dependencies.
	auth *AccountAuthentication
	// operatorAuth and accountAccess are nil when the module was composed
	// without operator dependencies.
	operatorAuth  *OperatorAuthentication
	accountAccess *OperatorAccountAccess
	// lifecycle is nil when the module was composed without lifecycle
	// dependencies.
	lifecycle *AccountLifecycle
	// roles is nil when the module was composed without lifecycle
	// dependencies.
	roles *RoleAdministration
	// resets is nil when the module was composed without password reset
	// dependencies.
	resets *PasswordReset
	// invitations is nil when the module was composed without the school
	// invitation dependencies.
	invitations *SchoolInvitation
	// provisioning is nil when the module was composed without lifecycle
	// dependencies.
	provisioning *AccountProvisioning
	// administration is nil when the module was composed without lifecycle
	// dependencies.
	administration *AccountAdministration
	// operatorProvisioning is nil when the module was composed without the
	// operator provisioning dependencies.
	operatorProvisioning *OperatorProvisioning
	// invitationMaintenance is always composed: spending a deleted school's
	// invitations and deleting expired ones need no flow dependencies.
	invitationMaintenance *SchoolInvitationMaintenance
	// signInRetention is always composed: the fixed retention of short-lived
	// sign-in data needs no flow dependencies (#3954).
	signInRetention *SignInDataRetention
	// mfaFlows carries the second factor and the passkey ceremonies. Its
	// members are nil when the module was composed without them.
	mfaFlows MFAFlows
	// runtime attaches the composed unit of work ahead of every session flow.
	runtime func(context.Context) context.Context
	// withRecoveryProof binds a refresh-token recovery proof to the context.
	withRecoveryProof func(ctx context.Context, proof string) context.Context
}

func (e engine) FindAccount(ctx context.Context, id int64) (identityaccess.Account, error) {
	value, err := e.service.FindAccount(ctx, id)
	return identityaccess.Account(value), MapError(err)
}

func (e engine) FindAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error) {
	value, err := e.service.FindAccountByEmail(ctx, email)
	return identityaccess.Account(value), MapError(err)
}

func (e engine) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (identityaccess.GuardianTenantAccess, error) {
	value, err := e.service.GrantGuardianTenantAccess(ctx, accountID)
	return identityaccess.GuardianTenantAccess(value), MapError(err)
}

func (e engine) FindOperator(ctx context.Context, id int64) (identityaccess.Operator, error) {
	value, err := e.service.FindOperator(ctx, id)
	return identityaccess.Operator(value), MapError(err)
}

func (e engine) FindOperatorForUpdate(ctx context.Context, id int64) (identityaccess.Operator, error) {
	value, err := e.service.FindOperatorForUpdate(ctx, id)
	return identityaccess.Operator(value), MapError(err)
}

func (e engine) FindOperatorByEmail(ctx context.Context, email string) (identityaccess.Operator, error) {
	value, err := e.service.FindOperatorByEmail(ctx, email)
	return identityaccess.Operator(value), MapError(err)
}

func (e engine) ListOperators(ctx context.Context) ([]identityaccess.Operator, error) {
	values, err := e.service.ListOperators(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]identityaccess.Operator, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.Operator(value))
	}
	return result, nil
}

func (e engine) CreateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	value, err := e.service.CreateOperator(ctx, domain.Operator(operator))
	return identityaccess.Operator(value), MapError(err)
}

func (e engine) UpdateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	value, err := e.service.UpdateOperator(ctx, domain.Operator(operator))
	return identityaccess.Operator(value), MapError(err)
}

func (e engine) DeleteOperator(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteOperator(ctx, id))
}

func (e engine) RecordOperatorLogin(ctx context.Context, id int64) error {
	return MapError(e.service.RecordOperatorLogin(ctx, id))
}

func (e engine) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (identityaccess.OperatorMFAAttempts, error) {
	value, err := e.service.IncrementOperatorMFAAttempts(ctx, id, threshold, lockout)
	return identityaccess.OperatorMFAAttempts(value), MapError(err)
}

func (e engine) ResetOperatorMFAAttempts(ctx context.Context, id int64) error {
	return MapError(e.service.ResetOperatorMFAAttempts(ctx, id))
}

func (e engine) FindOperatorSessionForUpdate(ctx context.Context, token string) (identityaccess.OperatorSession, error) {
	value, err := e.service.FindOperatorSessionForUpdate(ctx, token)
	return identityaccess.OperatorSession(value), MapError(err)
}

func (e engine) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (identityaccess.OperatorSession, error) {
	value, err := e.service.LatestOperatorSessionInFamily(ctx, familyID)
	return identityaccess.OperatorSession(value), MapError(err)
}

func (e engine) CreateOperatorSession(ctx context.Context, session identityaccess.OperatorSession) (identityaccess.OperatorSession, error) {
	value, err := e.service.CreateOperatorSession(ctx, domain.OperatorSession(session))
	return identityaccess.OperatorSession(value), MapError(err)
}

func (e engine) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return MapError(e.service.MarkOperatorSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt))
}

func (e engine) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error {
	return MapError(e.service.DeleteExpiredRotatedOperatorSessions(ctx, familyID, now))
}

func (e engine) DeleteOperatorSession(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteOperatorSession(ctx, id))
}

func (e engine) RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]identityaccess.OperatorSession, error) {
	values, err := e.service.RevokeOperatorSessions(ctx, operatorID)
	return operatorSessions(values), MapError(err)
}

func (e engine) RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]identityaccess.OperatorSession, error) {
	values, err := e.service.RevokeOperatorSessionFamily(ctx, familyID)
	return operatorSessions(values), MapError(err)
}

func (e engine) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error) {
	deleted, err := e.service.DeleteExpiredOperatorSessions(ctx, now)
	return deleted, MapError(err)
}

func (e engine) FindAccountSession(ctx context.Context, token string) (identityaccess.AccountSession, error) {
	value, err := e.service.FindAccountSession(ctx, token)
	return identityaccess.AccountSession(value), MapError(err)
}

func (e engine) FindAccountSessionForUpdate(ctx context.Context, token string) (identityaccess.AccountSession, error) {
	value, err := e.service.FindAccountSessionForUpdate(ctx, token)
	return identityaccess.AccountSession(value), MapError(err)
}

func (e engine) LatestAccountSessionInFamily(ctx context.Context, familyID string) (identityaccess.AccountSession, error) {
	value, err := e.service.LatestAccountSessionInFamily(ctx, familyID)
	return identityaccess.AccountSession(value), MapError(err)
}

func (e engine) ListAccountSessions(ctx context.Context, filter identityaccess.AccountSessionFilter) ([]identityaccess.AccountSession, error) {
	values, err := e.service.ListAccountSessions(ctx, domain.AccountSessionFilter{
		AccountID: filter.AccountID, FamilyID: filter.FamilyID, Mobile: filter.Mobile, Liveness: domain.AccountSessionLiveness(filter.Liveness),
	})
	return accountSessions(values), MapError(err)
}

func (e engine) CountExpiredAccountSessions(ctx context.Context) (int, error) {
	count, err := e.service.CountExpiredAccountSessions(ctx)
	return count, MapError(err)
}

func (e engine) ListInactiveAccountIDsWithLiveSessions(ctx context.Context) ([]int64, error) {
	ids, err := e.service.ListInactiveAccountIDsWithLiveSessions(ctx)
	return ids, MapError(err)
}

func (e engine) HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error) {
	exists, err := e.service.HasLiveAccountSessionsCreatedAfter(ctx, accountID, since)
	return exists, MapError(err)
}

func (e engine) CreateAccountSession(ctx context.Context, session identityaccess.AccountSession) (identityaccess.AccountSession, error) {
	value, err := e.service.CreateAccountSession(ctx, domain.AccountSession(session))
	return identityaccess.AccountSession(value), MapError(err)
}

func (e engine) MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return MapError(e.service.MarkAccountSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt))
}

func (e engine) DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error {
	return MapError(e.service.DeleteExpiredRotatedAccountSessions(ctx, accountID, now))
}

func (e engine) RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error {
	return MapError(e.service.RetireAccountSessionFamily(ctx, accountID, familyID, expiry))
}

func (e engine) EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int) ([]identityaccess.AccountSession, error) {
	values, err := e.service.EnforceAccountSessionCap(ctx, accountID, portalScope, keep)
	return accountSessions(values), MapError(err)
}

func (e engine) DeleteAccountSession(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteAccountSession(ctx, id))
}

func (e engine) RevokeAccountSessionFamily(ctx context.Context, familyID string) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAccountSessionFamily(ctx, familyID)
	return accountSessions(values), MapError(err)
}

func (e engine) RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAccountSessionsInTenant(ctx, accountID)
	return accountSessions(values), MapError(err)
}

func (e engine) RevokeAllAccountSessions(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAllAccountSessions(ctx, accountID)
	return accountSessions(values), MapError(err)
}

func (e engine) RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeAccountSessionsCreatedAtOrBefore(ctx, accountID, cutoff)
	return accountSessions(values), MapError(err)
}

func (e engine) RevokeTenantAccountSessions(ctx context.Context, tenantID int64) ([]identityaccess.AccountSession, error) {
	values, err := e.service.RevokeTenantAccountSessions(ctx, tenantID)
	return accountSessions(values), MapError(err)
}

func (e engine) DeleteExpiredAccountSessions(ctx context.Context) (int, error) {
	deleted, err := e.service.DeleteExpiredAccountSessions(ctx)
	return deleted, MapError(err)
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
