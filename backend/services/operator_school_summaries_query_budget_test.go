package services

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestOperatorSchoolSummariesQueryBudget pins the statement count of the
// operator school listing (#3568). Besides the school rows it reads one
// aggregate per owner: accounts, devices, persons and the Kontingentzahl, all
// inside the administrative transaction, so it stays flat in the number of
// schools and children.
func TestOperatorSchoolSummariesQueryBudget(t *testing.T) {
	t.Parallel()

	// The counter attaches to this database, so the scenario owns its own
	// clone and cannot see statements from a parallel package.
	db := testpkg.SetupIsolatedTestDB(t)
	provisioning := buildOperatorProvisioning(t, db).OperatorProvisioning
	counter := testpkg.CaptureQueries(t, db)
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	listSchools := func() []string {
		t.Helper()
		var statements []string
		require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
			counter.Reset()
			schools, err := provisioning.ListSchoolSummaries(adminCtx)
			require.NoError(t, err)
			require.NotEmpty(t, schools)
			statements = counter.Queries()
			return nil
		}))
		return statements
	}

	addSchoolWithChildren := func(children int) {
		t.Helper()
		tenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, tenantID)
		for range children {
			testpkg.CreateTestStudentForTenant(t, db, tenantID, "Budget", "Kind", "1a")
		}
	}

	addSchoolWithChildren(1)
	before := listSchools()
	testpkg.AssertQueryBudget(t, "services.operator.school_summaries", before)

	for range 3 {
		addSchoolWithChildren(3)
	}
	after := listSchools()
	testpkg.AssertQueryBudget(t, "services.operator.school_summaries", after)

	require.Len(t, after, len(before),
		"operator school listing must stay flat in the number of schools and children")
}
