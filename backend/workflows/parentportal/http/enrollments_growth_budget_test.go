package parent_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
	parentHTTP "github.com/moto-ogs/project-phoenix/workflows/parentportal/http"
	"github.com/stretchr/testify/require"
)

func TestParentMyEnrollmentsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	repos := api.NewCarePlanSuiteRepositories(database)
	portal := portalCompose.New(portalCompose.Dependencies{GuardianPolicy: owners.NewParentGuardianPolicy(), DeparturePolicy: owners.ParentDeparturePolicy(), Fingerprints: owners.SecurityFingerprints{}, Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{}, Logger: slog.Default(), EnrollmentRequestRepo: repos.ParentEnrollmentReads()})
	router := testpkg.TenantRuntimeMiddleware(t, db)(parentHTTP.NewResource(parentHTTP.ResourceConfig{Parent: portal}).Router(api.ParentSessionChain))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestParentGuardianChain(t, db)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	insertChild := func(requestID int64, name string) {
		_, err := db.NewRaw(`INSERT INTO enrollment.request_children (tenant_id,request_id,first_name,last_name,date_of_birth,status,target_grade_level) VALUES (?,?,?,'Child','2018-04-15','submitted',1)`, testpkg.Tenant(t), requestID, name).Exec(testpkg.Ctx(t))
		require.NoError(t, err)
	}
	var ids []string
	add := func(n int) {
		for range n {
			row := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, chain.Email, fmt.Sprintf("parent-growth-%d-%d", testpkg.UniqueSuffix(), len(ids)))
			insertChild(row.ID, "ParentGrowth")
			ids = append(ids, fmt.Sprint(row.ID))
		}
	}
	unrelated := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, other.Email, fmt.Sprintf("other-growth-%d", testpkg.UniqueSuffix()))
	insertChild(unrelated.ID, "Unrelated")
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(chain.AccountID), Sub: chain.Email, Roles: []string{"parent"}, Scope: "parent"})
	counter := testpkg.CaptureQueries(t, db)
	run := func(n int) testpkg.QueryMeasurement {
		req := httptest.NewRequest(http.MethodGet, "/me/enrollments", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		counter.Reset()
		testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/parent", router), rec, req)
		queries := counter.Queries()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data []parentHTTP.EnrollmentRequestResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Data, n)
		actual := make([]string, 0, n)
		for _, row := range body.Data {
			actual = append(actual, row.RequestID)
			require.NotEqual(t, fmt.Sprint(unrelated.ID), row.RequestID)
			require.Equal(t, fmt.Sprint(phase.ID), row.PhaseID)
			require.Equal(t, fmt.Sprint(testpkg.Tenant(t)), row.TenantID)
			require.NotEmpty(t, row.StatusToken)
			require.Len(t, row.Children, 1)
			require.Equal(t, "ParentGrowth", row.Children[0].FirstName)
			require.Nil(t, row.Children[0].StatusReason)
		}
		require.ElementsMatch(t, ids, actual)
		return counter.Measurement(queries)
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	t.Logf("parent enrollments N3=%d N8=%d", len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "parent.enrollments.growth.mine", large)
}
