package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestWorkforceRecordWritesRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Record", "Rollback")
	capability := buildWorkforce(t, db)
	tables := []string{"active.staff_balance_adjustments", "active.staff_vacation_openings", "active.staff_vacation_quota", "users.staff_master_data", "users.staff_financial_data", "users.staff_qualifications", "users.staff_documents", "users.staff_document_file_cleanup"}
	snapshot := func() map[string]string {
		result := make(map[string]string)
		for _, table := range tables {
			var rows string
			require.NoError(t, db.NewSelect().TableExpr("? AS entry", bun.Ident(table)).ColumnExpr("COALESCE(jsonb_agg(to_jsonb(entry) ORDER BY entry.id), '[]'::jsonb)").Where("entry.tenant_id = ?", testpkg.Tenant(t)).Scan(ctx, &rows))
			result[table] = rows
		}
		return result
	}
	type writeStep struct {
		name string
		run  func(context.Context) error
	}
	abort := errors.New("injected failure after authoritative workforce write")
	run := func(steps []writeStep) {
		before := snapshot()
		for failAfter, step := range steps {
			err := testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				for index, write := range steps {
					if err := write.run(txCtx); err != nil {
						return err
					}
					if index == failAfter {
						return abort
					}
				}
				return nil
			})
			require.ErrorIs(t, err, abort, step.name)
			require.Equal(t, before, snapshot(), "every row must be restored after "+step.name)
		}
		require.NoError(t, testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			for _, step := range steps {
				if err := step.run(txCtx); err != nil {
					return err
				}
			}
			return nil
		}))
		require.NotEqual(t, before, snapshot(), "retry must commit the intended changes")
	}
	var master workforce.StaffMasterData
	var financial workforce.StaffFinancialData
	var adjustment workforce.StaffBalanceAdjustment
	var document workforce.StaffDocument
	var err error
	run([]writeStep{
		{"master-create", func(ctx context.Context) error {
			master, err = capability.CreateStaffMasterData(ctx, workforce.StaffMasterData{StaffID: staff.ID})
			return err
		}},
		{"master-update", func(ctx context.Context) error {
			master.Phone = new("12345")
			master, err = capability.UpdateStaffMasterData(ctx, master)
			return err
		}},
		{"financial-create", func(ctx context.Context) error {
			financial, err = capability.CreateStaffFinancialData(ctx, workforce.StaffFinancialData{StaffID: staff.ID})
			return err
		}},
		{"financial-update", func(ctx context.Context) error {
			financial.TaxID = new("12345678901")
			financial, err = capability.UpdateStaffFinancialData(ctx, financial)
			return err
		}},
		{"qualifications-replace", func(ctx context.Context) error {
			_, err := capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{{Name: "Qualification"}})
			return err
		}},
		{"balance-create", func(ctx context.Context) error {
			adjustment, err = capability.CreateStaffBalanceAdjustment(ctx, workforce.StaffBalanceAdjustment{StaffID: staff.ID, Type: workforce.BalanceAdjustmentTypeOpening, EffectiveDate: "2026-05-04", DecidedBy: staff.ID})
			return err
		}},
		{"balance-update", func(ctx context.Context) error {
			adjustment.MinutesDelta = 60
			adjustment, err = capability.UpdateStaffBalanceAdjustment(ctx, adjustment)
			return err
		}},
		{"opening-create", func(ctx context.Context) error {
			_, err := capability.CreateStaffVacationOpening(ctx, workforce.StaffVacationOpening{StaffID: staff.ID, Year: 2026, EffectiveDate: "2026-05-04", EnteredRemainingDays: 30, DecidedBy: staff.ID})
			return err
		}},
		{"quota-insert", func(ctx context.Context) error {
			return capability.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: staff.ID, Year: 2026, EntitledDays: 30})
		}},
		{"quota-upsert", func(ctx context.Context) error {
			return capability.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: staff.ID, Year: 2026, EntitledDays: 31})
		}},
		{"document-create", func(ctx context.Context) error {
			document, err = capability.CreateStaffDocument(ctx, testDocument(staff.ID, workforce.StaffDocumentCategorySonstiges, "rollback-document"))
			return err
		}},
	})
	for _, table := range tables[:len(tables)-1] {
		count, err := db.NewSelect().Table(table).Where("tenant_id = ?", testpkg.Tenant(t)).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count, "retry must leave exactly one intended row in "+table)
	}
	run([]writeStep{
		{"document-soft-delete", func(ctx context.Context) error {
			changed, err := capability.SoftDeleteStaffDocument(ctx, document.ID, staff.ID, time.Now())
			require.EqualValues(t, 1, changed)
			return err
		}},
		{"document-cleanup-intent", func(ctx context.Context) error {
			return capability.QueueStaffDocumentFileCleanup(ctx, workforce.StaffDocumentFileCleanup{StaffID: staff.ID, FilenameStored: document.FilenameStored, RetryAfter: time.Now().Add(-time.Hour)})
		}},
	})
	intents, err := capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, intents, 1, "only the committed retry may leave a cleanup intent")
	run([]writeStep{
		{"document-file-deleted", func(ctx context.Context) error {
			return capability.MarkStaffDocumentFileDeleted(ctx, document.ID, time.Now())
		}},
		{"document-cleanup-complete", func(ctx context.Context) error {
			return capability.CompleteStaffDocumentFileCleanup(ctx, intents[0].ID)
		}},
	})
	intents, err = capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
	require.NoError(t, err)
	require.Empty(t, intents)
}
