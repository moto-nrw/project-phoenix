package inbound

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	substitutionsHTTP "github.com/moto-nrw/project-phoenix/api/substitutions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewSubstitutionsResource wires /api/substitutions over the substitution
// operations capability.
func NewSubstitutionsResource(substitutions workforce.Substitutions, db *bun.DB) *substitutionsHTTP.Resource {
	if substitutions == nil || db == nil {
		panic("substitutions HTTP composition: all dependencies are required")
	}
	return substitutionsHTTP.NewResource(substitutions, substitutionsRuntime(db))
}

func substitutionsRuntime(db *bun.DB) substitutionsHTTP.Runtime {
	return substitutionsHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, substitutionsHTTP.Middleware)) {
			common.ProtectedTenantGroup(router, db, routes)
		},
		Caller:  substitutionCaller,
		Success: common.Respond,
		Failure: renderSubstitutionsFailure,
	}
}

// substitutionCaller turns the request principal into the capability's
// caller. A principal of another tenant than the request's is forbidden.
func substitutionCaller(ctx context.Context) (workforce.SubstitutionCaller, error) {
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil || principal.TenantID() != tenant.FromContext(ctx) {
		return workforce.SubstitutionCaller{}, workforce.ErrSubstitutionForbidden
	}
	return workforce.SubstitutionCaller{
		AccountID: principal.AccountID(), TenantID: principal.TenantID(), Scope: string(principal.Scope()),
		Roles: principal.Roles(), Admin: principal.HasAdminScope(), HasPermission: principal.HasPermission,
	}, nil
}

// renderSubstitutionsFailure renders the stable status, code and message the
// adapter classified; the underlying error only reaches the log.
func renderSubstitutionsFailure(w http.ResponseWriter, r *http.Request, failure substitutionsHTTP.Failure) {
	common.RenderError(w, r, &common.ErrResponse{
		Err: failure.Err, HTTPStatusCode: failure.Status, Status: "error", ErrorText: failure.Message, Code: failure.Code,
	})
}
