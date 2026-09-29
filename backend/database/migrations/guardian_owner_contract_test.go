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

// Migrations older than the Contract read and write users.students_guardians.
// Their tests restore the rollback mirror inside their own clone.
func setupGuardianStorageBeforeContract(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestoreGuardianStorageBeforeContract(t, db)
	return db
}

// Isolated DDL tests use the same live checks as ordinary deployments.
// Registered execution enters through guardianOwnerContractUp.
func contractGuardianOwnerStorage(ctx context.Context, db *bun.DB) error {
	return contractGuardianOwnerStorageChecked(ctx, db, nil)
}

// guardianContractFixture rebuilds the frozen pre-Contract world: the
// historical base table, its backfill and the committed, validated cutover
// with its rollback mirror, routing and mirror triggers and write counter.
func guardianContractFixture(t *testing.T) (*testpkg.DB, int64, []int64) {
	t.Helper()
	db := setupGuardianStorageBeforeCutover(t)
	tenantID, ids := guardianCutoverFixture(t, db, 3)
	require.NoError(t, guardianOwnerCutoverUp(t.Context(), db))
	return db, tenantID, ids
}

func guardianContractOwnerRows(t *testing.T, db bun.IDB) []string {
	t.Helper()
	var snapshots []string
	for _, table := range []string{
		"users.student_guardian_relationships", "users.student_guardian_pickup_permissions",
		"auth.guardian_student_access", "platform.storage_backfill_checkpoints",
	} {
		var snapshot string
		require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text)::text, '[]') FROM ? AS r`, bun.Ident(table)).Scan(t.Context(), &snapshot))
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func guardianCompatibilityRetained(t *testing.T, db bun.IDB) bool {
	t.Helper()
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students_guardians') IS NOT NULL
		AND to_regclass('users.students_guardians_compatibility_writes') IS NOT NULL
		AND to_regprocedure('users.route_students_guardians_compatibility()') IS NOT NULL
		AND EXISTS (SELECT FROM pg_trigger WHERE tgname = 'student_guardian_relationships_mirror')`).Scan(t.Context(), &retained))
	return retained
}

func TestGuardianContractRunsLockedCheckBeforeDestructiveDDL(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	refused := errors.New("locked replay check refused cleanup")
	checked := false
	err := contractGuardianOwnerStorageChecked(t.Context(), db, func(ctx context.Context, connection bun.IDB) error {
		_, inTransaction := connection.(bun.Tx)
		require.True(t, inTransaction)
		checked = true
		return refused
	})
	require.ErrorIs(t, err, refused)
	require.True(t, checked)
	require.True(t, guardianCompatibilityRetained(t, db))
}

