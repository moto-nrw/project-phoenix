// Hermetic integration tests for the per-user conflict acknowledgements
// (#2139): GET/PUT/DELETE /conflict-acks. Mirrors instances_list_test.go —
// handlers are mounted without the JWT and TenantTx middleware; tenant and
// claims are injected via a thin wrapper so account and tenant scoping are
// exercised against the real repository.
package timetable

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const ackFingerprintA = "0123456789abcdef0123456789abcdef"
const ackFingerprintB = "fedcba9876543210fedcba9876543210"

type ackSetup struct {
	res       *Resource
	db        *bun.DB
	accountID int64
}

func buildAckSetup(t *testing.T) *ackSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("ack-%d@example.com", time.Now().UnixNano()))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			"DELETE FROM schedule.timetable_conflict_acks WHERE account_id = ?", account.ID)
	})

	res := NewResource(Dependencies{
		TimetableData: testTimetableData(db),
		DB:            db,
	})
	return &ackSetup{res: res, db: db, accountID: account.ID}
}

// ackRouter mounts the three handlers with tenant + claims injection.
func conflictAckRouter(res *Resource, tenantID, accountID int64) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/conflict-acks", res.listConflictAcks)
	r.Put("/conflict-acks/{fingerprint}", res.acknowledgeConflict)
	r.Delete("/conflict-acks/{fingerprint}", res.unacknowledgeConflict)
	return r
}

func doConflictAck(t *testing.T, router chi.Router, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeAckList(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Fingerprints []string `json:"fingerprints"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	return env.Data.Fingerprints
}

func TestConflictAcks_AcknowledgeListUnacknowledge(t *testing.T) {
	t.Parallel()

	s := buildAckSetup(t)
	router := conflictAckRouter(s.res, 1, s.accountID)

	// Initially empty (and never nil — the frontend iterates it).
	w := doConflictAck(t, router, http.MethodGet, "/conflict-acks")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, decodeAckList(t, w))

	// Acknowledge two conflicts; one twice (idempotent).
	for _, fp := range []string{ackFingerprintA, ackFingerprintB, ackFingerprintA} {
		w = doConflictAck(t, router, http.MethodPut, "/conflict-acks/"+fp)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	}
	w = doConflictAck(t, router, http.MethodGet, "/conflict-acks")
	require.Equal(t, http.StatusOK, w.Code)
	assert.ElementsMatch(t, []string{ackFingerprintA, ackFingerprintB}, decodeAckList(t, w))

	// Unacknowledge survives re-listing; removing twice stays 200.
	for range 2 {
		w = doConflictAck(t, router, http.MethodDelete, "/conflict-acks/"+ackFingerprintA)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	}
	w = doConflictAck(t, router, http.MethodGet, "/conflict-acks")
	assert.Equal(t, []string{ackFingerprintB}, decodeAckList(t, w))
}

func TestConflictAcks_InvalidFingerprintRejected(t *testing.T) {
	t.Parallel()

	s := buildAckSetup(t)
	router := conflictAckRouter(s.res, 1, s.accountID)

	for _, fp := range []string{"UPPERCASE1234567", "zz", "short", "0123456789abcde"} {
		w := doConflictAck(t, router, http.MethodPut, "/conflict-acks/"+fp)
		assert.Equal(t, http.StatusBadRequest, w.Code, "fingerprint %q must be rejected", fp)
	}
}

func TestConflictAcks_ScopedPerAccount(t *testing.T) {
	t.Parallel()

	s := buildAckSetup(t)
	other := testpkg.CreateTestAccount(t, s.db, fmt.Sprintf("ack-other-%d@example.com", time.Now().UnixNano()))

	mine := conflictAckRouter(s.res, 1, s.accountID)
	theirs := conflictAckRouter(s.res, 1, other.ID)

	w := doConflictAck(t, mine, http.MethodPut, "/conflict-acks/"+ackFingerprintA)
	require.Equal(t, http.StatusOK, w.Code)

	// The other account neither sees nor can remove my acknowledgement.
	w = doConflictAck(t, theirs, http.MethodGet, "/conflict-acks")
	assert.Empty(t, decodeAckList(t, w), "acks are per user, not per school")

	w = doConflictAck(t, theirs, http.MethodDelete, "/conflict-acks/"+ackFingerprintA)
	require.Equal(t, http.StatusOK, w.Code)
	w = doConflictAck(t, mine, http.MethodGet, "/conflict-acks")
	assert.Equal(t, []string{ackFingerprintA}, decodeAckList(t, w),
		"another account's delete must not remove my acknowledgement")
}

func TestConflictAcks_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := buildAckSetup(t)
	tenantOne := conflictAckRouter(s.res, 1, s.accountID)
	tenantOther := conflictAckRouter(s.res, 999, s.accountID)

	w := doConflictAck(t, tenantOne, http.MethodPut, "/conflict-acks/"+ackFingerprintA)
	require.Equal(t, http.StatusOK, w.Code)

	// Same account, different school: the acknowledgement is invisible.
	w = doConflictAck(t, tenantOther, http.MethodGet, "/conflict-acks")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, decodeAckList(t, w), "acks must not leak across tenants")
}

func TestConflictAcks_MissingAccountRejected(t *testing.T) {
	t.Parallel()

	s := buildAckSetup(t)
	// Claims with ID 0 — e.g. a token shape that never carried an account.
	router := conflictAckRouter(s.res, 1, 0)

	w := doConflictAck(t, router, http.MethodGet, "/conflict-acks")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}
