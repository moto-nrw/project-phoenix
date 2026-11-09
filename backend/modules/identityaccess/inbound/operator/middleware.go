package operator

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// RequiresOperatorScope is middleware that checks if the JWT has platform scope
// This ensures only operator tokens (not tenant tokens) can access operator routes
func RequiresOperatorScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := sessiontoken.ClaimsFromCtx(r.Context())

		if !claims.IsPlatformScope() {
			slog.WarnContext(r.Context(), "operator scope required but not present",
				slog.String("path", r.URL.Path),
			)
			common.RenderError(w, r, common.OperatorForbidden("This endpoint requires operator authentication"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequiresActiveOperator rejects stale access tokens after an operator account
// is deactivated or removed, or its refresh family ended.
func RequiresActiveOperator(lookup identityaccess.OperatorSessionLiveness) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := sessiontoken.ClaimsFromCtx(r.Context())
			if claims.ID <= 0 {
				common.RenderError(w, r, common.OperatorUnauthorized())
				return
			}
			if lookup == nil {
				slog.ErrorContext(r.Context(), "operator status lookup is not configured",
					slog.String("path", r.URL.Path),
					slog.Int("operator_id", claims.ID),
				)
				common.RenderError(w, r, common.OperatorServiceUnavailable("Operator status temporarily unavailable, please retry"))
				return
			}

			if err := lookup.VerifyOperatorSession(r.Context(), int64(claims.ID), claims.FamilyID); err != nil {
				renderOperatorLookupError(w, r, claims.ID, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func renderOperatorLookupError(w http.ResponseWriter, r *http.Request, operatorID int, err error) {
	if errors.Is(err, identityaccess.ErrOperatorInactive) || errors.Is(err, identityaccess.ErrOperatorNotFound) {
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}

	slog.ErrorContext(r.Context(), "operator status lookup failed",
		slog.String("path", r.URL.Path),
		slog.Int("operator_id", operatorID),
		slog.String("error", err.Error()),
	)
	common.RenderError(w, r, common.OperatorServiceUnavailable("Operator status temporarily unavailable, please retry"))
}
