package operator

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// OperatorLookup reloads the current platform operator state and refresh-token
// family for protected operator routes.
type OperatorLookup interface {
	FindOperator(ctx context.Context, id int64) (Operator, error)
	LatestOperatorSessionInFamily(ctx context.Context, familyID string) (OperatorSession, error)
}

// RequiresOperatorScope is middleware that checks if the JWT has platform scope
// This ensures only operator tokens (not tenant tokens) can access operator routes
func RequiresOperatorScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := sessiontoken.ClaimsFromCtx(r.Context())

		// Check if this is a platform scope token
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
// is deactivated or removed.
func RequiresActiveOperator(lookup OperatorLookup) func(http.Handler) http.Handler {
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

			if err := validateActiveOperatorSession(r.Context(), lookup, int64(claims.ID), claims.FamilyID); err != nil {
				renderOperatorLookupError(w, r, claims.ID, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validateActiveOperatorSession(ctx context.Context, lookup OperatorLookup, operatorID int64, familyID string) error {
	operator, err := lookup.FindOperator(ctx, operatorID)
	if err != nil {
		return err
	}
	if !operator.Active || familyID == "" {
		return ErrOperatorInactive
	}
	session, err := lookup.LatestOperatorSessionInFamily(ctx, familyID)
	if errors.Is(err, ErrOperatorSessionNotFound) {
		return ErrOperatorInactive
	}
	if err != nil {
		return err
	}
	if session.OperatorID != operatorID || !session.Expiry.After(time.Now()) {
		return ErrOperatorInactive
	}
	return nil
}

func renderOperatorLookupError(w http.ResponseWriter, r *http.Request, operatorID int, err error) {
	if errors.Is(err, ErrOperatorInactive) || errors.Is(err, ErrOperatorNotFound) {
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
