package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestImportBatches_OpeningBalanceOwnersRollbackAndReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, table := range []string{"active.staff_balance_adjustments", "active.staff_vacation_quota", "active.staff_vacation_openings", "audit.data_imports"} {
		t.Run(table, func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := services.NewImportTestModuleWithOptions(db, testpkg.TenantRuntime(t, db), services.ImportTestOptions{
				Clock: func() time.Time { return time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC) },
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			staff := testpkg.CreateTestStaff(t, db, "Opening", "Batch")
			ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
			svc := module.OpeningBalanceImport(testpkg.Date(2026, time.September, 1), "Übernahme", actor.staffID)
			request := importModels.ImportRequest[importModels.OpeningBalanceImportRow]{
				Rows: []importModels.OpeningBalanceImportRow{{FirstName: "Opening", LastName: "Batch", HoursBalance: "-3,25", VacationEntitled: "30", VacationCarryover: "4", VacationRemaining: "12,5"}},
				Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
			}
			audit := importService.BatchAudit{EntityType: "opening_balance", Filename: "opening.csv", AccountID: actor.accountID, Options: "2026-09-01"}
			count := func(table string) int {
				t.Helper()
				var count int
				require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Scan(ctx, &count))
				return count
			}
			request.DryRun = true
			preview, err := svc.ImportBatches(ctx, request, audit)
			require.NoError(t, err)
			require.Zero(t, preview.ErrorCount, "%+v", preview.Errors)
			assert.Equal(t, 1, preview.CreatedCount)
			assert.Zero(t, count("active.staff_balance_adjustments"))
			request.DryRun = false
			removeFailure := refuseOwnerInsert(t, db, table)
			result, err := svc.ImportBatches(ctx, request, audit)
			require.Error(t, err)
			require.NotNil(t, result)
			assert.Zero(t, result.CreatedCount)
			for _, ownerTable := range []string{"active.staff_balance_adjustments", "active.staff_vacation_quota", "active.staff_vacation_openings"} {
				assert.Zero(t, count(ownerTable), ownerTable)
			}
			assert.Equal(t, 1, count("audit.data_imports"), "only the preview audit survives")
			removeFailure()
			result, err = svc.ImportBatches(ctx, request, audit)
			require.NoError(t, err)
			require.Zero(t, result.ErrorCount, "%+v", result.Errors)
			assert.Equal(t, 1, result.CreatedCount)
			var opening struct{ TakenBeforeDays, EnteredRemainingDays float64 }
			require.NoError(t, db.NewSelect().TableExpr("active.staff_vacation_openings").ColumnExpr("taken_before_days, entered_remaining_days").Where("tenant_id = ? AND staff_id = ?", testpkg.Tenant(t), staff.ID).Scan(ctx, &opening))
			assert.Equal(t, 21.5, opening.TakenBeforeDays)
			assert.Equal(t, 12.5, opening.EnteredRemainingDays)
			result, err = svc.ImportBatches(ctx, request, audit)
			require.NoError(t, err)
			assert.Equal(t, 1, result.CreatedCount)
			assert.Zero(t, result.ErrorCount, "completed replay restores outcomes despite existing opening guards")
			assert.Equal(t, 2, count("audit.data_imports"))
		})
	}
}
