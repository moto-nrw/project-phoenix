package api

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// SecurityPrincipalMiddleware maps native authenticated claims to the security
// facts consumed by the HTTP guard and the validated authorization contract.
func SecurityPrincipalMiddleware(next http.Handler) http.Handler {
	return common.PrincipalFromFacts(principalSessionFacts)(next)
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
