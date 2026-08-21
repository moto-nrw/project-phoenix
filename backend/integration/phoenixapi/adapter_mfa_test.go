package phoenixapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LoginOperator: MFA enrollment-required branch
// =============================================================================

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_EnrollmentRequired_Success(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	var (
		startHits   int32
		confirmHits int32
	)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"status":                  "mfa_enrollment_required",
					"access_token":            "pending-enrollment-jwt",
					"mfa_enrollment_required": true,
				},
			})
		case "/operator/auth/mfa/enroll/start":
			atomic.AddInt32(&startHits, 1)
			assert.Equal(t, "Bearer pending-enrollment-jwt", r.Header.Get("Authorization"),
				"enroll/start must be called with the pending enrollment token")
			w.WriteHeader(http.StatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			atomic.AddInt32(&confirmHits, 1)
			assert.Equal(t, "Bearer pending-enrollment-jwt", r.Header.Get("Authorization"),
				"enroll/confirm must be called with the pending enrollment token")
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, "424242", payload["code"], "the code from mailpit must be submitted verbatim")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]string{"access_token": "final-session-jwt"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.NoError(t, err)
	assert.Equal(t, AuthBearer, auth.Kind)
	assert.Equal(t, "operator", auth.Label)
	assert.Equal(t, "final-session-jwt", auth.Token, "must swap the pending token for the post-enrollment one")
	assert.Equal(t, int32(1), atomic.LoadInt32(&startHits), "enroll/start must be hit exactly once")
	assert.Equal(t, int32(1), atomic.LoadInt32(&confirmHits), "enroll/confirm must be hit exactly once")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_EnrollmentRequired_StatusFieldOnlyTriggersFlow(t *testing.T) {
	// Server emits `status` only, not the explicit boolean — the client
	// should still recognise the enrollment branch.
	mailpit := newCodeMailpitServer(t, "op@test.de", "111111")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"status":       "mfa_enrollment_required",
					"access_token": "pending",
				},
			})
		case "/operator/auth/mfa/enroll/start":
			w.WriteHeader(http.StatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{"access_token": "final"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.NoError(t, err)
	assert.Equal(t, "final", auth.Token)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_EnrollStartFails(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"access_token":            "pending",
					"mfa_enrollment_required": true,
				},
			})
		case "/operator/auth/mfa/enroll/start":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enroll start")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_EnrollConfirmFails(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"access_token":            "pending",
					"mfa_enrollment_required": true,
				},
			})
		case "/operator/auth/mfa/enroll/start":
			w.WriteHeader(http.StatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"Code invalid"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enroll confirm")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_EnrollConfirm_NoToken(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"access_token":            "pending",
					"mfa_enrollment_required": true,
				},
			})
		case "/operator/auth/mfa/enroll/start":
			w.WriteHeader(http.StatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			// Successful HTTP but no token in body.
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

// =============================================================================
// LoginOperator: MFA verify-required branch
// =============================================================================

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_VerifyRequired_Success(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "888888")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	var verifyHits int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			// Already-enrolled operator → mfa_required with a challenge
			// token and NO access token (server-side flow doesn't issue
			// a pending JWT here).
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"status":          "mfa_required",
					"challenge_token": "challenge-xyz",
				},
			})
		case "/operator/auth/mfa/verify":
			atomic.AddInt32(&verifyHits, 1)
			assert.Empty(t, r.Header.Get("Authorization"),
				"verify must be called without a bearer token — the challenge_token in the body is the proof")
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				ChallengeToken string `json:"challenge_token"`
				Code           string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, "challenge-xyz", payload.ChallengeToken)
			assert.Equal(t, "888888", payload.Code)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{"access_token": "verified-jwt"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.NoError(t, err)
	assert.Equal(t, "verified-jwt", auth.Token)
	assert.Equal(t, int32(1), atomic.LoadInt32(&verifyHits))
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_VerifyFails(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "888888")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"challenge_token": "ch", "status": "mfa_required"},
			})
		case "/operator/auth/mfa/verify":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"locked"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mfa verify")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_Verify_NoToken(t *testing.T) {
	mailpit := newCodeMailpitServer(t, "op@test.de", "888888")
	defer mailpit.Close()
	t.Setenv("MAILPIT_URL", mailpit.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"challenge_token": "ch"},
			})
		case "/operator/auth/mfa/verify":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestLoginOperator_MailpitDown_EnrollmentSurfaces(t *testing.T) {
	// Point mailpit at a closed port so polling fails immediately on
	// every iteration. The enrollment flow then surfaces a timeout
	// error after the ctx deadline expires.
	t.Setenv("MAILPIT_URL", "http://127.0.0.1:1")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"access_token":            "pending",
					"mfa_enrollment_required": true,
				},
			})
		case "/operator/auth/mfa/enroll/start":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	a := New(api.URL, false)
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	_, err := a.LoginOperator(ctx, "op@test.de", "secret")
	require.Error(t, err)
	// The error should surface as "complete operator mfa enrollment: ..."
	// with the context-deadline reason from FetchLatestMFACode.
	assert.Contains(t, err.Error(), "complete operator mfa enrollment")
}

// =============================================================================
// Test helpers
// =============================================================================

// newCodeMailpitServer returns a mailpit fake that always reports a
// single message addressed to recipient containing code.
func newCodeMailpitServer(t *testing.T, recipient, code string) *httptest.Server {
	t.Helper()
	now := time.Now()
	id := "msg-" + code
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/messages":
			_ = json.NewEncoder(w).Encode(mailpitMessageList{
				Total: 1,
				Messages: []mailpitMessageSummary{
					{ID: id, To: []mailpitAddress{{Address: recipient}}, Created: now},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/message/"):
			_ = json.NewEncoder(w).Encode(mailpitMessageDetail{ID: id, Text: "Ihr Code: " + code})
		default:
			http.NotFound(w, r)
		}
	}))
}
