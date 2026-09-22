package migrations

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Isolated DDL tests use the same live checks as ordinary deployments.
// Registered execution enters through requestChildStorageContractUp.
func contractRequestChildStorage(ctx context.Context, db *bun.DB) error {
	return contractRequestChildStorageChecked(ctx, db, nil)
}

// requestChildContractFixture rebuilds the frozen pre-Contract world: the
// historical base table, its backfill and the committed cutover with its
// compatibility view, archive, routing function and hit counters.
func requestChildContractFixture(t *testing.T) (*testpkg.DB, requestChildStorageFixture) {
	t.Helper()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	return db, fixture
}

func requestChildContractOwnerRows(t *testing.T, db bun.IDB) []string {
	t.Helper()
	var snapshots []string
	for _, table := range []string{
		"enrollment.request_child_offering_selections", "enrollment.care_offering_bookings",
		"enrollment.request_child_storage_backfill_checkpoints",
	} {
		var snapshot string
		require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text)::text, '[]') FROM ? AS r`, bun.Ident(table)).Scan(t.Context(), &snapshot))
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func requestChildCompatibilityRetained(t *testing.T, db bun.IDB) bool {
	t.Helper()
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('enrollment.request_child_offerings') IS NOT NULL
		AND to_regclass('enrollment.request_child_offerings_legacy') IS NOT NULL
		AND to_regclass('enrollment.request_child_compatibility_reads') IS NOT NULL
		AND to_regclass('enrollment.request_child_compatibility_writes') IS NOT NULL
		AND to_regprocedure('enrollment.route_request_child_offering_compatibility()') IS NOT NULL`).Scan(t.Context(), &retained))
	return retained
}

func TestRequestChildContractRunsLockedCheckBeforeDestructiveDDL(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	refused := errors.New("locked replay check refused cleanup")
	checked := false
	err := contractRequestChildStorageChecked(t.Context(), db, func(ctx context.Context, connection bun.IDB) error {
		_, inTransaction := connection.(bun.Tx)
		require.True(t, inTransaction)
		checked = true
		return refused
	})
	require.ErrorIs(t, err, refused)
	require.True(t, checked)
	require.True(t, requestChildCompatibilityRetained(t, db))
}

