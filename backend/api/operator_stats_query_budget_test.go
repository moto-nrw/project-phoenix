//go:build testhooks

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestOperatorStatsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, _, token, operatorID := operatorListBudgetRouter(t, db)
	provisioning := buildOperatorProvisioning(t, db)
	ctx := provisioningContext(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	read := func() (organizationtenancy.ProvisioningStats, []string) {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/operator/stats", nil).WithContext(counter.Context(ctx))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		counter.Reset()
		router.ServeHTTP(response, request)
		statements := counter.Queries()
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body struct {
			Data organizationtenancy.ProvisioningStats `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		return body.Data, statements
	}
	baseline, _ := read()
	created := 0
	for _, n := range []int{3, 8} {
		for created < n {
			provisionTestSchool(t, db, provisioning, operatorID, fmt.Sprintf("stats-budget-%d", created))
			created++
		}
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			stats, statements := read()
			require.Equal(t, organizationtenancy.ProvisioningStats{
				TraegerCount: baseline.TraegerCount + n,
				SchulenCount: baseline.SchulenCount + n,
				KontenCount:  baseline.KontenCount,
				GeraeteCount: baseline.GeraeteCount + n,
			}, stats)
			t.Logf("MEASURE operator.stats N%d statements=%d", n, len(statements))
			querybudgets.AssertQueryBudget(t, "operator.stats", statements)
		})
	}
}
