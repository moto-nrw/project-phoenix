package operator

import (
	"context"
	"net"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// idList parses one int64 id parameter, fetches a payload via fn, and
// responds 200 with the payload. Errors are rendered via renderErr.
//
// Thin inline-shape adapter over common.IDFetch (issue #575 B2): operator
// handlers are named methods that internal tests invoke directly, so the
// call sites keep the (w, r, ...) shape instead of switching to the
// HandlerFunc-factory style.
func idList[T any](w http.ResponseWriter, r *http.Request, param, invalidMsg string, fn func(ctx context.Context, id int64) (T, error), renderErr func(error) render.Renderer, successMsg string) {
	common.IDFetch(param, invalidMsg, fn, renderErr, successMsg)(w, r)
}

// idAuditedAction parses one int64 id parameter and invokes an
// operator-audited action (operator ID from JWT claims + client IP),
// responding 200 with no payload on success.
func idAuditedAction(w http.ResponseWriter, r *http.Request, param, invalidMsg string, fn func(ctx context.Context, id, operatorID int64, clientIP net.IP) error, renderErr func(error) render.Renderer, successMsg string) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	id, ok := common.ParseInt64IDWithError(w, r, param, invalidMsg)
	if !ok {
		return
	}

	clientIP := getClientIP(r)

	if err := fn(r.Context(), id, operatorID, clientIP); err != nil {
		common.RenderError(w, r, renderErr(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, successMsg)
}
