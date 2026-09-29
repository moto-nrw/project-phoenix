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
// Registered execution enters through staffOwnerContractUp.
func contractStaffOwnerStorage(ctx context.Context, db *bun.DB) error {
	return contractStaffOwnerStorageChecked(ctx, db, nil)
}

// staffContractFixture rebuilds the frozen pre-Contract world: the historical
// base table, its backfill and the committed, validated cutover with its
// compatibility view, archive, routing function and hit counters.
func staffContractFixture(t *testing.T) (*testpkg.DB, []int64) {
	t.Helper()
	db := setupIsolatedStaffStorageBeforeCutover(t)
	_, ids := staffOwnerCutoverFixture(t, db, 3)
	require.NoError(t, staffOwnerCutoverUp(t.Context(), db))
	return db, ids
}

func staffContractOwnerRows(t *testing.T, db bun.IDB) []string {
	t.Helper()
	var snapshots []string
	for _, table := range []string{
		"users.staff_school_memberships", "users.staff_employment_profiles",
		"platform.storage_backfill_checkpoints",
	} {
		var snapshot string
		require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text)::text, '[]') FROM ? AS r`, bun.Ident(table)).Scan(t.Context(), &snapshot))
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func staffCompatibilityRetained(t *testing.T, db bun.IDB) bool {
	t.Helper()
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.staff') IS NOT NULL
		AND to_regclass('users.staff_legacy') IS NOT NULL
		AND to_regclass('users.staff_compatibility_reads') IS NOT NULL
		AND to_regclass('users.staff_compatibility_writes') IS NOT NULL
		AND to_regprocedure('users.route_staff_compatibility()') IS NOT NULL`).Scan(t.Context(), &retained))
	return retained
}

func TestStaffContractRunsLockedCheckBeforeDestructiveDDL(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	refused := errors.New("locked replay check refused cleanup")
	checked := false
	err := contractStaffOwnerStorageChecked(t.Context(), db, func(ctx context.Context, connection bun.IDB) error {
		_, inTransaction := connection.(bun.Tx)
		require.True(t, inTransaction)
		checked = true
		return refused
	})
	require.ErrorIs(t, err, refused)
	require.True(t, checked)
	require.True(t, staffCompatibilityRetained(t, db))
}

func TestStaffContractRemovesCompatibilityObjectsAndPreservesOwners(t *testing.T) {
	t.Parallel()
	db, ids := staffContractFixture(t)
	// A current-image owner write after cutover legitimately differs from the
	// frozen archive. Neither the difference nor archive content may leak
	// into owner storage during Contract.
	_, err := db.ExecContext(t.Context(), `UPDATE users.staff_employment_profiles SET staff_notes = 'after cutover' WHERE membership_id = ?`, ids[0])
	require.NoError(t, err)
	before := staffContractOwnerRows(t, db)
	require.NoError(t, contractStaffOwnerStorage(t.Context(), db))
	require.Equal(t, before, staffContractOwnerRows(t, db))
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.staff') IS NULL
		AND to_regclass('users.staff_legacy') IS NULL
		AND to_regclass('users.staff_id_seq') IS NULL
		AND to_regclass('users.staff_compatibility_reads') IS NULL
		AND to_regclass('users.staff_compatibility_writes') IS NULL
		AND to_regprocedure('users.route_staff_compatibility()') IS NULL`).Scan(t.Context(), &absent))
	require.True(t, absent, "all obsolete storage and compatibility objects must be gone")
	// The personnel-number rule, the membership updated_at trigger and the
	// index names the owners classify conflicts by are current behaviour.
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.staff_school_memberships_id_seq') IS NOT NULL
		AND to_regclass('users.idx_staff_tenant_person') IS NOT NULL
		AND to_regprocedure('users.enforce_staff_personnel_number()') IS NOT NULL
		AND EXISTS (SELECT FROM pg_trigger WHERE tgname = 'staff_employment_profiles_personnel_number'
			AND tgrelid = 'users.staff_employment_profiles'::regclass)
		AND EXISTS (SELECT FROM pg_trigger WHERE tgname = 'update_staff_school_memberships_updated_at'
			AND tgrelid = 'users.staff_school_memberships'::regclass)`).Scan(t.Context(), &retained))
	require.True(t, retained, "owner storage and its invariants must survive")
	require.ErrorContains(t, staffOwnerContractDown(t.Context(), db), "restore the verified pre-Contract backup")
	require.Equal(t, before, staffContractOwnerRows(t, db), "refused Down must not rewrite current owner data")
}

