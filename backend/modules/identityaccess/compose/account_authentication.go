package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The account-authentication flows (#3251) need facts other owners hold and
// runtime seams the composition root binds. The seams below are expressed in
// public values so the root can implement them without the module's
// internal vocabulary; this package adapts them to the consumer-owned ports.

// SchoolDirectory reads the Organisation & Tenancy school facts login
// resolves. Missing schools report found=false.
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (identityaccess.School, bool, error)
	FindSchoolBySubdomain(ctx context.Context, subdomain string) (identityaccess.School, bool, error)
	LockSchoolShared(ctx context.Context, id int64) (identityaccess.School, bool, error)
	ListActiveSchoolsOfAccount(ctx context.Context, accountID int64) ([]identityaccess.School, error)
}

// PersonDirectory reads the person name of an account in the tenant the
// context carries.
type PersonDirectory interface {
	FindPersonName(ctx context.Context, accountID int64) (firstName, lastName string, found bool, err error)
}

// PasswordVerifier checks a password against its stored hash.
type PasswordVerifier interface {
	VerifyPassword(password, hash string) (bool, error)
}

// TokenCodec signs and parses the session JWTs.
type TokenCodec interface {
	IssueTokenPair(access identityaccess.SessionClaims, refresh identityaccess.RefreshClaims) (accessToken, refreshToken string, err error)
	IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error)
	ParseAccessToken(token string) (identityaccess.SessionClaims, error)
	ParseRefreshToken(token string) (identityaccess.RefreshClaims, error)
	RefreshExpiry() time.Duration
}

// MFAGate is the retained MFA service as login consults it. Configured
// false means "not required / not enrolled".
type MFAGate interface {
	Configured() bool
	IsRequired(ctx context.Context, accountID int64, email string, roleNames []string, tenantID int64) (bool, error)
	ResolvePolicy(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error)
	ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error)
	HasEnrollment(ctx context.Context, accountID int64) (bool, error)
	VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, cookie string) (bool, error)
	StartChallenge(ctx context.Context, accountID, tenantID int64, scope, ipAddress string) (string, error)
	IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool
	TrustedDeviceDays(ctx context.Context, tenantID int64) int
}

// MFAPolicyLock pins a school's MFA mode for the caller's transaction.
type MFAPolicyLock interface {
	LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error
}

// AuthAudit appends authentication events and resolves pending wipes.
type AuthAudit interface {
	RecordAuthEvent(ctx context.Context, event identityaccess.AuthEvent) error
	ListPendingAccountWideWipes(ctx context.Context) ([]identityaccess.PendingAccountWideWipe, error)
	ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]identityaccess.PendingAccountWideWipe, error)
}

// PushSubscriptionCleanup removes the Delivery platform's push rows a
// revocation orphans. portal is "staff", "school" or "parent".
type PushSubscriptionCleanup interface {
	DeleteStaffByAccount(ctx context.Context, accountID int64) error
	DeleteSchoolByAccount(ctx context.Context, accountID int64) error
	DeleteParentByAccount(ctx context.Context, accountID int64) error
	DeleteByTokenFamily(ctx context.Context, accountID int64, familyID string) error
	DeleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal string) error
	DeleteOrphaned(ctx context.Context) error
}

// SessionDependencies are the seams the account-authentication flows need
// beyond the database. TenantRuntime attaches the unit of work the flows
// open their transactions under; nil leaves the context as is.
type SessionDependencies struct {
	Schools       SchoolDirectory
	Persons       PersonDirectory
	Passwords     PasswordVerifier
	Codec         TokenCodec
	MFA           MFAGate
	MFALock       MFAPolicyLock
	Audit         AuthAudit
	Push          PushSubscriptionCleanup
	TenantRuntime func(context.Context) context.Context
	Logger        *slog.Logger
}

