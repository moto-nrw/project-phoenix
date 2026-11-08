package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	uploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	failures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentDeletionRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newDeletionTestFixture(t, db, "growth")
	resource := enrollmentHTTP.NewResource(nil, nil, nil, nil, nil, nil, nil, nil, nil, f.service(nil, nil), enrollmentHTTP.GuardianInvitationRuntime{}, nil, nil, uploads.UploadFiles{}, enrollmentHTTP.DatabaseFailures{Transient: failures.IsTransientDatabaseError, Constraint: failures.IsConstraintViolation})
	router := testpkg.TenantRuntimeMiddleware(t, db)(resource.Router(api.TenantSessionChain))
	account := testpkg.CreateTestAccount(t, db, "deletion-growth-reader")
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Username: account.Email, TenantID: f.scope.TenantID, Roles: []string{"admin"}, Permissions: []string{"config:manage"}})
	counter := testpkg.CaptureQueries(t, db)
	run := func(n int) []testpkg.QueryMeasurement {
		request := f.request(fmt.Sprintf("growth%d", n), nil)
		var firstChild int64
		for i := range n {
			child := f.child(request.ID, fmt.Sprintf("Child%d", i), enrollment.ChildStatusRejected, nil)
			if i == 0 {
				firstChild = child.ID
			}
		}
		queries := make([]testpkg.QueryMeasurement, 2)
		for i, path := range []string{fmt.Sprintf("/admin/requests/%d/delete-impact", request.ID), fmt.Sprintf("/admin/requests/%d/children/%d/delete-impact", request.ID, firstChild)} {
			raw, q := growthGET(t, router, counter, token, path)
			queries[i] = q
			var impact enrollmentHTTP.AdminEnrollmentDeletionImpact
			require.NoError(t, json.Unmarshal(raw, &impact))
			require.Equal(t, fmt.Sprint(request.ID), impact.RequestID)
			require.True(t, impact.CanDelete)
			if i == 0 {
				require.Equal(t, n, impact.Counts.RequestChildren)
				require.True(t, impact.DeletesRequest)
			} else {
				require.Equal(t, 1, impact.Counts.RequestChildren)
				require.Equal(t, fmt.Sprint(firstChild), impact.ChildID)
				require.False(t, impact.DeletesRequest)
			}
		}
		return queries
	}
	small, large := run(3), run(8)
	for i := range small {
		t.Logf("deletion route %d N3=%d N8=%d", i, len(small[i].Statements), len(large[i].Statements))
		require.Equal(t, len(small[i].Statements), len(large[i].Statements))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.request_delete_impact", large[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.child_delete_impact", large[1])
}
