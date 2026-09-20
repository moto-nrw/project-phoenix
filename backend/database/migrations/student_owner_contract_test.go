package migrations

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentOwnerContractRechecksEvidenceBeforeDestructiveDDL(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	changed := errors.New("operational evidence changed while acquiring locks")
	checked := false
	err := contractStudentOwnerStorageChecked(t.Context(), db, func(ctx context.Context, connection bun.IDB) error {
		_, inTransaction := connection.(bun.Tx)
		require.True(t, inTransaction)
		checked = true
		return changed
	})
	require.ErrorIs(t, err, changed)
	require.True(t, checked)
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NOT NULL AND to_regclass('users.students_legacy') IS NOT NULL`).Scan(t.Context(), &retained))
	require.True(t, retained)
}

func TestStudentOwnerContractHistoricalFixtureRestoresAfterRemoval(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	require.NoError(t, contractStudentOwnerStorage(t.Context(), db))
	testpkg.RestoreStudentStorageBeforeCutover(t, db)
	_, ids := studentOwnerCutoverFixture(t, db, 2)
	require.Len(t, ids, 2)
	require.NoError(t, studentOwnerCutoverUp(t.Context(), db))
	require.NoError(t, studentCareAbsenceCompatibilityUp(t.Context(), db))
	require.NoError(t, contractStudentOwnerStorage(t.Context(), db))
}

func TestStudentOwnerContractDataPreflightDoesNotProbeCompatibilityView(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	require.NoError(t, studentOwnerContractDataPreflight(t.Context(), db))
	var reads, writes int64
	require.NoError(t, db.NewRaw(`SELECT
		coalesce(pg_sequence_last_value('users.student_compatibility_reads'), 0),
		coalesce(pg_sequence_last_value('users.student_compatibility_writes'), 0)`).Scan(t.Context(), &reads, &writes))
	require.Zero(t, reads)
	require.Zero(t, writes)
}

func TestStudentOwnerContractRejectsUntrackedFunctionDependency(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION users.contract_hidden_reader() RETURNS bigint
		LANGUAGE sql AS 'SELECT count(*) FROM users.students'`)
	require.NoError(t, err)
	require.ErrorContains(t, contractStudentOwnerStorage(t.Context(), db), "stored function still references retired storage")
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NOT NULL`).Scan(t.Context(), &retained))
	require.True(t, retained)
}

func TestStudentOwnerContractRefusesLiveCompatibilityHits(t *testing.T) {
	t.Parallel()
	for _, counter := range []string{"users.student_compatibility_reads", "users.student_compatibility_writes"} {
		t.Run(counter, func(t *testing.T) {
			db := setupStudentStorageBeforeContract(t)
			_, err := db.NewRaw(`SELECT nextval(?::regclass)`, counter).Exec(t.Context())
			require.NoError(t, err)
			require.ErrorContains(t, studentOwnerContractDataPreflight(t.Context(), db), "compatibility hits prevent Contract")
		})
	}
}

func TestStudentOwnerContractPreservesCurrentAbsenceState(t *testing.T) {
	t.Parallel()
	db, ids := studentContractFixture(t)
	// Live owner state may legitimately differ from the frozen archive. Neither
	// a missing dated status nor archive inequality authorizes overwriting it.
	_, err := db.NewRaw(`UPDATE users.student_care_profiles c SET sick = true,
		sick_since = '2026-09-19 10:12:13', excused = true, excused_since = '2026-09-18 09:08:07'
		FROM users.student_school_memberships m
		WHERE c.membership_id = m.id AND m.student_profile_id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	before := studentContractOwnerRows(t, db)
	require.NoError(t, contractStudentOwnerStorage(t.Context(), db))
	require.Equal(t, before, studentContractOwnerRows(t, db))
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NULL
		AND to_regclass('users.expired_privacy_consents') IS NULL
		AND to_regclass('users.students_legacy') IS NULL
		AND to_regclass('users.students_id_seq') IS NULL
		AND to_regclass('users.student_compatibility_reads') IS NULL
		AND to_regclass('users.student_compatibility_writes') IS NULL
		AND to_regprocedure('users.route_student_compatibility()') IS NULL`).Scan(t.Context(), &absent))
	require.True(t, absent, "all obsolete storage and compatibility objects must be gone")
}

func TestStudentOwnerContractRollsBackDDLOnUnexpectedDependentView(t *testing.T) {
	t.Parallel()
	db, _ := studentContractFixture(t)
	_, err := db.ExecContext(t.Context(), `CREATE VIEW users.contract_unexpected_dependency AS SELECT id FROM users.students_legacy`)
	require.NoError(t, err)
	before := studentContractOwnerRows(t, db)
	require.ErrorContains(t, contractStudentOwnerStorage(t.Context(), db), "depend")
	require.NoError(t, studentOwnerContractDataPreflight(t.Context(), db), "failed DDL restores the view and counters")
	require.Equal(t, before, studentContractOwnerRows(t, db))
}

func TestStudentOwnerContractRefusesUnreconciledGuardianCopy(t *testing.T) {
	t.Parallel()
	db, ids := studentContractFixture(t)
	_, err := db.NewRaw(`UPDATE users.students_legacy SET guardian_email = 'unmatched@example.test' WHERE id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	require.ErrorContains(t, contractStudentOwnerStorage(t.Context(), db), "unreconciled guardian values")
}

