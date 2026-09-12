package integration

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestImportBatches_RollbackResumeAndIdempotentReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := make([]importModels.ClassListEntryImportRow, 205)
	for i := range rows {
		rows[i] = importModels.ClassListEntryImportRow{FirstName: fmt.Sprintf("Kind%d", i), LastName: "Batch", SchoolClass: "1a"}
	}
	rows[150].FirstName = "Refused"
	request := importModels.ImportRequest[importModels.ClassListEntryImportRow]{Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true}
	audit := importService.BatchAudit{EntityType: "class_list_entries", Filename: "batch.csv", AccountID: actor.accountID}
	_, err = db.ExecContext(context.Background(), `ALTER TABLE users.class_list_entries ADD CONSTRAINT refuse_import_batch CHECK (first_name <> 'Refused')`)
	require.NoError(t, err)
	removeFailure := func() {
		_, err := db.ExecContext(context.Background(), `ALTER TABLE users.class_list_entries DROP CONSTRAINT IF EXISTS refuse_import_batch`)
		require.NoError(t, err)
	}
	t.Cleanup(removeFailure)
	count := func(table string) int {
		t.Helper()
		var n int
		require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &n))
		return n
	}
	result, err := module.ClassListImport.ImportBatches(testpkg.Ctx(t), request, audit)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 100, result.CreatedCount, "only the first committed batch is acknowledged")
	assert.Equal(t, 100, count("users.class_list_entries"), "all fifty earlier writes in the failing batch roll back")
	assert.Equal(t, 100, count("audit.class_list_entry_changes"))
	assert.Equal(t, 1, count("audit.data_imports"), "checkpoint and audit record commit together")
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 152, result.Errors[0].RowNumber)
	assert.Equal(t, "creation_failed", result.Errors[0].Errors[0].Code)
	require.Len(t, *module.Observations, 1)
	failedObservation := (*module.Observations)[0]
	assert.Equal(t, 1, failedObservation.BatchesCommitted)
	assert.Equal(t, 105, failedObservation.CheckpointLag)
	assert.Equal(t, 100, failedObservation.Created)

	removeFailure()
	// Rebuild the workflow: progress lives in PostgreSQL, not this service's
	// caches or the lifetime of the process that committed the first batch.
	resumed, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	result, err = resumed.ClassListImport.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	assert.Equal(t, 205, result.CreatedCount)
	assert.Zero(t, result.ErrorCount)
	assert.Equal(t, 205, count("users.class_list_entries"))
	assert.Equal(t, 205, count("audit.class_list_entry_changes"))
	assert.Equal(t, 3, count("audit.data_imports"))
	require.Len(t, *resumed.Observations, 1)
	assert.Equal(t, 105, (*resumed.Observations)[0].Created, "resuming does not count earlier writes twice")
	assert.Equal(t, 2, (*resumed.Observations)[0].BatchesCommitted)
	result, err = resumed.ClassListImport.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	assert.Equal(t, 205, result.CreatedCount, "replay restores committed outcomes")
	assert.Zero(t, result.ErrorCount)
	assert.Equal(t, 205, count("users.class_list_entries"))
	assert.Equal(t, 3, count("audit.data_imports"), "replay appends neither rows nor receipts")
	require.Len(t, *resumed.Observations, 2)
	assert.Zero(t, (*resumed.Observations)[1].Created)
	assert.Zero(t, (*resumed.Observations)[1].BatchesCommitted)
	assert.Zero(t, (*resumed.Observations)[1].CheckpointLag)
}