func TestRequestChildContractRemovesCompatibilityObjectsAndPreservesOwners(t *testing.T) {
	t.Parallel()
	db, fixture := requestChildContractFixture(t)
	// A current-image booking change after cutover legitimately differs from
	// the frozen archive. Neither difference nor archive content may leak
	// into owner storage during Contract.
	_, err := db.NewRaw(`UPDATE enrollment.care_offering_bookings SET manual_selected_days = '["fri"]', automatic_selected_days = NULL WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	before := requestChildContractOwnerRows(t, db)
	require.NoError(t, contractRequestChildStorage(t.Context(), db))
	require.Equal(t, before, requestChildContractOwnerRows(t, db))
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('enrollment.request_child_offerings') IS NULL
		AND to_regclass('enrollment.request_child_offerings_legacy') IS NULL
		AND to_regclass('enrollment.request_child_offerings_id_seq') IS NULL
		AND to_regclass('enrollment.request_child_compatibility_reads') IS NULL
		AND to_regclass('enrollment.request_child_compatibility_writes') IS NULL
		AND to_regprocedure('enrollment.route_request_child_offering_compatibility()') IS NULL
		AND to_regprocedure('enrollment.request_child_legacy_manual(jsonb, jsonb, jsonb)') IS NULL
		AND to_regprocedure('enrollment.request_child_effective_days(jsonb, jsonb)') IS NULL`).Scan(t.Context(), &absent))
	require.True(t, absent, "all obsolete storage and compatibility objects must be gone")
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('enrollment.request_child_offering_selections') IS NOT NULL
		AND to_regclass('enrollment.care_offering_bookings') IS NOT NULL
		AND to_regclass('enrollment.care_offering_bookings_id_seq') IS NOT NULL
		AND to_regclass('enrollment.request_child_storage_backfill_checkpoints') IS NOT NULL`).Scan(t.Context(), &retained))
	require.True(t, retained, "owner storage and frozen backfill evidence must survive")
	require.ErrorContains(t, requestChildStorageContractDown(t.Context(), db), "restore the verified pre-Contract backup")
	require.Equal(t, before, requestChildContractOwnerRows(t, db), "refused Down must not rewrite current owner data")
}

func TestRequestChildContractHistoricalFixtureRestoresAfterRemoval(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	require.NoError(t, contractRequestChildStorage(t.Context(), db))
	testpkg.RestoreRequestChildStorageBeforeCutover(t, db)
	fixture := createRequestChildStorageFixture(t, db)
	require.Len(t, fixture.legacyIDs, 5)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	require.NoError(t, contractRequestChildStorage(t.Context(), db))
}

func TestRequestChildContractDataPreflightDoesNotProbeCompatibilityView(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	require.NoError(t, requestChildStorageContractDataPreflight(t.Context(), db))
	var reads, writes int64
	require.NoError(t, db.NewRaw(`SELECT
		coalesce(pg_sequence_last_value('enrollment.request_child_compatibility_reads'), 0),
		coalesce(pg_sequence_last_value('enrollment.request_child_compatibility_writes'), 0)`).Scan(t.Context(), &reads, &writes))
	require.Zero(t, reads)
	require.Zero(t, writes)
}

func TestRequestChildContractAllowsHistoricalCompatibilityHits(t *testing.T) {
	t.Parallel()
	for _, counter := range []string{"enrollment.request_child_compatibility_reads", "enrollment.request_child_compatibility_writes"} {
		t.Run(counter, func(t *testing.T) {
			db, _ := requestChildContractFixture(t)
			_, err := db.NewRaw(`SELECT nextval(?::regclass)`, counter).Exec(t.Context())
			require.NoError(t, err)
			require.NoError(t, requestChildStorageContractDataPreflight(t.Context(), db))
			require.NoError(t, contractRequestChildStorage(t.Context(), db))
		})
	}
}

func TestRequestChildContractRejectsUntrackedFunctionDependency(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"view":    `SELECT count(*) FROM enrollment.request_child_offerings`,
		"archive": `SELECT count(*) FROM enrollment.request_child_offerings_legacy`,
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := requestChildContractFixture(t)
			_, err := db.NewRaw(`CREATE FUNCTION enrollment.contract_hidden_reader() RETURNS bigint LANGUAGE sql AS ?`, body).Exec(t.Context())
			require.NoError(t, err)
			require.ErrorContains(t, contractRequestChildStorage(t.Context(), db), "stored function still references retired storage")
			require.True(t, requestChildCompatibilityRetained(t, db))
		})
	}
}

func TestRequestChildContractRefusesUnexpectedDependentView(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	_, err := db.ExecContext(t.Context(), `CREATE VIEW enrollment.contract_unexpected_dependency AS SELECT id FROM enrollment.request_child_offerings_legacy`)
	require.NoError(t, err)
	before := requestChildContractOwnerRows(t, db)
	require.ErrorContains(t, contractRequestChildStorage(t.Context(), db), "unexpected views depend on retired storage")
	require.True(t, requestChildCompatibilityRetained(t, db))
	require.Equal(t, before, requestChildContractOwnerRows(t, db))
}

func TestRequestChildContractRollsBackDDLOnFailedFingerprint(t *testing.T) {
	t.Parallel()
	db, fixture := requestChildContractFixture(t)
	// The obsolete objects are already dropped inside the transaction when the
	// fingerprint comparison runs; a change of owner rows must roll every
	// DROP back rather than commit a partially cleaned schema.
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION enrollment.contract_mutate_owner() RETURNS event_trigger LANGUAGE plpgsql AS $$
		BEGIN UPDATE enrollment.care_offering_bookings SET updated_at = updated_at + interval '1 second' WHERE id = `+strconv.FormatInt(fixture.legacyIDs[0], 10)+`; END $$;
		CREATE EVENT TRIGGER contract_mutate_owner ON sql_drop EXECUTE FUNCTION enrollment.contract_mutate_owner()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP EVENT TRIGGER IF EXISTS contract_mutate_owner; DROP FUNCTION IF EXISTS enrollment.contract_mutate_owner()`)
	})
	before := requestChildContractOwnerRows(t, db)
	require.ErrorContains(t, contractRequestChildStorage(t.Context(), db), "owner rows or checksums changed during Contract")
	require.True(t, requestChildCompatibilityRetained(t, db), "a failed fingerprint must roll back the DDL")
	require.Equal(t, before, requestChildContractOwnerRows(t, db))
}

func TestRequestChildContractRefusesIncompleteOwnerStorage(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		arrange string
		want    string
	}{
		{"archive FK", `CREATE TABLE enrollment.contract_old_caller (offering_id bigint REFERENCES enrollment.request_child_offerings_legacy(id))`, "foreign keys still referencing archive"},
		{"unvalidated FK", `ALTER TABLE enrollment.care_offering_bookings ADD CONSTRAINT contract_unvalidated
			FOREIGN KEY (tenant_id, request_child_id) REFERENCES enrollment.request_children(tenant_id, id) NOT VALID`, "unvalidated owner foreign keys"},
		{"RLS not forced", `ALTER TABLE enrollment.request_child_offering_selections NO FORCE ROW LEVEL SECURITY`, "owner RLS disabled"},
		{"orphan booking", `ALTER TABLE enrollment.care_offering_bookings DISABLE TRIGGER ALL;
			UPDATE enrollment.care_offering_bookings SET request_child_id = -1 WHERE id = (SELECT min(id) FROM enrollment.care_offering_bookings);
			ALTER TABLE enrollment.care_offering_bookings ENABLE TRIGGER ALL`, "bookings without request children"},
		{"orphan selection", `ALTER TABLE enrollment.request_child_offering_selections DISABLE TRIGGER ALL;
			UPDATE enrollment.request_child_offering_selections SET care_offering_id = -1 WHERE id = (SELECT min(id) FROM enrollment.request_child_offering_selections);
			ALTER TABLE enrollment.request_child_offering_selections ENABLE TRIGGER ALL`, "selections without care offerings"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, _ := requestChildContractFixture(t)
			_, err := db.ExecContext(t.Context(), scenario.arrange)
			require.NoError(t, err)
			before := requestChildContractOwnerRows(t, db)
			require.ErrorContains(t, contractRequestChildStorage(t.Context(), db), scenario.want)
			require.Equal(t, before, requestChildContractOwnerRows(t, db))
			require.True(t, requestChildCompatibilityRetained(t, db), "preflight failure must retain rollback storage")
		})
	}
}