func newAccountAuthentication(service *application.Service, store ports.AccountLoginStore, deps *SessionDependencies) (*application.AccountAuthentication, error) {
	if deps == nil {
		return nil, nil
	}
	switch {
	case deps.Schools == nil, deps.Persons == nil, deps.Passwords == nil, deps.Codec == nil, deps.MFA == nil,
		deps.MFALock == nil, deps.Audit == nil, deps.Push == nil:
		return nil, errors.New("identity access compose: every session dependency is required")
	}
	attach := deps.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewAccountAuthentication(service, application.AccountAuthenticationDependencies{
		Store:     store,
		Schools:   schoolDirectory{deps.Schools},
		Persons:   deps.Persons,
		Passwords: deps.Passwords,
		Codec:     tokenCodec{deps.Codec},
		MFA:       mfaGate{deps.MFA},
		MFALock:   deps.MFALock,
		Audit:     authAudit{deps.Audit},
		Push:      deps.Push,
		Runtime:   tenantRuntime{attach: attach, runner: tenant.NewTransactionRunner()},
		Rotation:  rotationPolicy{},
		Logger:    deps.Logger,
	})
}

type schoolDirectory struct{ source SchoolDirectory }

func (d schoolDirectory) FindSchool(ctx context.Context, id int64) (domain.School, bool, error) {
	school, found, err := d.source.FindSchool(ctx, id)
	return domain.School(school), found, err
}

func (d schoolDirectory) FindSchoolBySubdomain(ctx context.Context, subdomain string) (domain.School, bool, error) {
	school, found, err := d.source.FindSchoolBySubdomain(ctx, subdomain)
	return domain.School(school), found, err
}

func (d schoolDirectory) LockSchoolShared(ctx context.Context, id int64) (domain.School, bool, error) {
	school, found, err := d.source.LockSchoolShared(ctx, id)
	return domain.School(school), found, err
}

func (d schoolDirectory) ListActiveSchoolsOfAccount(ctx context.Context, accountID int64) ([]domain.School, error) {
	schools, err := d.source.ListActiveSchoolsOfAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, domain.School(school))
	}
	return result, nil
}

type tokenCodec struct{ source TokenCodec }

func (c tokenCodec) IssueTokenPair(access domain.SessionClaims, refresh domain.RefreshClaims) (string, string, error) {
	return c.source.IssueTokenPair(identityaccess.SessionClaims(access), identityaccess.RefreshClaims(refresh))
}

func (c tokenCodec) IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error) {
	return c.source.IssueMFAEnrollmentToken(accountID, tenantID, scope, ttl)
}

func (c tokenCodec) ParseAccessToken(token string) (domain.SessionClaims, error) {
	claims, err := c.source.ParseAccessToken(token)
	return domain.SessionClaims(claims), err
}

func (c tokenCodec) ParseRefreshToken(token string) (domain.RefreshClaims, error) {
	claims, err := c.source.ParseRefreshToken(token)
	return domain.RefreshClaims(claims), err
}

func (c tokenCodec) RefreshExpiry() time.Duration { return c.source.RefreshExpiry() }

type mfaGate struct{ source MFAGate }

func (g mfaGate) Configured() bool { return g.source.Configured() }

func (g mfaGate) IsRequired(ctx context.Context, accountID int64, email string, roleNames []string, tenantID int64) (bool, error) {
	return g.source.IsRequired(ctx, accountID, email, roleNames, tenantID)
}

func (g mfaGate) ResolvePolicy(ctx context.Context, accountID, tenantID int64) (ports.MFAPolicy, error) {
	policy, err := g.source.ResolvePolicy(ctx, accountID, tenantID)
	return policy, err
}

func (g mfaGate) ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (ports.MFAPolicy, error) {
	policy, err := g.source.ResolvePolicyInTx(ctx, accountID, tenantID)
	return policy, err
}

func (g mfaGate) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	return g.source.HasEnrollment(ctx, accountID)
}

func (g mfaGate) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, cookie string) (bool, error) {
	return g.source.VerifyTrustedDevice(ctx, accountID, tenantID, cookie)
}

func (g mfaGate) StartChallenge(ctx context.Context, accountID, tenantID int64, scope, ipAddress string) (string, error) {
	return g.source.StartChallenge(ctx, accountID, tenantID, scope, ipAddress)
}

func (g mfaGate) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return g.source.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (g mfaGate) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return g.source.TrustedDeviceDays(ctx, tenantID)
}

type authAudit struct{ source AuthAudit }