func TestImportBatches_ConcurrentIdenticalUploadsCommitOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	actor := newImporter(t, db)
	modules := make([]services.ImportTestModule, 2)
	for i := range modules {
		var err error
		modules[i], err = services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
	}
	rows := make([]importModels.ClassListEntryImportRow, 205)
	for i := range rows {
		rows[i] = importModels.ClassListEntryImportRow{FirstName: fmt.Sprintf("Concurrent%d", i), LastName: "Upload", SchoolClass: "1a"}
	}
	request := importModels.ImportRequest[importModels.ClassListEntryImportRow]{Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true}
	audit := importService.BatchAudit{EntityType: "class_list_entries", Filename: "concurrent.csv", AccountID: actor.accountID}
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()
	validated := make(chan struct{}, 2)
	release := make(chan struct{})
	type outcome struct {
		result *importModels.ImportResult[importModels.ClassListEntryImportRow]
		err    error
	}
	outcomes := make(chan outcome, 2)
	for _, module := range modules {
		go func(module services.ImportTestModule) {
			initial := true
			runCtx := tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
				if initial && event.Kind == tenant.UnitOfWorkTransaction {
					initial = false
					validated <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
					}
				}
			})
			result, err := module.ClassListImport.ImportBatches(runCtx, request, audit)
			outcomes <- outcome{result: result, err: err}
		}(module)
	}
	for range 2 {
		select {
		case <-validated:
		case <-ctx.Done():
			t.Fatal("concurrent uploads did not both finish validation")
		}
	}
	// Both runs saw an empty checkpoint history. Their batch transactions
	// must recheck that history under the database lock, not trust the preview.
	close(release)
	for range 2 {
		select {
		case outcome := <-outcomes:
			require.NoError(t, outcome.err)
			require.NotNil(t, outcome.result)
			assert.Equal(t, 205, outcome.result.CreatedCount)
			assert.Zero(t, outcome.result.ErrorCount)
		case <-ctx.Done():
			t.Fatal("concurrent uploads did not finish")
		}
	}
	for table, expected := range map[string]int{"users.class_list_entries": 205, "audit.class_list_entry_changes": 205, "audit.data_imports": 3} {
		var count int
		require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &count))
		assert.Equal(t, expected, count, table)
	}
	created, committed := 0, 0
	for _, module := range modules {
		require.Len(t, *module.Observations, 1)
		created += (*module.Observations)[0].Created
		committed += (*module.Observations)[0].BatchesCommitted
	}
	assert.Equal(t, 205, created, "runtime counters also count each write only once")
	assert.Equal(t, 3, committed)
}

func TestImportBatches_PreviewAndStableRejectedRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := make([]importModels.ClassListEntryImportRow, 205)
	for i := range rows {
		rows[i] = importModels.ClassListEntryImportRow{FirstName: fmt.Sprintf("Kind%d", i), LastName: "Preview", SchoolClass: "1a"}
	}
	rows[0].FirstName = "  Kind0  "
	rows[204].SchoolClass = ""
	request := importModels.ImportRequest[importModels.ClassListEntryImportRow]{Rows: rows, Mode: importModels.ImportModeCreate, DryRun: true, UserID: actor.staffID, SkipInvalidRows: true}
	audit := importService.BatchAudit{EntityType: "class_list_entries", Filename: "validation.csv", AccountID: actor.accountID}
	count := func(table string) int {
		t.Helper()
		var n int
		require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &n))
		return n
	}
	preview, err := module.ClassListImport.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	assert.Equal(t, 204, preview.CreatedCount)
	assert.Equal(t, 1, preview.ErrorCount)
	assert.Zero(t, count("users.class_list_entries"))
	assert.Equal(t, 1, count("audit.data_imports"), "preview is audited without a resume checkpoint")
	request.DryRun = false
	rows[0].FirstName = "  Kind0  "
	// Also prove import-specific observation does not suppress the root's
	// UnitOfWork metrics.
	transactions := 0
	ctx := tenant.WithUnitOfWorkObserver(testpkg.Ctx(t), func(event tenant.UnitOfWorkEvent) {
		if event.Kind == tenant.UnitOfWorkTransaction {
			transactions++
		}
	})
	result, err := module.ClassListImport.ImportBatches(ctx, request, audit)
	require.NoError(t, err)
	assert.Equal(t, 204, result.CreatedCount)
	assert.Equal(t, "  Kind0  ", rows[0].FirstName, "normalization must not change the caller's replay identity")
	assert.Equal(t, 1, result.ErrorCount)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 206, result.Errors[0].RowNumber)
	assert.Equal(t, 4, transactions, "one validation transaction and three bounded write transactions")
	assert.Equal(t, 204, count("users.class_list_entries"))
	assert.Equal(t, 4, count("audit.data_imports"))
	originalErrors, err := json.Marshal(result.Errors)
	require.NoError(t, err)
	result, err = module.ClassListImport.ImportBatches(ctx, request, audit)
	require.NoError(t, err)
	assert.Equal(t, 204, result.CreatedCount)
	replayedErrors, err := json.Marshal(result.Errors)
	require.NoError(t, err)
	assert.JSONEq(t, string(originalErrors), string(replayedErrors), "replay preserves row numbers, messages and error timestamps")
	assert.Equal(t, 204, count("users.class_list_entries"))
	assert.Equal(t, 4, count("audit.data_imports"))
}