func TestGuardianContractRemovesCompatibilityObjectsAndPreservesOwners(t *testing.T) {
	t.Parallel()
	db, _, ids := guardianContractFixture(t)
	// A current-image owner write after cutover reaches the mirror through its
	// trigger; the Contract must neither lose it nor touch any owner row.
	_, err := db.ExecContext(t.Context(), `UPDATE users.student_guardian_pickup_permissions SET pickup_notes = 'nach dem Cutover' WHERE relationship_id = ?`, ids[0])
	require.NoError(t, err)
	before := guardianContractOwnerRows(t, db)
	require.NoError(t, contractGuardianOwnerStorage(t.Context(), db))
	require.Equal(t, before, guardianContractOwnerRows(t, db))
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students_guardians') IS NULL
		AND to_regclass('users.students_guardians_id_seq') IS NULL
		AND to_regclass('users.students_guardians_compatibility_writes') IS NULL
		AND to_regprocedure('users.route_students_guardians_compatibility()') IS NULL
		AND to_regprocedure('users.mirror_student_guardian_relationship()') IS NULL
		AND to_regprocedure('users.mirror_student_guardian_pickup_permission()') IS NULL
		AND to_regprocedure('auth.mirror_guardian_student_access()') IS NULL
		AND to_regprocedure('users.enforce_single_primary_student_guardian()') IS NULL
		AND to_regprocedure('meta.invalidate_parent_student_consent_permission_grant()') IS NULL
		AND to_regprocedure('meta.invalidate_meal_participation_permission_grant()') IS NULL
		AND NOT EXISTS (SELECT FROM pg_trigger WHERE tgname IN ('student_guardian_relationships_mirror',
			'student_guardian_pickup_permissions_mirror', 'guardian_student_access_mirror'))`).Scan(t.Context(), &absent))
	require.True(t, absent, "all obsolete storage and compatibility objects must be gone")
	// The owners' own triggers, the account binding and the grant
	// invalidation on the access row are current behaviour.
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.student_guardian_relationships_id_seq') IS NOT NULL
		AND to_regprocedure('auth.bind_guardian_student_access_account()') IS NOT NULL
		AND to_regprocedure('meta.invalidate_parent_student_consent_access_grant()') IS NOT NULL
		AND to_regprocedure('meta.invalidate_meal_participation_access_grant()') IS NOT NULL
		AND (SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgname IN (
			'update_student_guardian_relationships_updated_at', 'update_student_guardian_pickup_permissions_updated_at',
			'update_guardian_student_access_updated_at', 'guardian_profiles_bind_student_access',
			'invalidate_parent_student_consent_permission_grant', 'invalidate_meal_participation_permission_grant')) = 6`).
		Scan(t.Context(), &retained))
	require.True(t, retained, "owner storage and its invariants must survive")
	require.ErrorContains(t, guardianOwnerContractDown(t.Context(), db), "restore the verified pre-Contract backup")
	require.Equal(t, before, guardianContractOwnerRows(t, db), "refused Down must not rewrite current owner data")
}

func TestGuardianContractKeepsOwnerWritesWorking(t *testing.T) {
	t.Parallel()
	db, tenantID, ids := guardianContractFixture(t)
	require.NoError(t, contractGuardianOwnerStorage(t.Context(), db))
	// Grants still name the relationship, and deleting it still cascades
	// through the pickup permission, the access row and the grants.
	var grantForeignKeys int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND convalidated
		AND confrelid = 'users.student_guardian_relationships'::regclass
		AND conname IN ('parent_student_consent_permission_gran_student_guardian_id_fkey',
			'meal_participation_permission_grants_student_guardian_id_fkey')`).Scan(t.Context(), &grantForeignKeys))
	require.Equal(t, 2, grantForeignKeys)
	_, err := db.ExecContext(t.Context(), `UPDATE users.student_guardian_relationships SET guardian_role = 'custom' WHERE id = ?`, ids[1])
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.student_guardian_relationships WHERE tenant_id = ? AND id = ?`, tenantID, ids[0])
	require.NoError(t, err)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM users.student_guardian_pickup_permissions WHERE relationship_id = ?0)
		+ (SELECT count(*) FROM auth.guardian_student_access WHERE relationship_id = ?0)`, ids[0]).Scan(t.Context(), &remaining))
	require.Zero(t, remaining)
}

func TestGuardianContractHistoricalFixtureRestoresAfterRemoval(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	require.NoError(t, contractGuardianOwnerStorage(t.Context(), db))
	testpkg.RestoreGuardianStorageBeforeContract(t, db)
	require.NoError(t, guardianOwnerContractDataPreflight(t.Context(), db))
	require.NoError(t, contractGuardianOwnerStorage(t.Context(), db))
}

func TestGuardianContractAllowsHistoricalCompatibilityWrites(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	_, err := db.ExecContext(t.Context(), `SELECT setval('users.students_guardians_compatibility_writes', 12)`)
	require.NoError(t, err)
	require.NoError(t, guardianOwnerContractDataPreflight(t.Context(), db))
	require.NoError(t, contractGuardianOwnerStorage(t.Context(), db))
}

func TestGuardianContractRejectsUntrackedFunctionDependency(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION users.contract_hidden_reader() RETURNS bigint LANGUAGE sql AS 'SELECT count(*) FROM users.students_guardians'`)
	require.NoError(t, err)
	require.ErrorContains(t, contractGuardianOwnerStorage(t.Context(), db), "stored function still references retired storage")
	require.True(t, guardianCompatibilityRetained(t, db))
}

