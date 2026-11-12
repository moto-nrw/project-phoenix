package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffImportBatchesEmailMatchesAndMisses(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModuleWithAuth(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{}, api.WithAuthTestMailer(testpkg.NewCapturingMailer()))
	require.NoError(t, err)
	actor := newImporter(t, db)
	role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", testpkg.Tenant(t))
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	ctx = importModels.ContextWithImporterPermissions(ctx, []string{"admin:*"})
	counter := testpkg.CaptureQueriesForContext(t, db)
	runner := importBudgetRun[importModels.StaffImportRow]{
		importer: module.StaffImport, ctx: counter.Context(ctx), counter: counter, userID: actor.accountID,
		audit: importModels.BatchAudit{EntityType: "staff", Filename: "email.csv", AccountID: actor.accountID},
	}
	rows := make([]importModels.StaffImportRow, 3)
	for i := range rows {
		rows[i] = importModels.StaffImportRow{
			FirstName: fmt.Sprintf("Email%d", i), LastName: "Staff", RoleName: role.Name,
			Email: fmt.Sprintf("batch.%d.%d@example.test", role.ID, i),
		}
	}
	for _, mode := range []importModels.ImportMode{importModels.ImportModeUpsert, importModels.ImportModeUpdate} {
		want := allCreated(len(rows))
		if mode == importModels.ImportModeUpdate {
			want = allUpdated(len(rows))
			for i := range rows {
				rows[i].LastName = "Renamed" // Only the pending email identity matches.
			}
		}
		runner.run(t, rows, mode, false, want)
		require.Len(t, counter.Matching(func(sql string) bool {
			return strings.Contains(sql, "union all") && strings.Contains(sql, "auth.invitation_tokens")
		}), 1, "one identity-set read per chunk, for both new and existing addresses")
		scalarReads := counter.Matching(func(sql string) bool {
			return strings.Contains(sql, "select id, email from auth.accounts") && strings.Contains(sql, "lower(email) =")
		})
		limit := len(rows) // Validation retains its pre-existing row lookup.
		if mode == importModels.ImportModeUpsert {
			limit += len(rows) // Creating each invitation also checks the account.
		}
		require.LessOrEqual(t, len(scalarReads), limit, "preflight and row matching add no scalar lookups")
	}
}