func (a authAudit) RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error {
	public := identityaccess.AuthEvent{
		AccountID: event.AccountID, TenantID: event.TenantID, Type: event.Type, Success: event.Success,
		IPAddress: event.IPAddress, UserAgent: event.UserAgent, ErrorMessage: event.ErrorMessage,
	}
	if event.RevokedSessions != nil {
		evidence := identityaccess.RevokedSessionsEvidence(*event.RevokedSessions)
		public.RevokedSessions = &evidence
	}
	if event.PendingWipe != nil {
		evidence := identityaccess.PendingWipeEvidence(*event.PendingWipe)
		public.PendingWipe = &evidence
	}
	if event.CompletedWipe != nil {
		evidence := identityaccess.CompletedWipeEvidence(*event.CompletedWipe)
		public.CompletedWipe = &evidence
	}
	return a.source.RecordAuthEvent(ctx, public)
}

func (a authAudit) ListPendingAccountWideWipes(ctx context.Context) ([]domain.PendingAccountWideWipe, error) {
	pending, err := a.source.ListPendingAccountWideWipes(ctx)
	return pendingWipes(pending), err
}

func (a authAudit) ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]domain.PendingAccountWideWipe, error) {
	pending, err := a.source.ClaimPendingAccountWideWipes(ctx, accountID)
	return pendingWipes(pending), err
}

func pendingWipes(values []identityaccess.PendingAccountWideWipe) []domain.PendingAccountWideWipe {
	if values == nil {
		return nil
	}
	result := make([]domain.PendingAccountWideWipe, 0, len(values))
	for _, value := range values {
		result = append(result, domain.PendingAccountWideWipe(value))
	}
	return result
}

// tenantRuntime binds the shared tenant transaction runtime: administrative
// transactions for the pre-authentication flows, tenant transactions for
// audit evidence, and the after-commit hooks a revocation defers to.
type tenantRuntime struct {
	attach func(context.Context) context.Context
	runner *tenant.TransactionRunner
}

func (r tenantRuntime) WithAdminTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(r.attach(ctx), fn)
}

func (r tenantRuntime) WithTenantTx(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(r.attach(ctx), struct{}{}, tenantID, func(txCtx context.Context, _ any) error {
		return fn(txCtx)
	})
}

// RunInTx joins an ambient transaction, runs tenantless scoped callers
// (organization and platform scope) administratively and otherwise opens the
// transaction of the tenant in context.
func (r tenantRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	ctx = r.attach(ctx)
	if tenant.FromContext(ctx) == 0 && tenant.ScopeFromContext(ctx) != "" {
		return tenant.WithinAdmin(ctx, fn)
	}
	return r.runner.RunInTx(ctx, fn)
}

func (tenantRuntime) IsAdminTx(ctx context.Context) bool { return tenant.IsAdminTx(ctx) }

func (tenantRuntime) HasTransaction(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

func (tenantRuntime) HasAfterCommitHooks(ctx context.Context) bool {
	return tenant.HasAfterCommitHooks(ctx)
}

func (tenantRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (tenantRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (tenantRuntime) Scope(ctx context.Context) string { return tenant.ScopeFromContext(ctx) }

func (tenantRuntime) OrgID(ctx context.Context) int64 { return tenant.OrgFromContext(ctx) }

func (tenantRuntime) WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func (tenantRuntime) Detach(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTenant(tenant.ContextWithoutTransaction(ctx)))
}

func (tenantRuntime) WithoutTransaction(ctx context.Context) context.Context {
	return tenant.ContextWithoutTransaction(ctx)
}

type rotationPolicy struct{}

func (rotationPolicy) RecoveryGrace() time.Duration { return rotation.RecoveryGrace }
func (rotationPolicy) MaxRecoveryHops() int         { return rotation.MaxRecoveryHops }
func (rotationPolicy) RecoveryProofHash(ctx context.Context) []byte {
	return rotation.RecoveryProofHash(ctx)
}
func (rotationPolicy) MatchesRecoveryProof(ctx context.Context, expected []byte) bool {
	return rotation.MatchesRecoveryProof(ctx, expected)
}
func (rotationPolicy) FamilyFingerprint(familyID string) string {
	return rotation.FamilyFingerprint(familyID)
}

// Engine methods: every flow answers ErrAccountAuthenticationUnavailable
// when the module was composed without session dependencies.

var errAccountAuthenticationUnavailable = identityaccess.ErrAccountAuthenticationUnavailable

// attach hands every session flow the unit of work it was composed with so
// the reads outside a transaction resolve the tenant runtime too.
func (e engine) attach(ctx context.Context) context.Context {
	if e.runtime == nil {
		return ctx
	}
	return e.runtime(ctx)
}

func (e engine) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.LoginWithAudit(e.attach(ctx), email, password, ipAddress, userAgent, tenantSlug)
	return access, refresh, authenticationError(err)
}

func (e engine) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*identityaccess.LoginResult, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := e.auth.LoginWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie)
	return loginResult(result), authenticationError(err)
}