func TestStaffContractKeepsDependentForeignKeysOnTheMembership(t *testing.T) {
	t.Parallel()
	db, ids := staffContractFixture(t)
	var before int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE contype = 'f' AND confrelid = 'users.staff_school_memberships'::regclass`).Scan(t.Context(), &before))
	require.NoError(t, contractStaffOwnerStorage(t.Context(), db))
	var after int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE contype = 'f' AND confrelid = 'users.staff_school_memberships'::regclass`).Scan(t.Context(), &after))
	require.Equal(t, before, after)
	// A dependent row of a staff member created after the Contract still
	// resolves through the membership, and deletion still cascades.
	_, err := db.ExecContext(t.Context(), `INSERT INTO config.staff_work_schedules (tenant_id, staff_id, day_of_week, target_minutes, valid_from)
		SELECT tenant_id, id, 1, 240, '2026-09-01' FROM users.staff_school_memberships WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.staff_school_memberships WHERE id = ?`, ids[0])
	require.NoError(t, err)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM config.staff_work_schedules WHERE staff_id = ?0)
		+ (SELECT count(*) FROM users.staff_employment_profiles WHERE membership_id = ?0)`, ids[0]).Scan(t.Context(), &remaining))
	require.Zero(t, remaining)
}

func TestStaffContractHistoricalFixtureRestoresAfterRemoval(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	require.NoError(t, contractStaffOwnerStorage(t.Context(), db))
	testpkg.RestoreStaffStorageBeforeCutover(t, db)
	// The first fixture's work-time model stays in the default school.
	ids := staffOwnerFixture(t, db, staffOwnerSecondTenant(t, db), 2)
	require.Len(t, ids, 2)
	_, err := RunStaffOwnerBackfill(t.Context(), db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	require.NoError(t, staffOwnerCutoverUp(t.Context(), db))
	require.NoError(t, contractStaffOwnerStorage(t.Context(), db))
}

func TestStaffContractDataPreflightDoesNotProbeCompatibilityView(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	reads, writes := staffCompatibilityHits(t, db)
	require.NoError(t, staffOwnerContractDataPreflight(t.Context(), db))
	readsAfter, writesAfter := staffCompatibilityHits(t, db)
	require.Equal(t, reads, readsAfter)
	require.Equal(t, writes, writesAfter)
}

func TestStaffContractAllowsHistoricalCompatibilityHits(t *testing.T) {
	t.Parallel()
	for _, counter := range []string{"users.staff_compatibility_reads", "users.staff_compatibility_writes"} {
		t.Run(counter, func(t *testing.T) {
			db, _ := staffContractFixture(t)
			_, err := db.NewRaw(`SELECT nextval(?::regclass)`, counter).Exec(t.Context())
			require.NoError(t, err)
			require.NoError(t, staffOwnerContractDataPreflight(t.Context(), db))
			require.NoError(t, contractStaffOwnerStorage(t.Context(), db))
		})
	}
}

func TestStaffContractRejectsUntrackedFunctionDependency(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"view":    `SELECT count(*) FROM users.staff`,
		"archive": `SELECT count(*) FROM users.staff_legacy`,
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := staffContractFixture(t)
			_, err := db.NewRaw(`CREATE FUNCTION users.contract_hidden_reader() RETURNS bigint LANGUAGE sql AS ?`, body).Exec(t.Context())
			require.NoError(t, err)
			require.ErrorContains(t, contractStaffOwnerStorage(t.Context(), db), "stored function still references retired storage")
			require.True(t, staffCompatibilityRetained(t, db))
		})
	}
}

func TestStaffContractRefusesUnexpectedDependentView(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	_, err := db.ExecContext(t.Context(), `CREATE VIEW users.contract_unexpected_dependency AS SELECT id FROM users.staff_legacy`)
	require.NoError(t, err)
	before := staffContractOwnerRows(t, db)
	require.ErrorContains(t, contractStaffOwnerStorage(t.Context(), db), "unexpected views depend on retired storage")
	require.True(t, staffCompatibilityRetained(t, db))
	require.Equal(t, before, staffContractOwnerRows(t, db))
}

func TestStaffContractRollsBackDDLOnFailedFingerprint(t *testing.T) {
	t.Parallel()
	db, ids := staffContractFixture(t)
	// The obsolete objects are already dropped inside the transaction when the
	// fingerprint comparison runs; a change of owner rows must roll every
	// DROP back rather than commit a partially cleaned schema.
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION users.contract_mutate_owner() RETURNS event_trigger LANGUAGE plpgsql AS $$
		BEGIN UPDATE users.staff_employment_profiles SET staff_notes = staff_notes || '!' WHERE membership_id = `+strconv.FormatInt(ids[0], 10)+`; END $$;
		CREATE EVENT TRIGGER contract_mutate_staff_owner ON sql_drop EXECUTE FUNCTION users.contract_mutate_owner()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP EVENT TRIGGER IF EXISTS contract_mutate_staff_owner; DROP FUNCTION IF EXISTS users.contract_mutate_owner()`)
	})
	before := staffContractOwnerRows(t, db)
	require.ErrorContains(t, contractStaffOwnerStorage(t.Context(), db), "owner rows or checksums changed during Contract")
	require.True(t, staffCompatibilityRetained(t, db), "a failed fingerprint must roll back the DDL")
	require.Equal(t, before, staffContractOwnerRows(t, db))
}

