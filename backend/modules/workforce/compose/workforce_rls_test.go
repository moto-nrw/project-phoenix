package compose

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestWorkforceCutoverTablesEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	var contexts []context.Context
	var tenants []int64
	var rows []map[string]int64
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			staff := testpkg.CreateTestStaff(t, db, "RLS", side)
			ids := make(map[string]int64)
			day := timezone.NewDate(2026, 5, 4)
			session, err := capability.CreateWorkSession(ctx, testWorkSession(staff.ID, day, day.BerlinMidnight().Add(8*time.Hour)))
			require.NoError(t, err)
			ids["active.work_sessions"] = session.ID
			pause, err := capability.CreateWorkSessionBreak(ctx, workforce.WorkSessionBreak{SessionID: session.ID, StartedAt: session.CheckInTime.Add(time.Hour)})
			require.NoError(t, err)
			ids["active.work_session_breaks"] = pause.ID
			adjustment, err := capability.CreateStaffBalanceAdjustment(ctx, workforce.StaffBalanceAdjustment{StaffID: staff.ID, Type: workforce.BalanceAdjustmentTypeOpening, EffectiveDate: day.String(), DecidedBy: staff.ID})
			require.NoError(t, err)
			ids["active.staff_balance_adjustments"] = adjustment.ID
			opening, err := capability.CreateStaffVacationOpening(ctx, workforce.StaffVacationOpening{StaffID: staff.ID, Year: 2026, EffectiveDate: day.String(), EnteredRemainingDays: 30, DecidedBy: staff.ID})
			require.NoError(t, err)
			ids["active.staff_vacation_openings"] = opening.ID
			require.NoError(t, capability.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: staff.ID, Year: 2026, EntitledDays: 30}))
			quotas, err := capability.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffID: staff.ID, Year: 2026})
			require.NoError(t, err)
			require.Len(t, quotas, 1)
			ids["active.staff_vacation_quota"] = quotas[0].ID
			master, err := capability.CreateStaffMasterData(ctx, workforce.StaffMasterData{StaffID: staff.ID})
			require.NoError(t, err)
			ids["users.staff_master_data"] = master.ID
			financial, err := capability.CreateStaffFinancialData(ctx, workforce.StaffFinancialData{StaffID: staff.ID})
			require.NoError(t, err)
			ids["users.staff_financial_data"] = financial.ID
			qualifications, err := capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{{Name: "RLS qualification"}})
			require.NoError(t, err)
			require.Len(t, qualifications, 1)
			ids["users.staff_qualifications"] = qualifications[0].ID
			filename := fmt.Sprintf("rls-%d", staff.ID)
			document, err := capability.CreateStaffDocument(ctx, testDocument(staff.ID, workforce.StaffDocumentCategorySonstiges, filename))
			require.NoError(t, err)
			ids["users.staff_documents"] = document.ID
			require.NoError(t, capability.QueueStaffDocumentFileCleanup(ctx, workforce.StaffDocumentFileCleanup{StaffID: staff.ID, FilenameStored: filename, RetryAfter: time.Now().Add(-time.Hour)}))
			cleanups, err := capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
			require.NoError(t, err)
			require.Len(t, cleanups, 1)
			ids["users.staff_document_file_cleanup"] = cleanups[0].ID
			contexts = append(contexts, ctx)
			tenants = append(tenants, testpkg.Tenant(t))
			rows = append(rows, ids)
		})
	}
	require.Len(t, rows[0], 10)
	for table, ownID := range rows[0] {
		t.Run(table, func(t *testing.T) {
			for side, ctx := range contexts {
				require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenants[side], func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass)
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, rows[1][table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{rows[side][table]}, ids, "RLS must isolate rows without an owner predicate")
					return nil
				}))
			}
		})
	}
}
