package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	substitutionsHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/substitutions"
)

// newSubstitutionsResource wires /api/substitutions over the substitution
// operations capability.
func newSubstitutionsResource(substitutions workforce.Substitutions) *substitutionsHTTP.Resource {
	if substitutions == nil {
		panic("substitutions HTTP composition: all dependencies are required")
	}
	return substitutionsHTTP.NewResource(substitutions, substitutionsRuntime())
}

func substitutionsRuntime() substitutionsHTTP.Runtime {
	return substitutionsHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, substitutionsHTTP.Middleware)) {
			ProtectedTenantRoutes(router, routes)
		},
		Caller:  substitutionsHTTP.RequestCaller,
		Success: common.Respond,
		Failure: renderSubstitutionsFailure,
	}
}

// renderSubstitutionsFailure renders the stable status, code and message the
// adapter classified; the underlying error only reaches the log.
func renderSubstitutionsFailure(w http.ResponseWriter, r *http.Request, failure substitutionsHTTP.Failure) {
	resp := &common.ErrResponse{
		Err: failure.Err, HTTPStatusCode: failure.Status, Detail: failure.Message, Code: failure.Code,
		Details: failure.Details,
	}
	if failure.Field != "" {
		resp.Errors = []common.FieldError{{Field: failure.Field, Reason: failure.Message}}
	}
	common.RenderError(w, r, resp)
}
