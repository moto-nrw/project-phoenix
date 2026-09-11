package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestDataImportRuntimeEvidence measures one preview and one import of the
// same ten-row file per iteration and prints the samples as JSON. Rows
// parsed/accepted/rejected come from the production observer, statements and
// rows from the query counter, transaction outcomes from the unit-of-work
// observer, and pool waits and deadlocks from the driver and PostgreSQL.
// It records no personal data: only counters, durations and the entity name.
//
//	CGO_ENABLED=0 scripts/run-go-toolchain.sh go test -C backend \
//	  ./services/import/integration -run TestDataImportRuntimeEvidence -count=1 -v
func TestDataImportRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.Tenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	observations := module.Observations
	actor := newImporter(t, db)

	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	baseCtx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.Ctx(t)))
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(context.Background(), &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(context.Background(), &count))
		return count
	}
	beforeDeadlocks := deadlocks()

	const rowsPerBatch = 10
	batch := func(iteration int) []importModels.StudentImportRow {
		rows := make([]importModels.StudentImportRow, 0, rowsPerBatch)
		for i := range rowsPerBatch {
			name := fmt.Sprintf("Runtime%d-%d", iteration, i)
			rows = append(rows, importModels.StudentImportRow{
				FirstName: name, LastName: "Evidence", SchoolClass: "4A", Birthday: "2017-04-05", DataRetentionDays: 30,
				AddressStreet: "Messweg 1", AddressPostalCode: "50667", AddressCity: "Köln",
				DepartureDays:   map[string]string{"mon": "bus", "tue": "pickup"},
				PrivacyAccepted: true,
				Guardians: []importModels.GuardianImportData{{
					FirstName: "Eltern", LastName: name, Email: fmt.Sprintf("%s.runtime@example.test", name), RelationshipType: "Mutter",
					PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: fmt.Sprintf("0151 %06d", iteration*100+i), PhoneType: "mobile"}},
				}},
				PickupSchedules: []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}},
			})
		}
		return rows
	}

	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	measure := func(operation string, iteration int, dryRun bool, rows []importModels.StudentImportRow) {
		counter.Reset()
		beforeStats := db.Stats()
		started := time.Now()
		var result *importModels.ImportResult[importModels.StudentImportRow]
		err := testpkg.WithTenantTx(t, baseCtx, db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			var txErr error
			result, txErr = module.Import.Import(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
				Rows: rows, Mode: importModels.ImportModeCreate, DryRun: dryRun, UserID: actor.staffID, SkipInvalidRows: true,
			})
			if txErr != nil {
				return txErr
			}
			return module.Import.RecordAuditInTransaction(ctx, "student", "runtime.csv", result, actor.accountID, dryRun, tenantID)
		})
		elapsed := time.Since(started)
		afterStats := db.Stats()
		require.NoError(t, err)
		require.Zero(t, result.ErrorCount)
		if iteration < 2 {
			return // warmups
		}
		writes := counter.WriteRows()
		affected, statements := counter.Rows()
		samples[operation] = append(samples[operation], testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			WriteRowsAffected: &writes, RowsAffected: affected, StatementsWithRows: statements,
			PoolWaitCount: afterStats.WaitCount - beforeStats.WaitCount,
			PoolWaitMS:    float64(afterStats.WaitDuration-beforeStats.WaitDuration) / float64(time.Millisecond),
		})
	}

	for iteration := range 12 {
		rows := batch(iteration)
		measure("preview", iteration, true, rows)
		measure("import", iteration, false, rows)
	}

	counters := map[string][]importService.ImportObservation{}
	for _, observation := range *observations {
		mode := "import"
		if observation.DryRun {
			mode = "preview"
		}
		counters[mode] = append(counters[mode], observation)
	}
	raw, err := json.Marshal(map[string]any{
		"postgres": version, "warmup": 2, "samples_per_operation": 10, "rows_per_batch": rowsPerBatch, "concurrency": 1,
		"samples": samples, "row_counters": counters,
		"unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks,
	})
	require.NoError(t, err)
	t.Logf("data-import-runtime %s", raw)
}
