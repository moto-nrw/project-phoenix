package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentProfileRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	chain := testpkg.CreateTestParentGuardianChain(t, env.db)
	unrelated := testpkg.CreateTestParentGuardianChain(t, env.db)
	f.resource.GuardianProfiles = api.NewEnrollmentGuardianAutofillSuite(testpkg.NewDatabase(t, env.db))
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(chain.AccountID), Sub: chain.Email, Username: chain.Email, TenantID: chain.TenantID, Roles: []string{"parent"}})
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, env.db)
	ids := []string{fmt.Sprint(chain.StudentID)}
	add := func(n int) {
		for range n {
			student := testpkg.CreateTestStudent(t, env.db, fmt.Sprintf("Profile%02d", len(ids)), "Child", "2a")
			testpkg.CreateTestStudentGuardianLink(t, env.db, student.ID, chain.GuardianProfileID, "primary_guardian")
			ids = append(ids, fmt.Sprint(student.ID))
		}
	}
	run := func(n int) testpkg.QueryMeasurement {
		raw, q := growthGET(t, router, counter, token, "/me/profile")
		var body struct {
			Guardian struct {
				Email string `json:"email"`
			} `json:"guardian"`
			Children []struct {
				ID               string `json:"id"`
				EnrollmentSubmit bool   `json:"enrollment_submit"`
				Status           string `json:"status"`
			} `json:"children"`
		}
		require.NoError(t, json.Unmarshal(raw, &body))
		require.Equal(t, chain.Email, body.Guardian.Email)
		require.Len(t, body.Children, n)
		actual := []string{}
		for _, child := range body.Children {
			actual = append(actual, child.ID)
			require.NotEqual(t, fmt.Sprint(unrelated.StudentID), child.ID)
			require.True(t, child.EnrollmentSubmit)
			require.NotEmpty(t, child.Status)
		}
		require.ElementsMatch(t, ids, actual)
		return q
	}
	add(2)
	small := run(3)
	add(5)
	large := run(8)
	require.Equal(t, len(small.Statements), len(large.Statements))
	t.Logf("profile N3=%d N8=%d", len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.profile", large)
}