func TestRequestChildContractHonorsCancellationBeforeDDL(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	cancel()
	require.Error(t, contractRequestChildStorage(ctx, db))
	require.NoError(t, requestChildStorageContractDataPreflight(t.Context(), db))
	require.True(t, requestChildCompatibilityRetained(t, db))
}

func TestRequestChildContractObservedLockWaitAndTimeout(t *testing.T) {
	t.Parallel()
	for _, holdUntilTimeout := range []bool{false, true} {
		name := "released"
		if holdUntilTimeout {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := requestChildContractFixture(t)
			before := requestChildContractOwnerRows(t, db)
			holder, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(t.Context(), `LOCK TABLE enrollment.care_offering_bookings IN SHARE MODE`)
			require.NoError(t, err)
			var holderPID int
			require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &holderPID))
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- contractRequestChildStorage(ctx, db) }()
			waiting := func() bool {
				var blocked bool
				err := db.NewRaw(`SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND ? = ANY(pg_blocking_pids(pid))
					AND query LIKE '%LOCK TABLE enrollment.request_child_offerings IN ACCESS EXCLUSIVE MODE%')`, holderPID).Scan(t.Context(), &blocked)
				return err == nil && blocked
			}
			require.Eventually(t, waiting, 3*time.Second, 5*time.Millisecond)
			if !holdUntilTimeout {
				require.NoError(t, holder.Commit())
			}
			select {
			case result := <-done:
				if holdUntilTimeout {
					require.ErrorContains(t, result, "lock timeout")
				} else {
					require.NoError(t, result)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Contract did not finish after lock release or timeout")
			}
			if holdUntilTimeout {
				require.NoError(t, holder.Rollback())
			}
			require.Equal(t, before, requestChildContractOwnerRows(t, db))
			require.Equal(t, holdUntilTimeout, requestChildCompatibilityRetained(t, db))
		})
	}
}

func TestRequestChildContractOrdinaryUpgradeContractsExistingStorage(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015413';
		SELECT setval('enrollment.request_child_compatibility_reads', 566);
		SELECT setval('enrollment.request_child_compatibility_writes', 12)`)
	require.NoError(t, err)
	before := requestChildContractOwnerRows(t, db)
	var output bytes.Buffer
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
	require.Contains(t, output.String(), "migration preflight OK 1.15.413")
	require.NoError(t, Migrate(t.Context(), db))
	require.Equal(t, before, requestChildContractOwnerRows(t, db))
	require.False(t, requestChildCompatibilityRetained(t, db), "ordinary upgrades must perform cleanup, not silently defer it")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015413')`).Scan(t.Context(), &applied))
	require.True(t, applied)
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
}

func TestRequestChildContractOrdinaryUpgradeRejectsUntrackedDependency(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015413';
		CREATE FUNCTION enrollment.contract_hidden_reader() RETURNS bigint LANGUAGE sql AS 'SELECT count(*) FROM enrollment.request_child_offerings'`)
	require.NoError(t, err)
	var output bytes.Buffer
	require.ErrorContains(t, migratePreflightTo(t.Context(), db, &output), "stored function still references retired storage")
	require.ErrorContains(t, Migrate(t.Context(), db), "stored function still references retired storage")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015413')`).Scan(t.Context(), &applied))
	require.False(t, applied)
	require.True(t, requestChildCompatibilityRetained(t, db))
}

func TestRequestChildContractInitialMigrationReachesContractedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var applied, removed bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM public.bun_migrations WHERE name = '001015413'),
		to_regclass('enrollment.request_child_offerings') IS NULL
		AND to_regclass('enrollment.request_child_offerings_legacy') IS NULL
		AND to_regclass('enrollment.request_child_compatibility_reads') IS NULL`).Scan(t.Context(), &applied, &removed))
	require.True(t, applied)
	require.True(t, removed)
}

func TestRequestChildContractFreshReplayRefusesUnexpectedData(t *testing.T) {
	t.Parallel()
	db, _ := requestChildContractFixture(t)
	ctx := context.WithValue(t.Context(), freshStudentStorageKey{}, true)
	require.NoError(t, requestChildStorageContractPrecondition(ctx, db), "a fresh replay has no schema to inspect before the cutover ran")
	require.ErrorContains(t, requestChildStorageContractUp(ctx, db), "initial replay contains request-child data")
	require.True(t, requestChildCompatibilityRetained(t, db))
}