func studentContractFixture(t *testing.T) (*bun.DB, []int64) {
	t.Helper()
	db := setupStudentStorageBeforeCutover(t)
	_, ids := studentOwnerCutoverFixture(t, db, 3)
	require.NoError(t, studentOwnerCutoverUp(t.Context(), db))
	require.NoError(t, studentCareAbsenceCompatibilityUp(t.Context(), db))
	return db, ids
}

func setupStudentStorageBeforeContract(t *testing.T) *bun.DB {
	t.Helper()
	db := setupStudentStorageBeforeCutover(t)
	require.NoError(t, studentOwnerCutoverUp(t.Context(), db))
	require.NoError(t, studentCareAbsenceCompatibilityUp(t.Context(), db))
	return db
}

func studentContractOwnerRows(t *testing.T, db bun.IDB) []string {
	t.Helper()
	var snapshots []string
	for _, table := range []string{"users.student_profiles", "users.student_school_memberships", "users.student_care_profiles", "active.student_status_days"} {
		var snapshot string
		require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text)::text, '[]') FROM ? AS r`, bun.Ident(table)).Scan(t.Context(), &snapshot))
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func TestStudentOwnerContractHonorsCancellationBeforeDDL(t *testing.T) {
	t.Parallel()
	db, _ := studentContractFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	cancel()
	require.Error(t, contractStudentOwnerStorage(ctx, db))
	require.NoError(t, studentOwnerContractDataPreflight(t.Context(), db))
}

func TestStudentOwnerContractRefusesIncompleteOwnerStorage(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		arrange string
		want    string
	}{
		{"missing care", `DELETE FROM users.student_care_profiles`, "live memberships without care"},
		{"unvalidated FK", `ALTER TABLE users.student_school_memberships ADD CONSTRAINT contract_unvalidated
			FOREIGN KEY (tenant_id, student_profile_id) REFERENCES users.student_profiles(tenant_id, id) NOT VALID`, "unvalidated owner foreign keys"},
		{"archive FK", `CREATE TABLE users.contract_old_caller (student_id bigint REFERENCES users.students_legacy(id))`, "foreign keys still referencing archive"},
		{"RLS disabled", `ALTER TABLE users.student_care_profiles DISABLE ROW LEVEL SECURITY`, "owner RLS disabled"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, _ := studentContractFixture(t)
			_, err := db.ExecContext(t.Context(), scenario.arrange)
			require.NoError(t, err)
			before := studentContractOwnerRows(t, db)
			require.ErrorContains(t, contractStudentOwnerStorage(t.Context(), db), scenario.want)
			require.Equal(t, before, studentContractOwnerRows(t, db))
			var retained bool
			require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NOT NULL
				AND to_regclass('users.students_legacy') IS NOT NULL`).Scan(t.Context(), &retained))
			require.True(t, retained, "preflight failure must retain rollback storage")
		})
	}
}

func TestStudentOwnerContractRefusesBrokenOwnerLinks(t *testing.T) {
	t.Parallel()
	db, ids := studentContractFixture(t)
	_, err := db.ExecContext(t.Context(), `ALTER TABLE users.student_school_memberships DROP CONSTRAINT fk_student_school_memberships_profile`)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.student_school_memberships SET student_profile_id = nextval('users.student_profiles_id_seq')
		WHERE student_profile_id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	before := studentContractOwnerRows(t, db)
	require.ErrorContains(t, contractStudentOwnerStorage(t.Context(), db), "memberships without profiles")
	require.Equal(t, before, studentContractOwnerRows(t, db))
}

func TestStudentOwnerContractRefusesDuplicateLiveMemberships(t *testing.T) {
	t.Parallel()
	db, ids := studentContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DROP INDEX users.uq_student_school_memberships_active_profile`)
	require.NoError(t, err)
	var membershipID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class, status)
		SELECT tenant_id, student_profile_id, school_class, status FROM users.student_school_memberships
		WHERE student_profile_id = ? RETURNING id`, ids[0]).Scan(t.Context(), &membershipID))
	_, err = db.NewRaw(`INSERT INTO users.student_care_profiles (tenant_id, membership_id)
		SELECT tenant_id, id FROM users.student_school_memberships WHERE id = ?`, membershipID).Exec(t.Context())
	require.NoError(t, err)
	before := studentContractOwnerRows(t, db)
	require.ErrorContains(t, contractStudentOwnerStorage(t.Context(), db), "duplicate live memberships")
	require.Equal(t, before, studentContractOwnerRows(t, db))
}
