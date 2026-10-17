package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application/rotation"
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
	ListActiveSchoolsByID(ctx context.Context, schoolIDs []int64) ([]identityaccess.School, error)
	// ListManageableSchoolIDs returns the live, active schools of the
	// organisation: the set an organisation-scoped administrator may
	// administer accounts at.
	ListManageableSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
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
	// IssueAccessToken signs an access-only JWT (the staff preview mints no
	// refresh token); ParseAccessTokenAllowExpired verifies its signature but
	// not its expiry; AccessExpiry is the access token lifetime.
	IssueAccessToken(claims identityaccess.SessionClaims) (string, error)
	ParseAccessTokenAllowExpired(token string) (identityaccess.SessionClaims, error)
	AccessExpiry() time.Duration
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
	MFALock       MFAPolicyLock
	Audit         AuthAudit
	Push          PushSubscriptionCleanup
	TenantRuntime func(context.Context) context.Context
	Logger        *slog.Logger
}

func newAccountAuthentication(service *application.Service, store ports.AccountLoginStore, deps *SessionDependencies, mfa ports.MFAGate) (*application.AccountAuthentication, error) {
	if deps == nil {
		return nil, nil
	}
	switch {
	case deps.Schools == nil, deps.Persons == nil, deps.Passwords == nil, deps.Codec == nil,
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
		MFA:       mfa,
		MFALock:   deps.MFALock,
		Audit:     authAudit{deps.Audit},
		Push:      deps.Push,
		Runtime:   tenantRuntime{attach: attach, runner: newTransactionRunner()},
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

func (d schoolDirectory) ListActiveSchoolsByID(ctx context.Context, schoolIDs []int64) ([]domain.School, error) {
	schools, err := d.source.ListActiveSchoolsByID(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	result := make([]domain.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, domain.School(school))
	}
	return result, nil
}

func (d schoolDirectory) ManageableSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	return d.source.ListManageableSchoolIDs(ctx, organizationID)
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

type authAudit struct{ source AuthAudit }

func (a authAudit) RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error {
	public := identityaccess.AuthEvent{
		AccountID: event.AccountID, TenantID: event.TenantID, Type: event.Type, Success: event.Success,
		IPAddress: event.IPAddress, UserAgent: event.UserAgent, ErrorMessage: event.ErrorMessage,
	}
	if event.TenantAccess != nil {
		evidence := identityaccess.TenantAccessEvidence(*event.TenantAccess)
		public.TenantAccess = &evidence
	}
	if event.RevokedSessions != nil {
		evidence := identityaccess.RevokedSessionsEvidence(*event.RevokedSessions)
		public.RevokedSessions = &evidence
	}
	if event.PendingWipe != nil {
		evidence := identityaccess.PendingWipeEvidence(*event.PendingWipe)
		public.PendingWipe = &evidence
	}
	if event.MFA != nil {
		evidence := identityaccess.MFAEvidence{
			ChallengeID: event.MFA.ChallengeID, DeviceID: event.MFA.DeviceID, LockedUntil: event.MFA.LockedUntil,
		}
		if event.MFA.Admin != nil {
			admin := identityaccess.MFAAdminEvidence(*event.MFA.Admin)
			evidence.Admin = &admin
		}
		public.MFA = &evidence
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

func newTransactionRunner() *tenant.TransactionRunner { return tenant.NewTransactionRunner() }

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

func (tenantRuntime) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
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