func (e engine) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.LoginParentWithAudit(e.attach(ctx), email, password, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.LoginResult, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := e.auth.LoginSchoolWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, trustedDeviceCookie)
	return loginResult(result), authenticationError(err)
}

func (e engine) LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*identityaccess.LoginResult, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := e.auth.LoginSchoolAtTenantWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
	return loginResult(result), authenticationError(err)
}

func (e engine) IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.IssueTokensForAuthenticatedAccount(e.attach(ctx), accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.IssueSchoolTokensForAuthenticatedAccount(e.attach(ctx), accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.RefreshTokenWithAudit(e.attach(ctx), refreshToken, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.LogoutWithAudit(e.attach(ctx), refreshToken, ipAddress, userAgent))
}

func (e engine) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.SwitchTenant(e.attach(ctx), accountID, tenantSlug, presentedFamilyID)
	return access, refresh, authenticationError(err)
}

func (e engine) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.SwitchSchool(e.attach(ctx), accountID, tenantSlug, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if e.auth == nil {
		return false, errAccountAuthenticationUnavailable
	}
	allowed, err := e.auth.HasSchoolPortalAccess(e.attach(ctx), accountID, tenantID)
	return allowed, authenticationError(err)
}

func (e engine) ValidateSessionTokens(ctx context.Context, accessToken, refreshToken, portal string) (identityaccess.SessionClaims, error) {
	if e.auth == nil {
		return identityaccess.SessionClaims{}, errAccountAuthenticationUnavailable
	}
	claims, err := e.auth.ValidateSessionTokens(e.attach(ctx), accessToken, refreshToken, portal)
	return identityaccess.SessionClaims(claims), authenticationError(err)
}

func (e engine) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if e.auth == nil {
		return false, errAccountAuthenticationUnavailable
	}
	return e.auth.VerifyAccountTenantMembership(e.attach(ctx), accountID, tenantID)
}

func (e engine) CountExpiredTokens(ctx context.Context) (int, error) {
	if e.auth == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := e.auth.CountExpiredTokens(e.attach(ctx))
	return count, authenticationError(err)
}

func (e engine) CleanupExpiredTokens(ctx context.Context) (int, error) {
	if e.auth == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := e.auth.CleanupExpiredTokens(e.attach(ctx))
	return count, authenticationError(err)
}

func (e engine) ListActiveSessions(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	sessions, err := e.auth.ListActiveSessions(e.attach(ctx), accountID)
	return accountSessions(sessions), authenticationError(err)
}

func (e engine) ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	ids, err := e.auth.ListSessionIDs(e.attach(ctx), accountID)
	return ids, authenticationError(err)
}

func (e engine) RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.RevokeAllTokensWithReason(e.attach(ctx), accountID, reason))
}

func (e engine) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	if e.auth == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := e.auth.RevokeTokensByTenantID(e.attach(ctx), tenantID)
	return count, authenticationError(err)
}

func (e engine) DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]identityaccess.AccountSession, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	sessions, err := e.auth.DeleteAccountSessionsWithAudit(e.attach(ctx), accountID, reason, ipAddress, userAgent)
	return accountSessions(sessions), authenticationError(err)
}

func (e engine) QueuePushCleanup(ctx context.Context, accountID int64, sessions []identityaccess.AccountSession, reason string) {
	if e.auth == nil {
		return
	}
	values := make([]domain.AccountSession, 0, len(sessions))
	for _, session := range sessions {
		values = append(values, domain.AccountSession(session))
	}
	e.auth.QueuePushCleanup(e.attach(ctx), accountID, values, reason)
}

func (e engine) ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.ScheduleAccountWideRevoke(e.attach(ctx), accountID, reason, ipAddress, userAgent))
}

func (e engine) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.MarkAccountWideWipeCompleted(e.attach(ctx), accountID))
}

