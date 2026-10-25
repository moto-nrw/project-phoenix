package compose

import (
	"context"
	"errors"

	boundaryBinding "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
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

func newAccountAuthentication(service *application.Service, store ports.AccountLoginStore, deps *identityPorts.SessionDependencies, mfa ports.MFAGate) (*application.AccountAuthentication, error) {
	if deps == nil {
		return nil, nil
	}
	switch {
	case deps.Schools == nil, deps.Persons == nil, deps.Passwords == nil, deps.Codec == nil,
		deps.PasswordLoginPolicy == nil, deps.MFALock == nil, deps.Audit == nil, deps.Push == nil, deps.PortalModules == nil:
		return nil, errors.New("identity access compose: every session dependency is required")
	}
	attach := deps.TenantRuntime
	return application.NewAccountAuthentication(service, application.AccountAuthenticationDependencies{
		Store:   store,
		Schools: application.SchoolDirectory{SchoolDirectory: deps.Schools}, Traeger: deps.Traeger,
		PortalModules: deps.PortalModules,
		Persons:       deps.Persons,
		Passwords:     deps.Passwords, PasswordPolicy: deps.PasswordLoginPolicy,
		Codec:    application.TokenCodec{TokenCodec: deps.Codec},
		MFA:      mfa,
		MFALock:  deps.MFALock,
		Audit:    application.AuthAudit{Source: deps.Audit},
		Push:     deps.Push,
		Runtime:  tenantRuntime{attach: attach, runner: application.NewTransactionRunner()},
		Attach:   attach,
		Rotation: rotation.RotationPolicy{}, WithRecoveryProof: rotation.WithRecoveryProof,
		Logger: deps.Logger,
	})
}

// tenantRuntime binds the shared tenant transaction runtime: administrative
// transactions for the pre-authentication flows, tenant transactions for
// audit evidence, and the after-commit hooks a revocation defers to.
type tenantRuntime struct {
	attach func(context.Context) context.Context
	runner *tenant.TransactionRunner
}

func (r tenantRuntime) WithAdminTx(ctx context.Context, fn func(context.Context) error) error {
	return boundaryBinding.TenantRuntimeWithAdminTxBinding(ctx, fn, tenant.WithinAdmin, r.attach)
}

func (r tenantRuntime) WithTenantTx(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return boundaryBinding.TenantRuntimeWithTenantTxBinding(ctx, tenantID, fn, tenant.WithTenantTx, r.attach)
}

// RunInTx joins an ambient transaction, runs tenantless scoped callers
// (organization and platform scope) administratively and otherwise opens the
// transaction of the tenant in context.
func (r tenantRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return boundaryBinding.TenantRuntimeRunInTxBinding(ctx, fn, r.attach, tenant.WithinAdmin, r.runner.RunInTx)
}

func (tenantRuntime) IsAdminTx(ctx context.Context) bool { return tenant.IsAdminTx(ctx) }

func (tenantRuntime) HasTransaction(ctx context.Context) bool {
	return boundaryBinding.HasTenantTransaction(ctx)
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
	return boundaryBinding.DetachCallerTransaction(ctx)
}

func (tenantRuntime) WithoutTransaction(ctx context.Context) context.Context {
	return tenant.ContextWithoutTransaction(ctx)
}

func (tenantRuntime) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}
