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

// OperatorRejection renders an operator outcome with its registered code
// (#2519). The message is developer diagnostics; the operator portal shows
// the catalog text of the code.
func OperatorRejection(status int, code, message string) render.Renderer {
	return &ErrResponse{HTTPStatusCode: status, Status: "error", ErrorText: message, Code: code}
}

// OperatorRejectionWithDetails is OperatorRejection with the structured
// values the catalog text of the code interpolates.
func OperatorRejectionWithDetails(status int, code, message string, details map[string]any) render.Renderer {
	return &ErrResponse{HTTPStatusCode: status, Status: "error", ErrorText: message, Code: code, Details: details}
}

// OperatorInvalidRequest renders a 400 with the error's text. The field
// errors of a validation error and the code of an InputRejection in its
// chain go along, so the operator portal can mark the field (#2519).
func OperatorInvalidRequest(err error) render.Renderer {
	return ErrorInputRejection(err)
}

// OperatorInvalidRequestWithCode renders a 400 with a registered code.
func OperatorInvalidRequestWithCode(err error, code string) render.Renderer {
	resp := newErrResponse(http.StatusBadRequest, err)
	resp.Errors = validationFieldErrors(err)
	resp.Code = code
	return resp
}

// OperatorRejectionOnField is OperatorRejection that also marks the request
// field the outcome is about, e.g. the slug that is already taken (#2519).
func OperatorRejectionOnField(status int, code, field, message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: status,
		Status:         "error",
		ErrorText:      message,
		Code:           code,
		Errors:         []FieldError{{Field: field, Reason: message}},
	}
}

// OperatorInvalidField renders a 400 that marks one request field.
func OperatorInvalidField(code, field, message string) render.Renderer {
	return OperatorRejectionOnField(http.StatusBadRequest, code, field, message)
}

// OperatorInvalidInput renders a 400 with a fixed text: the cause may carry
// adapter text, so only its field errors and InputRejection code go along.
func OperatorInvalidInput(cause error, message string) render.Renderer {
	resp := &ErrResponse{HTTPStatusCode: http.StatusBadRequest, Status: "error", ErrorText: message}
	resp.Errors = validationFieldErrors(cause)
	applyInputRejection(resp, cause)
	return resp
}

// OperatorInvalidCredentials renders the 401 of a failed operator login.
func OperatorInvalidCredentials() render.Renderer {
	return OperatorRejection(http.StatusUnauthorized, CodeIdentityInvalidCredentials, "Invalid email or password")
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
