package jwt

import "context"

// TenantScope binds verified claims to the tenant runtime. The tenant runtime
// implements it; each HTTP root passes that implementation to the scope
// middlewares below, so this adapter decides only which scope a route accepts.
type TenantScope interface {
	BindTenant(ctx context.Context, tenantID, orgID int64, scope string) (context.Context, error)
	BindOrganization(ctx context.Context, orgID int64, scope string) context.Context
	BindScope(ctx context.Context, scope string) context.Context
}

// Token scopes as they travel in the scope claim.
const (
	claimScopePlatform = "platform"
	claimScopeParent   = "parent"
	claimScopeSchool   = "school"
)
