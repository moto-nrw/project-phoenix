package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	tenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/erasureledger"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func terminateErasureLock(t *testing.T, db *bun.DB) {
	t.Helper()
	var pid int
	require.NoError(t, db.NewRaw(`SELECT pid FROM pg_locks WHERE locktype='advisory' AND database=(SELECT oid FROM pg_database WHERE datname=current_database())`).Scan(t.Context(), &pid))
	var terminated bool
	require.NoError(t, db.NewRaw(`SELECT pg_terminate_backend(?,5000)`, pid).Scan(t.Context(), &terminated))
	require.True(t, terminated)
}

func TestErasureCLIStopsFilesAndLedgerAfterLockSessionDies(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"files", "ledger"} {
		t.Run(phase, func(t *testing.T) {
			db := testpkg.SetupIsolatedTestDB(t)
			id := testpkg.Tenant(t)
			operator := testpkg.CreateTestOperator(t, db)
			ledgerPath := newTestErasureLedger(t)
			before, err := os.ReadFile(ledgerPath)
			require.NoError(t, err)
			uploads := t.TempDir()
			files := &failingErasureFiles{Local: objects.NewLocal(uploads)}
			retained := filepath.Join(uploads, "announcement-attachments", fmt.Sprint(id), "retained.bin")
			if phase == "files" {
				require.NoError(t, os.MkdirAll(filepath.Dir(retained), 0700))
				require.NoError(t, os.WriteFile(retained, []byte("fixture"), 0600))
				files.onRemove = func() { terminateErasureLock(t, db) }
			} else {
				// End coordination after the final DB step, before the ledger write.
				_, err := db.ExecContext(t.Context(), `CREATE FUNCTION platform.end_erasure_lock() RETURNS trigger LANGUAGE plpgsql AS $$
    BEGIN
     PERFORM pg_terminate_backend(pid,5000) FROM pg_locks WHERE locktype='advisory'
      AND database=(SELECT oid FROM pg_database WHERE datname=current_database()) AND pid<>pg_backend_pid();
     RETURN NEW;
    END $$;
    CREATE TRIGGER end_erasure_lock AFTER UPDATE ON platform.school_erasures
    FOR EACH ROW WHEN (NEW.status='completed') EXECUTE FUNCTION platform.end_erasure_lock()`)
				require.NoError(t, err)
				t.Cleanup(func() {
					_, err := db.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS end_erasure_lock ON platform.school_erasures; DROP FUNCTION IF EXISTS platform.end_erasure_lock()`)
					require.NoError(t, err)
				})
			}
			service, err := tenancyCompose.NewErasure(db, files, erasureledger.New(ledgerPath, securityruntime.Fingerprint))
			require.NoError(t, err)
			request := organizationtenancy.ErasureRequest{SchoolID: id, OperatorID: operator.ID, OrderReference: "ORDER-LOCK-LOSS", Confirmation: fmt.Sprintf("school:%d", id), PrerequisitesChecked: true, WritersStopped: true, LockTimeout: time.Second, StatementTimeout: time.Minute}
			result, err := service.Run(t.Context(), request)
			require.ErrorContains(t, err, "coordination session")
			require.ErrorContains(t, err, "last confirmed")
			if phase == "files" {
				require.FileExists(t, retained, "the next file namespace must not be erased after lock loss")
			}
			if phase == "ledger" {
				require.Empty(t, result, "no completion evidence without the guarded ledger write")
			}
			after, err := os.ReadFile(ledgerPath)
			require.NoError(t, err)
			require.Equal(t, before, after, "no ledger write after losing coordination")
		})
	}
}
