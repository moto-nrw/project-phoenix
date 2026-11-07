package integration

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Keep a small growth pair for the execution path with one guardian, phone
// and pickup schedule per child, including every transaction statement.
func TestStudentImportPhoneGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			actor := newImporter(t, db)
			counter := testpkg.CaptureQueriesForContext(t, db)
			ctx := counter.Context(testpkg.Ctx(t))
			result, err := module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
				Rows: studentBatch(n, n), Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
			}, importModels.BatchAudit{EntityType: "student", Filename: "phone-growth.csv", AccountID: actor.accountID})
			require.NoError(t, err)
			require.Zero(t, result.ErrorCount)
			if n == 3 {
				querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.phone_growth.n3", counter.Queries())
			} else {
				querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.phone_growth.n8", counter.Queries())
			}
			require.EqualValues(t, 5*n+1, counter.WriteRows())
			require.Empty(t, counter.Selects("users.guardian_phone_numbers"), "new guardians need no phone-list or count read before their first number")
		})
	}
}
