package authorize

import (
	"net/http"
	"slices"

	"github.com/moto-nrw/project-phoenix/auth/jwt"

	"github.com/go-chi/render"
)

// RequiresRole middleware restricts access to accounts having role parameter in their jwt claims.
func RequiresRole(role string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			claims := jwt.ClaimsFromCtx(r.Context())
			if !hasRole(role, claims.Roles) {
				if renderErr := render.Render(w, r, ErrForbidden); renderErr != nil {
					// Error already occurred while sending the response
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				}
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

func hasRole(role string, roles []string) bool {
	return slices.Contains(roles, role)
}