func TestGuardianContractRefusesUnexpectedDependentView(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	_, err := db.ExecContext(t.Context(), `CREATE VIEW users.contract_unexpected_dependency AS SELECT id FROM users.students_guardians`)
	require.NoError(t, err)
	before := guardianContractOwnerRows(t, db)
	require.ErrorContains(t, contractGuardianOwnerStorage(t.Context(), db), "views depend on the mirror")
	require.True(t, guardianCompatibilityRetained(t, db))
	require.Equal(t, before, guardianContractOwnerRows(t, db))
}

func TestGuardianContractRollsBackDDLOnFailedFingerprint(t *testing.T) {
	t.Parallel()
	db, _, ids := guardianContractFixture(t)
	// The obsolete objects are already dropped inside the transaction when the
	// fingerprint comparison runs; a change of owner rows must roll every
	// DROP back rather than commit a partially cleaned schema.
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION users.contract_mutate_owner() RETURNS event_trigger LANGUAGE plpgsql AS $$
		BEGIN UPDATE users.student_guardian_pickup_permissions SET pickup_notes = coalesce(pickup_notes, '') || '!' WHERE relationship_id = `+strconv.FormatInt(ids[0], 10)+`; END $$;
		CREATE EVENT TRIGGER contract_mutate_guardian_owner ON sql_drop EXECUTE FUNCTION users.contract_mutate_owner()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP EVENT TRIGGER IF EXISTS contract_mutate_guardian_owner; DROP FUNCTION IF EXISTS users.contract_mutate_owner()`)
	})
	before := guardianContractOwnerRows(t, db)
	require.ErrorContains(t, contractGuardianOwnerStorage(t.Context(), db), "owner rows or checksums changed during Contract")
	require.True(t, guardianCompatibilityRetained(t, db), "a failed fingerprint must roll back the DDL")
	require.Equal(t, before, guardianContractOwnerRows(t, db))
}

func TestGuardianContractRefusesIncompleteOwnerStorage(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		arrange string
		want    string
	}{
		{"mirror FK", `CREATE TABLE users.contract_old_caller (link_id bigint REFERENCES users.students_guardians(id))`, "foreign keys still referencing the mirror"},
		{"unvalidated FK", `ALTER TABLE users.student_guardian_pickup_permissions ADD CONSTRAINT contract_unvalidated
			FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) NOT VALID`, "unvalidated owner foreign keys"},
		{"RLS not forced", `ALTER TABLE auth.guardian_student_access NO FORCE ROW LEVEL SECURITY`, "owner RLS disabled"},
		// The mirror triggers never mirrored a standalone pickup or access
		// delete (#3539 review); the Contract refuses such a link instead.
		{"pickup permission deleted", `DELETE FROM users.student_guardian_pickup_permissions
			WHERE relationship_id = (SELECT max(id) FROM users.student_guardian_relationships)`, "incomplete links"},
		{"access row deleted", `DELETE FROM auth.guardian_student_access
			WHERE relationship_id = (SELECT max(id) FROM users.student_guardian_relationships)`, "incomplete links"},
		{"mirror drift", `ALTER TABLE users.students_guardians DISABLE TRIGGER students_guardians_route_compatibility;
			UPDATE users.students_guardians SET pickup_notes = 'nur im Spiegel' WHERE id = (SELECT max(id) FROM users.students_guardians);
			ALTER TABLE users.students_guardians ENABLE TRIGGER students_guardians_route_compatibility`, "mirror drift"},
		{"binding drift", `UPDATE auth.guardian_student_access AS a
			SET account_id = CASE WHEN g.account_id IS NULL THEN (SELECT min(id) FROM auth.accounts) END
			FROM users.student_guardian_relationships AS r
			JOIN users.guardian_profiles AS g ON g.tenant_id = r.tenant_id AND g.id = r.guardian_profile_id
			WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
			AND a.relationship_id = (SELECT max(id) FROM users.student_guardian_relationships)`, "access rows not bound"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, _, _ := guardianContractFixture(t)
			_, err := db.ExecContext(t.Context(), scenario.arrange)
			require.NoError(t, err)
			before := guardianContractOwnerRows(t, db)
			require.ErrorContains(t, contractGuardianOwnerStorage(t.Context(), db), scenario.want)
			require.Equal(t, before, guardianContractOwnerRows(t, db))
			require.True(t, guardianCompatibilityRetained(t, db), "preflight failure must retain rollback storage")
		})
	}
}

func TestGuardianContractHonorsCancellationBeforeDDL(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	cancel()
	require.Error(t, contractGuardianOwnerStorage(ctx, db))
	require.NoError(t, guardianOwnerContractDataPreflight(t.Context(), db))
	require.True(t, guardianCompatibilityRetained(t, db))
}

func TestGuardianContractObservedLockWaitAndTimeout(t *testing.T) {
	t.Parallel()
	for _, holdUntilTimeout := range []bool{false, true} {
		name := "released"
		if holdUntilTimeout {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			db, _, _ := guardianContractFixture(t)
			before := guardianContractOwnerRows(t, db)
			holder, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(t.Context(), `LOCK TABLE auth.guardian_student_access IN SHARE MODE`)
			require.NoError(t, err)
			var holderPID int
			require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &holderPID))
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- contractGuardianOwnerStorage(ctx, db) }()
			waiting := func() bool {
				var blocked bool
				err := db.NewRaw(`SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND ? = ANY(pg_blocking_pids(pid))
					AND query LIKE '%LOCK TABLE users.students_guardians%')`, holderPID).Scan(t.Context(), &blocked)
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
			require.Equal(t, before, guardianContractOwnerRows(t, db))
			require.Equal(t, holdUntilTimeout, guardianCompatibilityRetained(t, db))
		})
	}
}

func TestGuardianContractOrdinaryUpgradeContractsExistingStorage(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015428';
		SELECT setval('users.students_guardians_compatibility_writes', 7)`)
	require.NoError(t, err)
	before := guardianContractOwnerRows(t, db)
	var output bytes.Buffer
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
	require.Contains(t, output.String(), "migration preflight OK 1.15.428")
	require.NoError(t, Migrate(t.Context(), db))
	require.Equal(t, before, guardianContractOwnerRows(t, db))
	require.False(t, guardianCompatibilityRetained(t, db), "ordinary upgrades must perform cleanup, not silently defer it")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015428')`).Scan(t.Context(), &applied))
	require.True(t, applied)
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
}

