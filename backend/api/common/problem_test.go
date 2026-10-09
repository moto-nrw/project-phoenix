package common_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/stretchr/testify/require"
)

func TestProblemResponsePreservesLegacyFieldsAndRequestID(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	request = request.WithContext(context.WithValue(request.Context(), middleware.RequestIDKey, "request-123"))
	handler := common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		common.RenderError(w, r, common.ErrorConflictWithDetails(errors.New("occupied"), "room.occupied", map[string]any{"room_id": "42"}))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, "application/problem+json", recorder.Header().Get("Content-Type"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "error", body["status"])
	require.Equal(t, "occupied", body["error"])
	require.Equal(t, "room.occupied", body["code"])
	require.Equal(t, map[string]any{"room_id": "42"}, body["details"])
	require.Equal(t, "occupied", body["detail"])
	require.Equal(t, "request-123", body["instance"])
	require.Equal(t, "https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich", body["type"])
	require.Equal(t, "Conflict", body["title"])

	// A consumer that decodes only the old contract still gets the same data.
	var legacy struct {
		Status  string         `json:"status"`
		Error   string         `json:"error"`
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &legacy))
	require.Equal(t, "error", legacy.Status)
	require.Equal(t, "occupied", legacy.Error)
	require.Equal(t, "room.occupied", legacy.Code)
	require.Equal(t, map[string]any{"room_id": "42"}, legacy.Details)
}

func TestProblemResponseCoversPlainTextErrorAndLeavesSuccessAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusBadRequest, "general.input"},
		{http.StatusForbidden, "general.permission"},
		{http.StatusConflict, "general.business_rejection"},
		{http.StatusTooManyRequests, "general.unavailable"},
		{http.StatusInternalServerError, "general.server"},
	} {
		recorder := httptest.NewRecorder()
		common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "legacy text", tc.status)
		})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))
		var body map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, tc.code, body["code"])
		require.Equal(t, "legacy text", body["error"])
		require.Equal(t, "application/problem+json", recorder.Header().Get("Content-Type"))
	}

	recorder := httptest.NewRecorder()
	common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))
	require.Equal(t, "ok", recorder.Body.String())
	require.Equal(t, "text/plain", recorder.Header().Get("Content-Type"))
}

func TestProblemTypeUsesRegisteredClassRatherThanStatusOnly(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		common.RenderError(w, r, common.ErrorConflictWithCode(errors.New("not required"), "care.announcement_ack_not_required"))
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "care.announcement_ack_not_required", body["code"])
	require.Equal(t, "https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen", body["type"])
}

func TestProblemResponsePreservesLargeLegacyNumbers(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":"error","error":"conflict","details":{"id":9007199254740993}}`))
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.JSONEq(t, `{"id":9007199254740993}`, string(body["details"]))
}

// Every error body leaves the API in the one shared envelope (#2507), whatever
// shape a handler or middleware wrote: status "error", the text in `error`,
// never `message`, and every problem member filled.
func TestProblemResponseAnswersEveryLegacyBodyInTheSharedEnvelope(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		wantError   string
		wantCode    string
		wantKept    map[string]any
	}{
		{name: "message instead of error", status: http.StatusTooManyRequests, contentType: "application/json", body: `{"status":"Too Many Requests","message":"slow down"}`, wantError: "slow down", wantCode: "general.unavailable"},
		{name: "human status text without error text", status: http.StatusForbidden, contentType: "application/json", body: `{"status":"Forbidden"}`, wantError: "Forbidden", wantCode: "general.permission"},
		{name: "success envelope on a conflict", status: http.StatusConflict, contentType: "application/json", body: `{"status":"success","data":{"status":"conflict"},"message":"Conflict detected"}`, wantError: "Conflict detected", wantCode: "general.business_rejection", wantKept: map[string]any{"data": map[string]any{"status": "conflict"}}},
		{name: "numeric legacy code", status: http.StatusUnauthorized, contentType: "application/json", body: `{"status":"error","code":401,"error":"token unauthorized"}`, wantError: "token unauthorized", wantCode: "general.permission"},
		{name: "extension members stay", status: http.StatusConflict, contentType: "application/json", body: `{"conflicts":[{"id":"1"}],"message":"companion conflict"}`, wantError: "companion conflict", wantCode: "general.business_rejection", wantKept: map[string]any{"conflicts": []any{map[string]any{"id": "1"}}}},
		{name: "plain text", status: http.StatusNotFound, contentType: "text/plain", body: "not found", wantError: "not found", wantCode: "general.input"},
		{name: "empty body", status: http.StatusInternalServerError, wantError: "Internal Server Error", wantCode: "general.server"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			request = request.WithContext(context.WithValue(request.Context(), middleware.RequestIDKey, "request-7"))
			recorder := httptest.NewRecorder()
			common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})).ServeHTTP(recorder, request)

			require.Equal(t, tc.status, recorder.Code)
			require.Empty(t, routetest.ProblemEnvelopeViolations(recorder.Body.Bytes(), true), recorder.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, tc.wantError, body["error"])
			require.Equal(t, tc.wantError, body["detail"])
			require.Equal(t, tc.wantCode, body["code"])
			for member, want := range tc.wantKept {
				require.Equal(t, want, body[member], member)
			}
		})
	}
}

func TestErrorClassCodeNamesAnOversizedRequest(t *testing.T) {
	t.Parallel()

	require.Equal(t, common.CodeGeneralRequestTooLarge, common.ErrorClassCode(http.StatusRequestHeaderFieldsTooLarge))
	require.Equal(t, common.CodeGeneralInput, common.ErrorClassCode(http.StatusBadRequest))
}
