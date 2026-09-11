// Package identityaccess is the public Identity & Access capability other
// owners consume. The full Identity & Access migration comes late in #2580;
// this package exposes the narrow facts and commands earlier cutovers need
// just in time, starting with guardian portal access for enrollment
// acceptance (#2699).
//
// Accounts are platform-wide rows without a tenant. The school mapping
// (`auth.account_tenants`) and the guardian base role assignment
// (`auth.account_roles`) belong to the tenant in context.
package identityaccess

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrAccountNotFound reports a lookup that matched no platform account.
	ErrAccountNotFound = errors.New("account not found")
	// ErrTenantRequired reports a tenant-scoped command without a tenant.
	ErrTenantRequired = errors.New("tenant is required")
	// ErrGuardianRoleMissing reports a tenant without a guardian base role.
	ErrGuardianRoleMissing = errors.New("guardian role not found")
)

// ErrorCode maps a capability error to the stable code recorded in metrics.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrGuardianRoleMissing):
		return "not_found"
	case errors.Is(err, ErrTenantRequired):
		return "tenant_required"
	default:
		return "internal"
	}
}

// Account is the platform login fact enrollment acceptance needs: which
// address owns the account.
type Account struct {
	ID    int64
	Email string
}

// GuardianTenantAccess is the result of granting an account guardian access
// to the tenant in context. RoleAssigned is false when the guardian role was
// already assigned for this school.
type GuardianTenantAccess struct {
	AccountID    int64
	TenantID     int64
	RoleID       int64
	RoleAssigned bool
}

// GuardianAccessQuery resolves platform accounts. Both lookups return
// ErrAccountNotFound for an unknown account; e-mail matching is
// case-insensitive.
type GuardianAccessQuery interface {
	FindAccount(ctx context.Context, id int64) (Account, error)
	FindAccountByEmail(ctx context.Context, email string) (Account, error)
}

// GuardianAccessCommand makes an account's guardian membership in the tenant
// in context usable: the school mapping is created or reactivated and the
// guardian base role is assigned once. The command joins the caller's
// tenant transaction.
type GuardianAccessCommand interface {
	GrantGuardianTenantAccess(ctx context.Context, accountID int64) (GuardianTenantAccess, error)
}

// GuardianAccess is the capability enrollment acceptance consumes.
type GuardianAccess interface {
	GuardianAccessQuery
	GuardianAccessCommand
}

// Engine is the composed implementation behind the public module.
type Engine interface {
	GuardianAccess
}

// Module is the public Identity & Access facade.
type Module struct {
	engine Engine
}

// NewModule wraps the composed engine. Composition supplies it; consumers
// depend on the interfaces above.
func NewModule(engine Engine) *Module {
	if engine == nil {
		panic("identity access: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindAccount(ctx context.Context, id int64) (Account, error) {
	account, err := m.engine.FindAccount(ctx, id)
	if err != nil {
		return Account{}, fmt.Errorf("identity access: find account: %w", err)
	}
	return account, nil
}

func (m *Module) FindAccountByEmail(ctx context.Context, email string) (Account, error) {
	account, err := m.engine.FindAccountByEmail(ctx, email)
	if err != nil {
		return Account{}, fmt.Errorf("identity access: find account by email: %w", err)
	}
	return account, nil
}

func (m *Module) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (GuardianTenantAccess, error) {
	access, err := m.engine.GrantGuardianTenantAccess(ctx, accountID)
	if err != nil {
		return GuardianTenantAccess{}, fmt.Errorf("identity access: grant guardian tenant access: %w", err)
	}
	return access, nil
}
