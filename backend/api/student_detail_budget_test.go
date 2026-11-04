package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/middleware"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Drives real tenant routes through a real, account-keyed read bucket. With
// refill disabled the assertion is deterministic, independent of test speed.
func checkStudentDetailReadBudget(t *testing.T, runtime *goldenRuntime) {
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "detail-budget@example.com")
	claims := testpkg.TeacherTestClaims(int(account.ID))
	claims.Permissions = []string{"users:read"}
	token := testpkg.MintLiveSessionJWT(t, db, claims)
	auth, err := sessiontoken.NewTokenAuthWithDurations(testpkg.TestJWTSecret, 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	child := testpkg.CreateTestStudent(t, db, "Budget", "Visible", "1a")
	// The query option cannot widen portal scope. Use the production auth
	// chain, not an isolated handler with pre-injected claims.
	for _, scope := range []string{"parent", "school"} {
		foreignClaims := claims
		foreignClaims.Scope = scope
		foreignToken := testpkg.MintLiveSessionJWT(t, db, foreignClaims)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost/api/students/%d?include_privacy_consent=true", child.ID), nil)
		req.Header.Set("Authorization", "Bearer "+foreignToken)
		rr := httptest.NewRecorder()
		runtime.ServeHTTP(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code, "%s: %s", scope, rr.Body.String())
	}

	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy_%t", legacy), func(t *testing.T) {
			limiter := middleware.NewRateLimiter(0, 2)
			limiter.SetKeyFunc(auth.RateLimitKey)
			limiter.SetBucketFunc(func(*http.Request) string { return "read" })
			handler := limiter.Middleware()(runtime)
			get := func(path string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				return rr
			}
			path := "/api/students/999999999"
			if !legacy {
				path += "?include_privacy_consent=true"
			}
			require.Equal(t, http.StatusNotFound, get(path).Code)
			if legacy {
				require.Equal(t, http.StatusNotFound, get("/api/students/999999999/privacy-consent").Code)
			}
			next := get(fmt.Sprintf("/api/students/%d?include_privacy_consent=true", child.ID))
			if legacy {
				require.Equal(t, http.StatusTooManyRequests, next.Code)
			} else {
				require.Equal(t, http.StatusOK, next.Code, next.Body.String())
				require.Contains(t, next.Body.String(), `"has_full_access":false`)
			}
			require.Equal(t, http.StatusTooManyRequests, get("/api/students").Code, "aggregation must not bypass the bucket")
		})
	}
}
