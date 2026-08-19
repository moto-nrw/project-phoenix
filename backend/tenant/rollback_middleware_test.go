package tenant_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTenantTxMiddleware_MarkRollback proves the end-to-end contract of
// tenant.MarkRollback: a handler can render a non-5xx response and STILL have
// the enclosing tenant transaction rolled back, so its writes never persist.
// This backs the guardian full-delete 409 path (#819), where the handler must
// reject with a 409 without committing any partial link deletions.
func TestTenantTxMiddleware_MarkRollback(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile

	newProfile := func() *users.GuardianProfile {
		email := fmt.Sprintf("rollback-mw-%d@test.local", time.Now().UnixNano())
		return &users.GuardianProfile{
			FirstName:              "Rollback",
			LastName:               "Marker",
			Email:                  &email,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}
	}

	serve := func(t *testing.T, handler http.HandlerFunc) {
		t.Helper()
		mw := tenant.TenantTxMiddleware(db)(handler)
		req := httptest.NewRequest(http.MethodPost, "/api/guardians", nil)
		req = req.WithContext(tenant.WithTenantID(req.Context(), 1))
		mw.ServeHTTP(httptest.NewRecorder(), req)
	}

	t.Run("MarkRollback discards the write despite a 200 response", func(t *testing.T) {
		profile := newProfile()
		var rec *httptest.ResponseRecorder

		mw := tenant.TenantTxMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Write inside the request-scoped tenant tx.
			require.NoError(t, repo.Create(r.Context(), profile))
			require.Greater(t, profile.ID, int64(0), "insert must assign an id inside the tx")
			// Ask for a rollback, then return a perfectly successful status.
			tenant.MarkRollback(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodPost, "/api/guardians", nil)
		req = req.WithContext(tenant.WithTenantID(req.Context(), 1))
		rec = httptest.NewRecorder()
		mw.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "the client still sees the 200")

		// The row must NOT survive: the tx was rolled back.
		_, err := repo.FindByID(testpkg.TenantContext(1), profile.ID)
		assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound,
			"MarkRollback must prevent the insert from committing")
	})

	t.Run("without MarkRollback the same write commits", func(t *testing.T) {
		// Contrast case: proves the rollback above is caused by MarkRollback and
		// not by some unrelated failure to persist.
		profile := newProfile()
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, repo.Create(r.Context(), profile))
			w.WriteHeader(http.StatusOK)
		})
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		found, err := repo.FindByID(testpkg.TenantContext(1), profile.ID)
		require.NoError(t, err)
		assert.Equal(t, profile.ID, found.ID, "a normal 200 commits the write")
	})
}
