package operator

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// RequiresOperatorScope is middleware that checks if the JWT has platform scope
// This ensures only operator tokens (not tenant tokens) can access operator routes
func RequiresOperatorScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := jwt.ClaimsFromCtx(r.Context())

		// Check if this is a platform scope token
		if !claims.IsPlatformScope() {
			w.WriteHeader(http.StatusForbidden)
			if err := render.Render(w, r, &ErrResponse{
				HTTPStatusCode: http.StatusForbidden,
				StatusText:     "Forbidden",
				ErrorText:      "This endpoint requires operator authentication",
			}); err != nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ErrResponse is an error response struct
type ErrResponse struct {
	HTTPStatusCode int    `json:"-"`
	StatusText     string `json:"status"`
	ErrorText      string `json:"message,omitempty"`
}

// Render implements the render.Renderer interface
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}