func TestImportBatches_RetriesWholeTransactionAfterDatabaseConflict(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, code := range []string{"40P01", "40001"} {
		t.Run(code, func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			actor := newImporter(t, db)
			// Sequence increments survive rollback. The second row fails once,
			// after the first owner write, then succeeds on the transaction retry.
			_, err = db.ExecContext(context.Background(), `
				CREATE SEQUENCE public.import_conflict_attempt;
				GRANT USAGE ON SEQUENCE public.import_conflict_attempt TO phoenix_tenant;
				CREATE FUNCTION public.import_conflict_once() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN
					IF NEW.first_name = 'Conflict' AND nextval('public.import_conflict_attempt') = 1 THEN
						RAISE EXCEPTION 'injected transaction conflict' USING ERRCODE = TG_ARGV[0];
					END IF;
					RETURN NEW;
				END $$;
			`)
			require.NoError(t, err)
			_, err = db.ExecContext(context.Background(), fmt.Sprintf(`CREATE TRIGGER import_conflict_once AFTER INSERT ON users.class_list_entries FOR EACH ROW EXECUTE FUNCTION public.import_conflict_once('%s')`, code))
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := db.ExecContext(context.Background(), `DROP TRIGGER import_conflict_once ON users.class_list_entries; DROP FUNCTION public.import_conflict_once(); DROP SEQUENCE public.import_conflict_attempt`)
				require.NoError(t, err)
			})
			ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
			result, err := module.ClassListImport.ImportBatches(ctx, importModels.ImportRequest[importModels.ClassListEntryImportRow]{
				Rows: []importModels.ClassListEntryImportRow{
					{FirstName: "Before", LastName: "Retry", SchoolClass: "1a"},
					{FirstName: "Conflict", LastName: "Retry", SchoolClass: "1a"},
				},
				Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
			}, importService.BatchAudit{EntityType: "class_list_entries", Filename: "conflict.csv", AccountID: actor.accountID})
			require.NoError(t, err)
			assert.Equal(t, 2, result.CreatedCount)
			assert.Zero(t, result.ErrorCount, "the failed attempt's errors are not part of the committed result")
			for table, expected := range map[string]int{"users.class_list_entries": 2, "audit.class_list_entry_changes": 2, "audit.data_imports": 1} {
				var count int
				require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("count(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &count))
				assert.Equal(t, expected, count, table)
			}
			require.Len(t, *module.Observations, 1)
			observation := (*module.Observations)[0]
			assert.Equal(t, 1, observation.BatchesRetried)
			assert.Equal(t, 1, observation.BatchesCommitted)
			assert.Equal(t, 2, observation.Created)
			assert.Zero(t, observation.CheckpointLag)
			ownerAttempts, ownerFailures := 0, 0
			for _, command := range observation.Commands {
				if command.Owner == "school-membership" && command.Operation == "create_class_list_entry" {
					ownerAttempts++
					if command.Failed {
						ownerFailures++
					}
					assert.Greater(t, command.Duration.Nanoseconds(), int64(0))
				}
			}
			assert.Equal(t, 4, ownerAttempts, "both rows are attempted again after rollback")
			assert.Equal(t, 1, ownerFailures)
			if code == "40P01" {
				assert.Equal(t, 1, observation.Deadlocks)
			} else {
				assert.Zero(t, observation.Deadlocks)
			}
		})
	}
}
