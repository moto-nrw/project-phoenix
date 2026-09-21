package tenant

import "context"

// ClaimScope binds the scope of an authenticated token to the request context.
// The token adapter decides which scope a route accepts; the tenant runtime
// owns what a bound tenant, organization and scope mean.
type ClaimScope struct{}

// BindTenant pins the request to one school. An invalid tenant ID is reported
// to the entry-point observer and returned, so the caller rejects the request.
func (ClaimScope) BindTenant(ctx context.Context, tenantID, orgID int64, scope string) (context.Context, error) {
	id, err := NewTenantID(tenantID)
	if err != nil {
		ObserveMissingTenant(ctx, err)
		return ctx, err
	}
	return WithScope(WithOrgID(WithTenant(ctx, id), orgID), scope), nil
}

// BindOrganization serves platform tokens, which carry no tenant.
func (ClaimScope) BindOrganization(ctx context.Context, orgID int64, scope string) context.Context {
	return WithScope(WithOrgID(ctx, orgID), scope)
}

// BindScope serves cross-tenant tokens: no tenant and no organization is bound.
func (ClaimScope) BindScope(ctx context.Context, scope string) context.Context {
	return WithScope(ctx, scope)
}
