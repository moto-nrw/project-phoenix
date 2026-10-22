package common

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

const problemContentType = "application/problem+json"

var helpAnchorByErrorClass = map[string]string{
	"input":              "anleitung-eingabe-pruefen",
	"permission":         "anleitung-zugriff-pruefen",
	"business_rejection": "anleitung-vorgang-nicht-moeglich",
	"unavailable":        "anleitung-gerade-nicht-erreichbar",
	"server":             "anleitung-unerwarteter-fehler",
}

// ErrorClassCode supplies a stable class identity where a handler has no
// domain-specific code yet. Every class code is registered, so an answer
// without a code of its own still carries a registry code (#2503).
func ErrorClassCode(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return CodeGeneralPermission
	case http.StatusRequestEntityTooLarge, http.StatusRequestHeaderFieldsTooLarge:
		return CodeGeneralRequestTooLarge
	case http.StatusConflict, http.StatusGone, http.StatusUnprocessableEntity:
		return CodeGeneralBusinessRejection
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 499:
		return CodeGeneralUnavailable
	default:
		if status >= 500 {
			return CodeGeneralServer
		}
		return CodeGeneralInput
	}
}

func problemTitle(status int) string {
	if status == 499 {
		return "Client Closed Request"
	}
	return http.StatusText(status)
}

func problemType(code string, status int) string {
	class := errorClassByCode[code]
	if class == "" {
		class = strings.TrimPrefix(ErrorClassCode(status), "general.")
	}
	anchor := helpAnchorByErrorClass[class]
	return "https://moto-app.de/help/fehlermeldungen#" + anchor
}

func requestID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

// ProblemResponseMiddleware answers every API failure in the one shared error
// envelope (ADR 0006, #2507), including handlers which write JSON directly or
// use http.Error. Successful responses and streaming responses pass through
// without buffering.
func ProblemResponseMiddleware(next http.Handler) http.Handler {
	return problemResponse(slog.Default)(next)
}

// problemResponse is ProblemResponseMiddleware with the logger that records a
// repaired body.
func problemResponse(logger func() *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return problemResponseHandler(next, logger)
	}
}

func problemResponseHandler(next http.Handler, logger func() *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture := &problemWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)
		if capture.status < 400 || capture.status > 599 {
			return
		}

		wroteBody := len(bytes.TrimSpace(capture.body.Bytes())) > 0
		body := readProblemBody(capture, w.Header().Get("Content-Type"))
		repaired := wroteBody && departsFromEnvelope(body, capture.status)
		addProblemFields(body, capture.status, middleware.GetReqID(r.Context()))
		if repaired {
			// A handler wrote its own error body. The answer is repaired, but
			// that is a regression of the one error path (#2520): it is logged
			// at error level with the route and the code it now answers with.
			// A status without a body is the documented way to let this
			// middleware answer and is not logged.
			logger().ErrorContext(r.Context(), "error body outside the shared envelope",
				slog.String("route", RoutePattern(r)),
				slog.Int("status", capture.status),
				slog.String("code", problemStringField(body, "code")),
			)
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			slog.Default().ErrorContext(r.Context(), "failed to encode problem response", slog.String("error", err.Error()))
			return
		}
		w.Header().Del("Content-Length")
		w.Header().Del("Content-Encoding")
		w.Header().Set("Content-Type", problemContentType)
		w.WriteHeader(capture.status)
		_, _ = w.Write(append(encoded, '\n'))
	})
}

func readProblemBody(capture *problemWriter, contentType string) map[string]json.RawMessage {
	body := map[string]json.RawMessage{}
	if strings.Contains(contentType, "json") {
		if err := json.Unmarshal(capture.body.Bytes(), &body); err != nil || body == nil {
			body = map[string]json.RawMessage{}
		}
	}
	if len(body) == 0 {
		message := strings.TrimSpace(capture.body.String())
		if message == "" {
			message = problemTitle(capture.status)
		}
		setProblemString(body, "detail", message)
	}
	return body
}

// departsFromEnvelope reports a body that is not the shared envelope yet:
// a legacy `error` or `message` text, a status member other than the HTTP
// status number, or no code.
func departsFromEnvelope(body map[string]json.RawMessage, status int) bool {
	_, hasMessage := body["message"]
	_, hasError := body["error"]
	return hasMessage || hasError || problemStatusField(body) != status || problemStringField(body, "code") == ""
}

// addProblemFields completes the RFC 9457 envelope. Shared renderers already
// supplied every member. A direct writer's text moves from `error` or
// `message` into `detail`, its status member becomes the HTTP status number
// and a missing or unregistered code becomes the class code of the status.
// Further members stay as RFC 9457 extensions.
func addProblemFields(body map[string]json.RawMessage, status int, requestID string) {
	if problemStringField(body, "detail") == "" {
		setProblemString(body, "detail", problemDetail(body, status))
	}
	delete(body, "error")
	delete(body, "message")
	body["status"] = json.RawMessage(strconv.Itoa(status))
	code := problemStringField(body, "code")
	codeReplaced := false
	if _, registered := errorClassByCode[code]; !registered {
		code = ErrorClassCode(status)
		setProblemString(body, "code", code)
		codeReplaced = true
	}
	if codeReplaced || problemStringField(body, "type") == "" {
		setProblemString(body, "type", problemType(code, status))
	}
	if problemStringField(body, "title") == "" {
		setProblemString(body, "title", problemTitle(status))
	}
	if problemStringField(body, "instance") == "" {
		setProblemString(body, "instance", requestID)
	}
}

func problemDetail(body map[string]json.RawMessage, status int) string {
	for _, key := range []string{"detail", "error", "message"} {
		if value := problemStringField(body, key); value != "" {
			return value
		}
	}
	return problemTitle(status)
}

func problemStatusField(body map[string]json.RawMessage) int {
	var value int
	if err := json.Unmarshal(body["status"], &value); err != nil {
		return 0
	}
	return value
}

func problemStringField(body map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(body[key], &value)
	return value
}

func setProblemString(body map[string]json.RawMessage, key, value string) {
	encoded, _ := json.Marshal(value)
	body[key] = encoded
}

type problemWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *problemWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if isInterimStatus(status) {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	if status < 400 || status > 599 {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *problemWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status >= 400 && w.status <= 599 {
		return w.body.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *problemWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status < 400 || w.status > 599 {
		if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
