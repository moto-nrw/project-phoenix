package account_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPasswordResetRateLimitKeepsUnknownAndKnownResponsesEqual(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := setupAuthModuleOn(t, db, api.WithAuthTestPasswordResetRateLimit(true), api.WithAuthTestMailer(testpkg.NewCapturingMailer()), api.WithAuthTestPasswordResetBackoff(time.Millisecond))
	settings := api.NewAuthTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	resource := authAPI.NewResource(module.Auth, module.Invitation, testSchoolDirectory{schools: api.NewOrganizationTestRows(testpkg.NewDatabase(t, db)).School, db: db, runtime: testpkg.TenantRuntime(t, db)}, module.AccountAuthentication, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, settings)

	router := testutil.NewTenantRouter(db)
	router.Mount("/auth", resource.Router(api.TenantSessionChain))
	account := testpkg.CreateTestAccount(t, db, "reset-wire-privacy")
	for _, address := range []string{account.Email, fmt.Sprintf("no-account-%d@example.test", testpkg.Tenant(t))} {

		for range 3 {
			req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/password-reset", map[string]string{"email": address})
			rr := testutil.ExecuteRequest(router, req)
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		}
		req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/password-reset", map[string]string{"email": address})
		rr := testutil.ExecuteRequest(router, req)
		require.Equal(t, http.StatusTooManyRequests, rr.Code, rr.Body.String())
		require.Contains(t, rr.Body.String(), "identity.password_reset_rate_limited")
		seconds, err := strconv.Atoi(rr.Header().Get("Retry-After"))
		require.NoError(t, err)
		require.Positive(t, seconds)
		require.LessOrEqual(t, seconds, 3600)
	}
}
