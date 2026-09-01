package api

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LoginOperator: MFA enrollment-required branch
// =============================================================================

func TestLoginOperator_EnrollmentRequired_Success(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()

	var (
		startHits   int32
		confirmHits int32
	)
	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			w.WriteHeader(testpkg.HTTPStatusNoContent)
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
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.NoError(t, err)
	assert.Equal(t, AuthBearer, auth.Kind)
	assert.Equal(t, "operator", auth.Label)
	assert.Equal(t, "final-session-jwt", auth.Token, "must swap the pending token for the post-enrollment one")
	assert.Equal(t, int32(1), atomic.LoadInt32(&startHits), "enroll/start must be hit exactly once")
	assert.Equal(t, int32(1), atomic.LoadInt32(&confirmHits), "enroll/confirm must be hit exactly once")
}

func TestLoginOperator_EnrollmentRequired_StatusFieldOnlyTriggersFlow(t *testing.T) {
	t.Parallel()

	// Server emits `status` only, not the explicit boolean — the client
	// should still recognise the enrollment branch.
	mailpit := newCodeMailpitServer(t, "op@test.de", "111111")
	defer mailpit.Close()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			w.WriteHeader(testpkg.HTTPStatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{"access_token": "final"},
			})
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.NoError(t, err)
	assert.Equal(t, "final", auth.Token)
}

func TestLoginOperator_EnrollStartFails(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			w.WriteHeader(testpkg.HTTPStatusInternalServerError)
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enroll start")
}

func TestLoginOperator_EnrollConfirmFails(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			w.WriteHeader(testpkg.HTTPStatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			w.WriteHeader(testpkg.HTTPStatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"Code invalid"}`))
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enroll confirm")
}

func TestLoginOperator_EnrollConfirm_NoToken(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "424242")
	defer mailpit.Close()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			w.WriteHeader(testpkg.HTTPStatusNoContent)
		case "/operator/auth/mfa/enroll/confirm":
			// Successful HTTP but no token in body.
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

// =============================================================================
// LoginOperator: MFA verify-required branch
// =============================================================================

func TestLoginOperator_VerifyRequired_Success(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "888888")
	defer mailpit.Close()

	var verifyHits int32
	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.NoError(t, err)
	assert.Equal(t, "verified-jwt", auth.Token)
	assert.Equal(t, int32(1), atomic.LoadInt32(&verifyHits))
}

func TestLoginOperator_VerifyFails(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "888888")
	defer mailpit.Close()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"challenge_token": "ch", "status": "mfa_required"},
			})
		case "/operator/auth/mfa/verify":
			w.WriteHeader(testpkg.HTTPStatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"locked"}`))
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mfa verify")
}

func TestLoginOperator_Verify_NoToken(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "op@test.de", "888888")
	defer mailpit.Close()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"challenge_token": "ch"},
			})
		case "/operator/auth/mfa/verify":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, mailpit.URL)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

func TestLoginOperator_MailpitDown_EnrollmentSurfaces(t *testing.T) {
	t.Parallel()

	// Point mailpit at a closed port so polling fails immediately on
	// every iteration. The enrollment flow then surfaces a timeout
	// error after the ctx deadline expires.

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			w.WriteHeader(testpkg.HTTPStatusNoContent)
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	a := NewCommandAdapter(api.URL, false)
	useMailpit(a, "http://127.0.0.1:1")
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

func useMailpit(adapter *Adapter, baseURL string) {
	adapter.fetchLatestMFACode = func(ctx context.Context, recipient string, notBefore time.Time) (string, error) {
		return fetchLatestMFACodeAt(ctx, baseURL, recipient, notBefore)
	}
}

// newCodeMailpitServer returns a mailpit fake that always reports a
// single message addressed to recipient containing code.
func newCodeMailpitServer(t *testing.T, recipient, code string) *testpkg.HTTPServer {
	t.Helper()
	now := time.Now()
	id := "msg-" + code
	return testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
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
			testpkg.HTTPNotFound(w, r)
		}
	})
}