func (e engine) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountClaims, error) {
	if e.auth == nil {
		return identityaccess.AccountClaims{}, errAccountAuthenticationUnavailable
	}
	payload, err := e.auth.LoadAccountClaims(e.attach(ctx), accountID, tenantID)
	if err != nil {
		return identityaccess.AccountClaims{}, authenticationError(err)
	}
	roles := make([]identityaccess.AccountRole, 0, len(payload.Roles))
	for _, role := range payload.Roles {
		roles = append(roles, identityaccess.AccountRole{ID: role.RoleID, Name: role.Name, IsSystem: role.IsSystem, TenantID: role.TenantID})
	}
	return identityaccess.AccountClaims{
		RoleNames: payload.RoleNames, Roles: roles, Permissions: payload.Permissions, Username: payload.Username,
		FirstName: payload.FirstName, LastName: payload.LastName, IsAdmin: payload.IsAdmin,
		TenantID: payload.TenantID, OrgID: payload.OrgID, Scope: payload.Scope,
	}, nil
}

func (e engine) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if e.auth == nil {
		return false, 0, errAccountAuthenticationUnavailable
	}
	found, tenantID, err := e.auth.FindGuardianTenant(e.attach(ctx), accountID)
	return found, tenantID, authenticationError(err)
}

func (e engine) FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if e.auth == nil {
		return false, 0, errAccountAuthenticationUnavailable
	}
	found, tenantID, err := e.auth.FindSchoolPortalTenant(e.attach(ctx), accountID)
	return found, tenantID, authenticationError(err)
}

func loginResult(value *domain.LoginResult) *identityaccess.LoginResult {
	if value == nil {
		return nil
	}
	result := identityaccess.LoginResult{
		Status: identityaccess.LoginStatus(value.Status), AccessToken: value.AccessToken, RefreshToken: value.RefreshToken,
		ChallengeToken: value.ChallengeToken, MaskedEmail: value.MaskedEmail, MFAEnrollmentRequired: value.MFAEnrollmentRequired,
		TrustedDeviceEnabled: value.TrustedDeviceEnabled, TrustedDeviceDays: value.TrustedDeviceDays,
	}
	return &result
}

var authenticationSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrInvalidCredentials, identityaccess.ErrInvalidCredentials},
	{domain.ErrAccountInactive, identityaccess.ErrAccountInactive},
	{domain.ErrAccountNotFound, identityaccess.ErrAccountNotFound},
	{domain.ErrTenantNotFound, identityaccess.ErrTenantNotFound},
	{domain.ErrTenantAccessDenied, identityaccess.ErrTenantAccessDenied},
	{domain.ErrParentMustUseParentPortal, identityaccess.ErrParentMustUseParentPortal},
	{domain.ErrAccountNoGuardianRole, identityaccess.ErrAccountNoGuardianRole},
	{domain.ErrAccountNoSchoolPortalRole, identityaccess.ErrAccountNoSchoolPortalRole},
	{domain.ErrMustUseSchoolPortal, identityaccess.ErrMustUseSchoolPortal},
	{domain.ErrInvalidToken, identityaccess.ErrInvalidToken},
	{domain.ErrTokenExpired, identityaccess.ErrTokenExpired},
	{domain.ErrTokenNotFound, identityaccess.ErrTokenNotFound},
	{domain.ErrMFAStatusUnavailable, identityaccess.ErrMFAStatusUnavailable},
	{domain.ErrTenantRequired, identityaccess.ErrTenantRequired},
	{domain.ErrAccountSessionNotFound, identityaccess.ErrAccountSessionNotFound},
	{domain.ErrAccountSessionRotated, identityaccess.ErrAccountSessionRotated},
}

// authenticationError translates a flow error to the public contract: the
// operation envelope keeps its text, and a cause that carries an internal
// sentinel gains the public one while keeping its message.
func authenticationError(err error) error {
	if err == nil {
		return nil
	}
	var operation *application.OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: authenticationError(operation.Err)}
	}
	for _, sentinel := range authenticationSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return err
}

// translatedError keeps a wrapped cause's text while exposing the public
// sentinel to errors.Is.
type translatedError struct {
	text   string
	public error
	cause  error
}

func (e *translatedError) Error() string { return e.text }

func (e *translatedError) Unwrap() []error { return []error{e.public, e.cause} }
