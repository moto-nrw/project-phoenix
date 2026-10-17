package test

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantSessionGates supplies native gates and facts to consumer-owned session ports.
// These route fixtures avoid importing the root into its own dependencies.
func TenantSessionGates() (func(http.Handler) http.Handler, func(http.Handler) http.Handler, func(context.Context) bool, func(context.Context) securityruntime.PrincipalFacts) {
	return sessiontoken.Authenticator, sessiontoken.TenantMiddleware(tenant.ClaimScope{}), previewSessionFact, principalSessionFacts
}

func previewSessionFact(ctx context.Context) bool {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return claims.IsReadOnlyPreview()
}

func principalSessionFacts(ctx context.Context) securityruntime.PrincipalFacts {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return securityruntime.PrincipalFacts{
		AccountID:      int64(claims.ID),
		TenantID:       claims.TenantID,
		OrganizationID: claims.OrgID,
		Scope:          claims.Scope,
		Roles:          claims.Roles,
		Permissions:    claims.Permissions,
		Admin:          claims.IsAdmin,
		FamilyID:       claims.FamilyID,
	}
}
