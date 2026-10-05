package common

import (
	"context"
	"net"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Operator error bodies. The operator router in api/operator and the operator
// handlers its owner modules serve share these constructors (#3232, #3231).
// They answer in the shared error envelope like every other surface (#2507);
// they exist for the operator surface's fixed texts and message strings.

// operatorError carries no Err: the message is client text, not a cause, so a
// 5xx neither logs it as one nor hands it to Sentry. ServerErrorReporting
// reports the status as before.
func operatorError(status int, message string) render.Renderer {
	return &ErrResponse{HTTPStatusCode: status, Status: "error", ErrorText: message}
}

// OperatorInvalidRequest renders a 400 with the error's text.
func OperatorInvalidRequest(err error) render.Renderer {
	return newErrResponse(http.StatusBadRequest, err)
}

// OperatorInvalidCredentials renders the 401 of a failed operator login.
func OperatorInvalidCredentials() render.Renderer {
	return operatorError(http.StatusUnauthorized, "Invalid email or password")
}

// OperatorUnauthorized renders the 401 of an invalid or expired token.
func OperatorUnauthorized() render.Renderer {
	return operatorError(http.StatusUnauthorized, "Unauthorized")
}

// OperatorNotFound renders a 404.
func OperatorNotFound(message string) render.Renderer {
	return operatorError(http.StatusNotFound, message)
}

// OperatorConflict renders a 409.
func OperatorConflict(message string) render.Renderer {
	return operatorError(http.StatusConflict, message)
}

// OperatorForbidden renders a 403.
func OperatorForbidden(message string) render.Renderer {
	return operatorError(http.StatusForbidden, message)
}

// OperatorTooManyRequests renders a 429.
func OperatorTooManyRequests(message string) render.Renderer {
	return operatorError(http.StatusTooManyRequests, message)
}

// OperatorInternal renders a 500.
func OperatorInternal(message string) render.Renderer {
	return operatorError(http.StatusInternalServerError, message)
}

// OperatorServiceUnavailable renders a 503. Used when a transient dependency
// makes a security decision impossible and the safe behaviour is to refuse
// this caller without globally locking everyone out.
func OperatorServiceUnavailable(message string) render.Renderer {
	return operatorError(http.StatusServiceUnavailable, message)
}

// OperatorAuditedIDAction parses one int64 id parameter and invokes an
// operator-audited action (operator ID from the JWT claims plus the client
// IP), responding 200 with no payload on success.
func OperatorAuditedIDAction(w http.ResponseWriter, r *http.Request, param, invalidMsg string, fn func(ctx context.Context, id, operatorID int64, clientIP net.IP) error, renderErr func(error) render.Renderer, successMsg string) {
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)

	id, ok := ParseInt64IDWithError(w, r, param, invalidMsg)
	if !ok {
		return
	}

	if err := fn(r.Context(), id, operatorID, ParseClientIP(r)); err != nil {
		RenderError(w, r, renderErr(err))
		return
	}

	Respond(w, r, http.StatusOK, nil, successMsg)
}
