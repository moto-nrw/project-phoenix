package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application/rotation"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The account-authentication flows (#3251) need facts other owners hold and
// runtime seams the composition root binds. The seams below are expressed in
// public values so the root can implement them without the module's
// internal vocabulary; this package adapts them to the consumer-owned ports.

// PersonDirectory reads the person name of an account in the tenant the
// context carries.
type PersonDirectory interface {
	FindPersonName(ctx context.Context, accountID int64) (firstName, lastName string, found bool, err error)
}

// PasswordVerifier checks a password against its stored hash.
type PasswordVerifier interface {
	VerifyPassword(password, hash string) (bool, error)
}

// MFAPolicyLock pins a school's MFA mode for the caller's transaction.
type MFAPolicyLock interface {
	LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error
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
	Schools       identityPorts.SchoolDirectory
	Persons       PersonDirectory
	Passwords     PasswordVerifier
	Codec         identityPorts.TokenCodec
	MFALock       MFAPolicyLock
	Audit         identityPorts.AuthAudit
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
	return application.NewAccountAuthentication(service, application.AccountAuthenticationDependencies{
		Store:     store,
		Schools:   application.SchoolDirectory{Source: deps.Schools},
		Persons:   deps.Persons,
		Passwords: deps.Passwords,
		Codec:     application.TokenCodec{Source: deps.Codec},
		MFA:       mfa,
		MFALock:   deps.MFALock,
		Audit:     application.AuthAudit{Source: deps.Audit},
		Push:      deps.Push,
		Runtime:   tenantRuntime{attach: attach, runner: newTransactionRunner()},
		Rotation:  rotationPolicy{},
		Logger:    deps.Logger,
	})
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
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	scope := tenant.ScopeFromContext(ctx)
	if tenant.FromContext(ctx) == 0 && (scope == "platform" || (scope == "org" && tenant.OrgFromContext(ctx) > 0)) {
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

// Engine methods: every flow answers ErrAccountAuthenticationUnavailable
// when the module was composed without session dependencies.

type rotationPolicy struct{}

func (rotationPolicy) RecoveryGrace() time.Duration { return rotation.RecoveryGrace }

func (rotationPolicy) MaxRecoveryHops() int { return rotation.MaxRecoveryHops }

func (rotationPolicy) RecoveryProofHash(ctx context.Context) []byte {
	return rotation.RecoveryProofHash(ctx)
}

func (rotationPolicy) MatchesRecoveryProof(ctx context.Context, expected []byte) bool {
	return rotation.MatchesRecoveryProof(ctx, expected)
}

func (rotationPolicy) FamilyFingerprint(familyID string) string {
	return rotation.FamilyFingerprint(familyID)
}
