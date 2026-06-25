package operator

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// OperatorLookup reloads the current platform operator state for protected
// operator routes.
type OperatorLookup interface {
	GetOperator(ctx context.Context, id int64) (*platformModels.Operator, error)
}

// RequiresOperatorScope is middleware that checks if the JWT has platform scope
// This ensures only operator tokens (not tenant tokens) can access operator routes
func RequiresOperatorScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := jwt.ClaimsFromCtx(r.Context())

		// Check if this is a platform scope token
		if !claims.IsPlatformScope() {
			slog.WarnContext(r.Context(), "operator scope required but not present",
				slog.String("path", r.URL.Path),
			)
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

// RequiresActiveOperator rejects stale access tokens after an operator account
// is deactivated or removed.
func RequiresActiveOperator(lookup OperatorLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := jwt.ClaimsFromCtx(r.Context())
			if claims.ID <= 0 {
				common.RenderError(w, r, ErrUnauthorized())
				return
			}
			if lookup == nil {
				slog.ErrorContext(r.Context(), "operator status lookup is not configured",
					slog.String("path", r.URL.Path),
					slog.Int("operator_id", claims.ID),
				)
				common.RenderError(w, r, ErrServiceUnavailable("Operator status temporarily unavailable, please retry"))
				return
			}

			operator, err := lookup.GetOperator(r.Context(), int64(claims.ID))
			if err != nil {
				renderOperatorLookupError(w, r, claims.ID, err)
				return
			}
			if operator == nil {
				common.RenderError(w, r, AuthErrorRenderer(&platformSvc.OperatorNotFoundError{
					OperatorID: int64(claims.ID),
				}))
				return
			}
			if !operator.Active {
				common.RenderError(w, r, AuthErrorRenderer(&platformSvc.OperatorInactiveError{
					OperatorID: operator.ID,
				}))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func renderOperatorLookupError(w http.ResponseWriter, r *http.Request, operatorID int, err error) {
	var inactive *platformSvc.OperatorInactiveError
	var notFound *platformSvc.OperatorNotFoundError
	if errors.As(err, &inactive) || errors.As(err, &notFound) {
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}

	slog.ErrorContext(r.Context(), "operator status lookup failed",
		slog.String("path", r.URL.Path),
		slog.Int("operator_id", operatorID),
		slog.String("error", err.Error()),
	)
	common.RenderError(w, r, ErrServiceUnavailable("Operator status temporarily unavailable, please retry"))
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