func TestStaffContractRefusesIncompleteOwnerStorage(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		arrange string
		want    string
	}{
		{"archive FK", `CREATE TABLE users.contract_old_caller (staff_id bigint REFERENCES users.staff_legacy(id))`, "foreign keys still referencing archive"},
		{"unvalidated FK", `ALTER TABLE users.staff_employment_profiles ADD CONSTRAINT contract_unvalidated
			FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) NOT VALID`, "unvalidated owner foreign keys"},
		{"RLS not forced", `ALTER TABLE users.staff_school_memberships NO FORCE ROW LEVEL SECURITY`, "owner RLS disabled"},
		{"membership without profile", `DELETE FROM users.staff_employment_profiles
			WHERE membership_id = (SELECT max(id) FROM users.staff_school_memberships)`, "memberships without employment profiles"},
		{"membership without person", `ALTER TABLE users.staff_school_memberships DISABLE TRIGGER ALL;
			UPDATE users.staff_school_memberships SET person_id = -1 WHERE id = (SELECT max(id) FROM users.staff_school_memberships);
			ALTER TABLE users.staff_school_memberships ENABLE TRIGGER ALL`, "memberships without persons"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, _ := staffContractFixture(t)
			_, err := db.ExecContext(t.Context(), scenario.arrange)
			require.NoError(t, err)
			before := staffContractOwnerRows(t, db)
			require.ErrorContains(t, contractStaffOwnerStorage(t.Context(), db), scenario.want)
			require.Equal(t, before, staffContractOwnerRows(t, db))
			require.True(t, staffCompatibilityRetained(t, db), "preflight failure must retain rollback storage")
		})
	}
}

func TestStaffContractHonorsCancellationBeforeDDL(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	cancel()
	require.Error(t, contractStaffOwnerStorage(ctx, db))
	require.NoError(t, staffOwnerContractDataPreflight(t.Context(), db))
	require.True(t, staffCompatibilityRetained(t, db))
}

func TestStaffContractObservedLockWaitAndTimeout(t *testing.T) {
	t.Parallel()
	for _, holdUntilTimeout := range []bool{false, true} {
		name := "released"
		if holdUntilTimeout {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := staffContractFixture(t)
			before := staffContractOwnerRows(t, db)
			holder, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(t.Context(), `LOCK TABLE users.staff_employment_profiles IN SHARE MODE`)
			require.NoError(t, err)
			var holderPID int
			require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &holderPID))
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- contractStaffOwnerStorage(ctx, db) }()
			waiting := func() bool {
				var blocked bool
				err := db.NewRaw(`SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND ? = ANY(pg_blocking_pids(pid))
					AND query LIKE '%LOCK TABLE users.staff IN ACCESS EXCLUSIVE MODE%')`, holderPID).Scan(t.Context(), &blocked)
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
			require.Equal(t, before, staffContractOwnerRows(t, db))
			require.Equal(t, holdUntilTimeout, staffCompatibilityRetained(t, db))
		})
	}
}

func TestStaffContractOrdinaryUpgradeContractsExistingStorage(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015427';
		SELECT setval('users.staff_compatibility_reads', 566);
		SELECT setval('users.staff_compatibility_writes', 12)`)
	require.NoError(t, err)
	before := staffContractOwnerRows(t, db)
	var output bytes.Buffer
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
	require.Contains(t, output.String(), "migration preflight OK 1.15.427")
	require.NoError(t, Migrate(t.Context(), db))
	require.Equal(t, before, staffContractOwnerRows(t, db))
	require.False(t, staffCompatibilityRetained(t, db), "ordinary upgrades must perform cleanup, not silently defer it")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015427')`).Scan(t.Context(), &applied))
	require.True(t, applied)
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
}

func TestStaffContractOrdinaryUpgradeRejectsUntrackedDependency(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015427';
		CREATE FUNCTION users.contract_hidden_reader() RETURNS bigint LANGUAGE sql AS 'SELECT count(*) FROM users.staff'`)
	require.NoError(t, err)
	var output bytes.Buffer
	require.ErrorContains(t, migratePreflightTo(t.Context(), db, &output), "stored function still references retired storage")
	require.ErrorContains(t, Migrate(t.Context(), db), "stored function still references retired storage")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015427')`).Scan(t.Context(), &applied))
	require.False(t, applied)
	require.True(t, staffCompatibilityRetained(t, db))
}

func TestStaffContractInitialMigrationReachesContractedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var applied, removed bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM public.bun_migrations WHERE name = '001015427'),
		to_regclass('users.staff') IS NULL
		AND to_regclass('users.staff_legacy') IS NULL
		AND to_regclass('users.staff_compatibility_reads') IS NULL`).Scan(t.Context(), &applied, &removed))
	require.True(t, applied)
	require.True(t, removed)
}

func TestStaffContractFreshReplayRefusesUnexpectedData(t *testing.T) {
	t.Parallel()
	db, _ := staffContractFixture(t)
	ctx := context.WithValue(t.Context(), freshStudentStorageKey{}, true)
	require.NoError(t, staffOwnerContractPrecondition(ctx, db), "a fresh replay has no schema to inspect before the cutover ran")
	require.ErrorContains(t, staffOwnerContractUp(ctx, db), "initial replay contains staff data")
	require.True(t, staffCompatibilityRetained(t, db))
}
