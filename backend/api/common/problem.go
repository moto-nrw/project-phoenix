package common

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
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
// domain-specific code yet. The numeric status remains on the HTTP response;
// the legacy JSON status string is deliberately unchanged.
func ErrorClassCode(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return CodeGeneralPermission
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
	class := errorClassByWireCode[code]
	if class == "" {
		class = strings.TrimPrefix(ErrorClassCode(status), "general.")
	}
	anchor := helpAnchorByErrorClass[class]
	return "https://moto-app.de/help/fehlermeldungen#" + anchor
}

func requestID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

// ProblemResponseMiddleware expands every API failure, including legacy
// handlers which write JSON directly or use http.Error. Successful responses
// and streaming responses pass through without buffering.
func ProblemResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture := &problemWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)
		if capture.status < 400 || capture.status > 599 {
			return
		}

		body := readProblemBody(capture, w.Header().Get("Content-Type"))
		addProblemFields(body, capture.status, middleware.GetReqID(r.Context()))
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
		setProblemString(body, "status", "error")
		setProblemString(body, "error", message)
	}
	return body
}

// Shared renderers already supplied these fields. Fill only missing members
// for legacy direct writers, preserving their values verbatim.
func addProblemFields(body map[string]json.RawMessage, status int, requestID string) {
	code := problemStringField(body, "code")
	if code == "" {
		code = ErrorClassCode(status)
		setProblemString(body, "code", code)
	}
	if problemStringField(body, "type") == "" {
		setProblemString(body, "type", problemType(code, status))
	}
	if problemStringField(body, "title") == "" {
		setProblemString(body, "title", problemTitle(status))
	}
	if problemStringField(body, "detail") == "" {
		setProblemString(body, "detail", problemDetail(body, status))
	}
	if problemStringField(body, "instance") == "" {
		setProblemString(body, "instance", requestID)
	}
}

func problemDetail(body map[string]json.RawMessage, status int) string {
	for _, key := range []string{"error", "message"} {
		if value := problemStringField(body, key); value != "" {
			return value
		}
	}
	return problemTitle(status)
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
