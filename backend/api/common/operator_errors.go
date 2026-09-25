package common

import (
	"context"
	"net"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Operator error bodies. The operator surface keeps its own wire format
// (`message` instead of `error`, and a literal "error" status for most
// outcomes), pinned by operator_wire_format_test.go. The operator router in
// api/operator and the operator handlers its owner modules serve share these
// constructors, so the surface keeps one wire format wherever a handler lives
// (#3232, #3231).

// OperatorErrResponse is the operator surface's error body.
type OperatorErrResponse struct {
	HTTPStatusCode int    `json:"-"`
	StatusText     string `json:"status"`
	ErrorText      string `json:"message,omitempty"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Detail         string `json:"detail"`
	Instance       string `json:"instance"`
	Code           string `json:"code"`
}

// Render implements render.Renderer.
func (e *OperatorErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	e.Code = ErrorClassCode(e.HTTPStatusCode)
	e.Type = problemType(e.Code, e.HTTPStatusCode)
	e.Title = problemTitle(e.HTTPStatusCode)
	e.Detail = e.ErrorText
	e.Instance = requestID(r)
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func operatorError(status int, statusText, message string) render.Renderer {
	return &OperatorErrResponse{HTTPStatusCode: status, StatusText: statusText, ErrorText: message}
}

// OperatorInvalidRequest renders a 400 with the error's text.
func OperatorInvalidRequest(err error) render.Renderer {
	return operatorError(http.StatusBadRequest, "error", err.Error())
}

// OperatorInvalidCredentials renders the 401 of a failed operator login.
func OperatorInvalidCredentials() render.Renderer {
	return operatorError(http.StatusUnauthorized, "error", "Invalid email or password")
}

// OperatorUnauthorized renders the 401 of an invalid or expired token.
func OperatorUnauthorized() render.Renderer {
	return operatorError(http.StatusUnauthorized, "error", "Unauthorized")
}

// OperatorNotFound renders a 404.
func OperatorNotFound(message string) render.Renderer {
	return operatorError(http.StatusNotFound, "error", message)
}

// OperatorConflict renders a 409.
func OperatorConflict(message string) render.Renderer {
	return operatorError(http.StatusConflict, "error", message)
}

// OperatorForbidden renders a 403.
func OperatorForbidden(message string) render.Renderer {
	return operatorError(http.StatusForbidden, "error", message)
}

// OperatorTooManyRequests renders a 429.
func OperatorTooManyRequests(message string) render.Renderer {
	return operatorError(http.StatusTooManyRequests, "Too Many Requests", message)
}

// OperatorInternal renders a 500.
func OperatorInternal(message string) render.Renderer {
	return operatorError(http.StatusInternalServerError, "error", message)
}

// OperatorServiceUnavailable renders a 503. Used when a transient dependency
// makes a security decision impossible and the safe behaviour is to refuse
// this caller without globally locking everyone out.
func OperatorServiceUnavailable(message string) render.Renderer {
	return operatorError(http.StatusServiceUnavailable, "Service Unavailable", message)
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
