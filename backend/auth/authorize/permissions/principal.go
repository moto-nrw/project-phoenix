package permissions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrPrincipalRequired = errors.New("security principal is required")
	ErrInvalidPrincipal  = errors.New("security principal is invalid")
)

type Scope string

const (
	ScopeTenant       Scope = ""
	ScopeOrganization Scope = "org"
	ScopePlatform     Scope = "platform"
	ScopeParent       Scope = "parent"
	ScopeSchool       Scope = "school"
)

type PrincipalInput struct {
	AccountID      int64
	TenantID       int64
	OrganizationID int64
	Scope          string
	Roles          []string
	Permissions    []string
	Admin          bool
	FamilyID       string
}

type Principal struct {
	accountID      int64
	tenantID       int64
	organizationID int64
	scope          Scope
	roles          []string
	permissions    []string
	admin          bool
	familyID       string
}

type principalContextKey struct{}

func NewPrincipal(input PrincipalInput) (Principal, error) {
	if input.AccountID <= 0 {
		return Principal{}, fmt.Errorf("%w: account ID must be positive", ErrInvalidPrincipal)
	}
	scope, err := normalizeScope(input.Scope)
	if err != nil {
		return Principal{}, err
	}
	if err := validateScope(scope, input.TenantID, input.OrganizationID); err != nil {
		return Principal{}, err
	}
	return Principal{
		accountID: input.AccountID, tenantID: input.TenantID,
		organizationID: input.OrganizationID, scope: scope,
		roles: slices.Clone(input.Roles), permissions: slices.Clone(input.Permissions),
		admin: input.Admin, familyID: input.FamilyID,
	}, nil
}

func normalizeScope(raw string) (Scope, error) {
	switch strings.TrimSpace(raw) {
	case "", "tenant":
		return ScopeTenant, nil
	case string(ScopeOrganization):
		return ScopeOrganization, nil
	case string(ScopePlatform):
		return ScopePlatform, nil
	case string(ScopeParent):
		return ScopeParent, nil
	case string(ScopeSchool):
		return ScopeSchool, nil
	default:
		return "", fmt.Errorf("%w: unknown scope %q", ErrInvalidPrincipal, raw)
	}
}

func validateScope(scope Scope, tenantID, organizationID int64) error {
	switch scope {
	case ScopeTenant, ScopeSchool:
		if tenantID <= 0 {
			return fmt.Errorf("%w: scope %q requires a tenant", ErrInvalidPrincipal, scope)
		}
	case ScopeOrganization:
		if tenantID <= 0 || organizationID <= 0 {
			return fmt.Errorf("%w: organization scope requires tenant and organization", ErrInvalidPrincipal)
		}
	case ScopeParent, ScopePlatform:
		if tenantID != 0 {
			return fmt.Errorf("%w: scope %q must not carry a tenant", ErrInvalidPrincipal, scope)
		}
	default:
		return fmt.Errorf("%w: unknown scope %q", ErrInvalidPrincipal, scope)
	}
	return nil
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, error) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok || principal.accountID <= 0 {
		return Principal{}, ErrPrincipalRequired
	}
	return principal, nil
}

func (p Principal) AccountID() int64      { return p.accountID }
func (p Principal) TenantID() int64       { return p.tenantID }
func (p Principal) OrganizationID() int64 { return p.organizationID }
func (p Principal) Scope() Scope          { return p.scope }
func (p Principal) FamilyID() string      { return p.familyID }
func (p Principal) Roles() []string       { return slices.Clone(p.roles) }
func (p Principal) Permissions() []string { return slices.Clone(p.permissions) }

func (p Principal) HasRole(required string) bool {
	return slices.ContainsFunc(p.roles, func(role string) bool { return strings.EqualFold(role, required) })
}

func (p Principal) HasPermission(required string) bool {
	if required == "" {
		return true
	}
	parts := strings.Split(required, ":")
	if len(parts) != 2 {
		return false
	}
	if p.hasAdminPermission() {
		return true
	}
	for _, permission := range p.permissions {
		candidate := strings.Split(permission, ":")
		if len(candidate) == 2 && matches(candidate[0], parts[0]) && matches(candidate[1], parts[1]) {
			return true
		}
	}
	return false
}

func (p Principal) HasAdminScope() bool {
	return p.admin || p.hasAdminPermission()
}

func (p Principal) hasAdminPermission() bool {
	return slices.Contains(p.permissions, "admin:*") || slices.Contains(p.permissions, "*:*")
}

func matches(pattern, required string) bool {
	if pattern == required || pattern == "*" {
		return true
	}
	return strings.HasSuffix(pattern, "*") && strings.HasPrefix(required, strings.TrimSuffix(pattern, "*"))
}