func TestGuardianContractOrdinaryUpgradeRejectsIncompleteLink(t *testing.T) {
	t.Parallel()
	db, _, ids := guardianContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015428';
		DELETE FROM auth.guardian_student_access WHERE relationship_id = `+strconv.FormatInt(ids[0], 10))
	require.NoError(t, err)
	var output bytes.Buffer
	require.ErrorContains(t, migratePreflightTo(t.Context(), db, &output), "incomplete links")
	require.ErrorContains(t, Migrate(t.Context(), db), "incomplete links")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015428')`).Scan(t.Context(), &applied))
	require.False(t, applied)
	require.True(t, guardianCompatibilityRetained(t, db))
}

func TestGuardianContractInitialMigrationReachesContractedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var applied, removed bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM public.bun_migrations WHERE name = '001015428'),
		to_regclass('users.students_guardians') IS NULL
		AND to_regclass('users.students_guardians_compatibility_writes') IS NULL`).Scan(t.Context(), &applied, &removed))
	require.True(t, applied)
	require.True(t, removed)
}

func TestGuardianContractFreshReplayRefusesUnexpectedData(t *testing.T) {
	t.Parallel()
	db, _, _ := guardianContractFixture(t)
	ctx := context.WithValue(t.Context(), freshStudentStorageKey{}, true)
	require.NoError(t, guardianOwnerContractPrecondition(ctx, db), "a fresh replay has no schema to inspect before the cutover ran")
	require.ErrorContains(t, guardianOwnerContractUp(ctx, db), "initial replay contains guardian links")
	require.True(t, guardianCompatibilityRetained(t, db))
}
