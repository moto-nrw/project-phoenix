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
		common.RenderError(w, r, common.ErrorConflictWithCode(errors.New("not required"), "announcement_ack_not_required"))
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "announcement_ack_not_required", body["code"])
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
