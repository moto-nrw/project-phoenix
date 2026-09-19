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

// The Expand and Backfill contracts describe a world in which users.students is
// still the authoritative base table. Restore that world inside the disposable
// clone so those tests keep testing what they were written for; production
// rollback deliberately retains the compatibility shape instead.
func setupStudentStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.RestoreStudentStorageBeforeCutover(t, db)
	return db
}

// The historical Expand contracts take an explicit clone; they keep it.
func setupIsolatedStudentStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestoreStudentStorageBeforeCutover(t, db)
	return db
}

// studentOwnerCutoverFixture returns a backfilled tenant that is ready to be
// switched.
func studentOwnerCutoverFixture(t *testing.T, db *testpkg.DB, count int) (int64, []int64) {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, count)
	_, err := RunStudentOwnerBackfill(t.Context(), db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	return tenantID, ids
}

func studentRowsJSON(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(
		`SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY s.id)::text, '[]') FROM users.students s WHERE s.tenant_id = ?`,
		tenantID).Scan(t.Context(), &rows))
	return rows
}

func TestStudentOwnerCutoverPreservesContractGolden(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, _ := studentOwnerCutoverFixture(t, db, 6)
	before := studentRowsJSON(t, db, tenantID)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	require.JSONEq(t, before, studentRowsJSON(t, db, tenantID),
		"the previous image must read the same columns, NULLs and JSON shapes through the compatibility view")

	var relkind string
	require.NoError(t, db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students'::regclass`).Scan(t.Context(), &relkind))
	require.Equal(t, "v", relkind)
	_, err := RunStudentOwnerBackfill(t.Context(), db, StudentOwnerBackfillOptions{})
	require.ErrorContains(t, err, "not a base table")
	require.ErrorContains(t, ResetStudentOwnerBackfill(t.Context(), db), "not a base table")
}

func TestStudentOwnerCutoverRequiresCompletedBackfill(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	studentOwnerFixture(t, db, testpkg.Tenant(t), 2)
	switched := false
	err := finalizeStudentOwnerStorage(t.Context(), db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.ErrorContains(t, err, "requires a completed backfill pass")
	require.False(t, switched)
}

func TestStudentOwnerCutoverRejectsDrift(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 3)
	// A target-only edit is drift the final delta cannot reconcile: the copy
	// only writes rows the source still names, so the switch has to refuse.
	_, err := db.NewRaw(`UPDATE users.student_care_profiles SET health_info = 'drifted' WHERE tenant_id = ? AND membership_id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.students SET updated_at = updated_at WHERE id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	switched := false
	err = finalizeStudentOwnerStorage(t.Context(), db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.NoError(t, err, "the final delta reconciles a target-only edit of a row the source still holds")
	require.True(t, switched)
}

func TestStudentOwnerCutoverRefusesUnreconciledGuardianValues(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	_, err := db.NewRaw(`UPDATE users.students SET guardian_email = 'nobody@example.org' WHERE tenant_id = ? AND id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	switched := false
	err = finalizeStudentOwnerStorage(t.Context(), db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.ErrorContains(t, err, "is not reproducible")
	require.ErrorContains(t, err, "1 unreconciled guardian values")
	require.False(t, switched, "a legacy contact without a counterpart must block the switch, not be dropped")
}

func TestStudentOwnerCutoverFinalDeltaRollsBackWithTheSwitch(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 4)
	_, err := db.NewRaw(`UPDATE users.students SET health_info = 'last minute' WHERE id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	before := studentOwnerCheckpointRow(t, db, tenantID)

	injected := errors.New("switch failed after the final delta")
	err = finalizeStudentOwnerStorage(t.Context(), db, func(ctx context.Context, tx bun.Tx) error {
		verification, verifyErr := verifyStudentOwnerTenant(ctx, tx, tenantID)
		require.NoError(t, verifyErr)
		require.True(t, verification.Equal(), verification.Describe())
		return injected
	})
	require.ErrorIs(t, err, injected)
	require.Equal(t, before, studentOwnerCheckpointRow(t, db, tenantID),
		"a failed switch must roll back its checkpoint evidence")
	var copied string
	require.NoError(t, db.NewRaw(`SELECT coalesce(health_info, '') FROM users.student_care_profiles WHERE membership_id = ?`, ids[0]).
		Scan(t.Context(), &copied))
	require.NotEqual(t, "last minute", copied, "a failed switch must roll back its final delta")

	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility),
		"the retry must reconcile the same delta again")
	require.NoError(t, db.NewRaw(`SELECT coalesce(health_info, '') FROM users.student_care_profiles WHERE membership_id = ?`, ids[0]).
		Scan(t.Context(), &copied))
	require.Equal(t, "last minute", copied)
	after := studentOwnerCheckpointRow(t, db, tenantID)
	require.Equal(t, after.SourceChecksum, after.TargetChecksum)
	require.Zero(t, after.MismatchCount)
	require.True(t, after.Verified())
}

func studentOwnerCheckpointRow(t *testing.T, db *testpkg.DB, tenantID int64) StudentOwnerBackfillCheckpoint {
	t.Helper()
	cp := new(StudentOwnerBackfillCheckpoint)
	require.NoError(t, db.NewSelect().Model(cp).
		Where("backfill = ? AND tenant_id = ?", StudentOwnerBackfillName, tenantID).Scan(t.Context()))
	return *cp
}

func TestStudentOwnerCutoverRoutesPreviousImageWrites(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	ctx := t.Context()

	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Rollback", "Child")
	var created int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.students (tenant_id, person_id, school_class, status, guardian_name, health_info)
		VALUES (?, ?, '4b', 'active', 'Legacy Guardian', 'asthma') RETURNING id`,
		tenantID, person.ID).Scan(ctx, &created))
	require.Positive(t, created)

	var profiles, memberships, care int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM users.student_profiles WHERE id = ?0),
		(SELECT count(*) FROM users.student_school_memberships WHERE student_profile_id = ?0 AND deleted_at IS NULL),
		(SELECT count(*) FROM users.student_care_profiles c JOIN users.student_school_memberships m
			ON m.id = c.membership_id WHERE m.student_profile_id = ?0)`, created).
		Scan(ctx, &profiles, &memberships, &care))
	require.Equal(t, 1, profiles)
	require.Equal(t, 1, memberships)
	require.Equal(t, 1, care)

	var archivedGuardian, careHealth string
	require.NoError(t, db.NewRaw(`SELECT guardian_name FROM users.students_legacy WHERE id = ?`, created).Scan(ctx, &archivedGuardian))
	require.Equal(t, "Legacy Guardian", archivedGuardian, "a column without a target keeps its value in the rollback archive")
	require.NoError(t, db.NewRaw(`SELECT c.health_info FROM users.student_care_profiles c
		JOIN users.student_school_memberships m ON m.id = c.membership_id WHERE m.student_profile_id = ?`, created).Scan(ctx, &careHealth))
	require.Equal(t, "asthma", careHealth, "a Care Plan column is routed to its owner, not to the archive")

	_, err := db.NewRaw(`UPDATE users.students SET school_class = '5c', supervisor_notes = 'moved', guardian_phone = '+49 30 123456'
		WHERE id = ?`, created).Exec(ctx)
	require.NoError(t, err)
	var class, notes, phone string
	require.NoError(t, db.NewRaw(`SELECT m.school_class, c.supervisor_notes,
		(SELECT guardian_phone FROM users.students_legacy WHERE id = ?0)
		FROM users.student_school_memberships m JOIN users.student_care_profiles c ON c.membership_id = m.id
		WHERE m.student_profile_id = ?0 AND m.deleted_at IS NULL`, created).Scan(ctx, &class, &notes, &phone))
	require.Equal(t, "5c", class)
	require.Equal(t, "moved", notes)
	require.Equal(t, "+49 30 123456", phone)

	_, err = db.NewRaw(`UPDATE users.students SET id = id + 1000 WHERE id = ?`, created).Exec(ctx)
	require.ErrorContains(t, err, "student identity is immutable")

	require.NoError(t, withPhoenixTenantTx(t, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		var locked int64
		return tx.NewRaw(`SELECT s.id FROM users.students s WHERE s.id = ? FOR UPDATE`, ids[0]).Scan(ctx, &locked)
	}), "the previous image's row lock must remain supported")

	_, err = db.NewRaw(`DELETE FROM users.students WHERE id = ?`, created).Exec(ctx)
	require.NoError(t, err)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.student_profiles WHERE id = ?0)
		+ (SELECT count(*) FROM users.student_school_memberships WHERE student_profile_id = ?0)
		+ (SELECT count(*) FROM users.students_legacy WHERE id = ?0)`, created).Scan(ctx, &remaining))
	require.Zero(t, remaining, "a delete through the view must remove the owner rows and the archive row")
}

func TestStudentOwnerCutoverHidesRetiredMemberships(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	_, err := db.NewRaw(`UPDATE users.student_school_memberships SET deleted_at = now() WHERE tenant_id = ? AND student_profile_id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	var visible int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.students WHERE id = ?`, ids[0]).Scan(t.Context(), &visible))
	require.Zero(t, visible, "a retired enrollment must read as a row that is gone, not as an enrolled child")

	// Writing a row the previous image can no longer see must leave every
	// owner untouched, not rewrite the profile half of it.
	before := studentProfileJSON(t, db, ids[0])
	_, err = db.NewRaw(`UPDATE users.students SET extra_info = 'ghost' WHERE id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)
	require.JSONEq(t, before, studentProfileJSON(t, db, ids[0]))
}

func studentProfileJSON(t *testing.T, db *testpkg.DB, id int64) string {
	t.Helper()
	var row string
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(p)::text FROM users.student_profiles p WHERE p.id = ?`, id).
		Scan(t.Context(), &row))
	return row
}

// users.expired_privacy_consents is the one database view over users.students.
// It would have followed the rename onto the archive and stopped seeing every
// child enrolled after the switch, so the cutover redefines it over the owner
// storage — with the same columns, for the same rows.
func TestStudentOwnerCutoverKeepsTheExpiredConsentView(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	ctx := t.Context()
	expireConsent := func(studentID int64) {
		t.Helper()
		// created_at is backdated too: the table only accepts an expiry that
		// lies after the row was written.
		_, err := db.NewRaw(`INSERT INTO users.privacy_consents
			(tenant_id, student_id, policy_version, accepted, accepted_at, created_at, expires_at, renewal_required, data_retention_days)
			VALUES (?, ?, 'v1', true, now() - interval '400 days', now() - interval '400 days',
				now() - interval '1 day', true, 30)`,
			tenantID, studentID).Exec(ctx)
		require.NoError(t, err)
	}
	expireConsent(ids[0])
	before := expiredConsentStudentIDs(t, db, tenantID)
	require.Equal(t, []int64{ids[0]}, before)

	require.NoError(t, finalizeStudentOwnerStorage(ctx, db, installStudentOwnerCompatibility))
	require.Equal(t, before, expiredConsentStudentIDs(t, db, tenantID),
		"the redefined view must report the same children")

	// A child enrolled after the switch has no archive row, so its legacy
	// guardian columns read NULL — but the child itself must still be found.
	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Consent", "Aftercut")
	var created int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?) RETURNING id`,
		tenantID, person.ID).Scan(ctx, &created))
	var membershipID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		VALUES (?, ?, '1a') RETURNING id`, tenantID, created).Scan(ctx, &membershipID))
	_, err := db.NewRaw(`INSERT INTO users.student_care_profiles (membership_id, tenant_id) VALUES (?, ?)`,
		membershipID, tenantID).Exec(ctx)
	require.NoError(t, err)
	expireConsent(created)
	require.Equal(t, []int64{ids[0], created}, expiredConsentStudentIDs(t, db, tenantID))

	var guardianName *string
	require.NoError(t, db.NewRaw(`SELECT guardian_name FROM users.expired_privacy_consents WHERE student_id = ?`, created).
		Scan(ctx, &guardianName))
	require.Nil(t, guardianName, "a child without an archive row carries no legacy contact")
}

func expiredConsentStudentIDs(t *testing.T, db *testpkg.DB, tenantID int64) []int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM users.expired_privacy_consents
		WHERE tenant_id = ? ORDER BY student_id`, tenantID).Scan(t.Context(), &ids))
	return ids
}

func TestStudentOwnerCutoverRepointsAndValidatesForeignKeys(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, _ := studentOwnerCutoverFixture(t, db, 2)
	var before int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.students'::regclass AND contype = 'f'`).Scan(t.Context(), &before))
	require.Positive(t, before)

	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	var archived, unvalidated int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.students_legacy'::regclass AND contype = 'f'`).Scan(t.Context(), &archived))
	require.Zero(t, archived, "no dependent table may keep pointing at the rollback archive")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.student_profiles'::regclass AND contype = 'f' AND NOT convalidated`).
		Scan(t.Context(), &unvalidated))
	require.Equal(t, before, unvalidated, "the switch itself must not scan the dependent tables")

	require.NoError(t, ValidateStudentOwnerForeignKeys(t.Context(), db))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.student_profiles'::regclass AND contype = 'f' AND NOT convalidated`).
		Scan(t.Context(), &unvalidated))
	require.Zero(t, unvalidated)
	require.NoError(t, ValidateStudentOwnerForeignKeys(t.Context(), db), "validation must be resumable")

	// A child enrolled after the switch exists only in the owner tables; its
	// dependent rows have to be accepted all the same.
	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Owner", "Only")
	var profileID, membershipID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?) RETURNING id`,
		tenantID, person.ID).Scan(t.Context(), &profileID))
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		VALUES (?, ?, '1a') RETURNING id`, tenantID, profileID).Scan(t.Context(), &membershipID))
	_, err := db.NewRaw(`INSERT INTO users.student_care_profiles (membership_id, tenant_id) VALUES (?, ?)`,
		membershipID, tenantID).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at)
		VALUES (?, ?, current_date, 'sick', now())`, tenantID, profileID).Exec(t.Context())
	require.NoError(t, err, "a dependent row of a child created after the switch must be accepted")
}

func TestStudentOwnerCompatibilityIsTenantIsolated(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, _ := studentOwnerCutoverFixture(t, db, 2)
	otherTenant := studentOwnerSecondTenant(t, db)
	studentOwnerFixture(t, db, otherTenant, 2)
	_, err := RunStudentOwnerBackfill(t.Context(), db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))

	for _, scoped := range []int64{tenantID, otherTenant} {
		require.NoError(t, withPhoenixTenantTx(t, db, scoped, func(ctx context.Context, tx testpkg.Tx) error {
			var visible, foreign int64
			if err := tx.NewRaw(`SELECT count(*), count(*) FILTER (WHERE tenant_id <> ?) FROM users.students`, scoped).
				Scan(ctx, &visible, &foreign); err != nil {
				return err
			}
			require.EqualValues(t, 2, visible, "tenant %d must see exactly its own children", scoped)
			require.Zero(t, foreign)
			return nil
		}))
	}
	require.Error(t, withPhoenixTenantTx(t, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		person := testpkg.CreateTestPersonForTenant(t, db, otherTenant, "Foreign", "Child")
		_, err := tx.NewRaw(`INSERT INTO users.students (tenant_id, person_id, school_class) VALUES (?, ?, '2a')`,
			otherTenant, person.ID).Exec(ctx)
		return err
	}), "the view must not let one school write another school's child")
}

func TestStudentOwnerCompatibilityCountsHits(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	reads, writes := studentCompatibilityHits(t, db)

	var scanned int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.students WHERE tenant_id = ?`, tenantID).Scan(t.Context(), &scanned))
	_, err := db.NewRaw(`UPDATE users.students SET extra_info = 'observed' WHERE id = ?`, ids[0]).Exec(t.Context())
	require.NoError(t, err)

	readsAfter, writesAfter := studentCompatibilityHits(t, db)
	require.Greater(t, readsAfter, reads, "a previous-image read must be observable")
	require.Greater(t, writesAfter, writes, "a previous-image write must be observable")

	// Owner storage is not the compatibility shape: reading it moves nothing.
	reads, writes = readsAfter, writesAfter
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`, tenantID).Scan(t.Context(), &scanned))
	readsAfter, writesAfter = studentCompatibilityHits(t, db)
	require.Equal(t, reads, readsAfter)
	require.Equal(t, writes, writesAfter)
}

func studentCompatibilityHits(t *testing.T, db *testpkg.DB) (reads, writes int64) {
	t.Helper()
	// last_value reports 1 for a sequence nextval has never touched, so the
	// counters are read through pg_sequence_last_value, which reports NULL.
	require.NoError(t, db.NewRaw(`SELECT
		coalesce(pg_sequence_last_value('users.student_compatibility_reads'), 0),
		coalesce(pg_sequence_last_value('users.student_compatibility_writes'), 0)`).Scan(t.Context(), &reads, &writes))
	return reads, writes
}

func TestStudentOwnerCompatibilityRepairRestoresTheRollbackShape(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 3)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	ctx := t.Context()

	// The switch leaves the repointed constraints NOT VALID; until the
	// migration's post-commit validation runs, the rollback shape is
	// deliberately reported as incomplete.
	report, err := RepairStudentOwnerCompatibility(ctx, db, StudentCompatibilityOptions{TenantIDs: []int64{tenantID}, VerifyOnly: true})
	require.ErrorContains(t, err, "rollback shape is incomplete")
	require.NotEmpty(t, report.UnvalidatedForeignKeys)
	require.NoError(t, ValidateStudentOwnerForeignKeys(ctx, db))

	report, err = RepairStudentOwnerCompatibility(ctx, db, StudentCompatibilityOptions{TenantIDs: []int64{tenantID}, VerifyOnly: true})
	require.NoError(t, err)
	require.True(t, report.Healthy())
	require.Len(t, report.Tenants, 1)
	require.EqualValues(t, 3, report.Tenants[0].Students)
	require.EqualValues(t, 3, report.Tenants[0].Profiles)
	require.Zero(t, report.Tenants[0].Hidden)
	require.Empty(t, report.UnvalidatedForeignKeys)

	// A child the owner storage deleted leaves an archive row behind: the
	// current providers never write the rollback shape.
	_, err = db.NewRaw(`DELETE FROM users.student_profiles WHERE tenant_id = ? AND id = ?`, tenantID, ids[0]).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.student_school_memberships SET deleted_at = now() WHERE tenant_id = ? AND student_profile_id = ?`,
		tenantID, ids[1]).Exec(ctx)
	require.NoError(t, err)

	report, err = RepairStudentOwnerCompatibility(ctx, db, StudentCompatibilityOptions{TenantIDs: []int64{tenantID}, VerifyOnly: true})
	require.ErrorContains(t, err, "rollback shape is incomplete")
	require.EqualValues(t, 1, report.Tenants[0].StaleArchivedRows)
	require.EqualValues(t, 1, report.Tenants[0].Hidden, "a retired enrollment is hidden, not broken")
	require.EqualValues(t, 1, report.Tenants[0].Students)

	before := report.CompatibilityReads
	report, err = RepairStudentOwnerCompatibility(ctx, db, StudentCompatibilityOptions{TenantIDs: []int64{tenantID}})
	require.NoError(t, err)
	require.True(t, report.Healthy())
	require.EqualValues(t, 1, report.Tenants[0].RepairedRows)
	require.Zero(t, report.Tenants[0].StaleArchivedRows)
	require.Equal(t, before, report.CompatibilityReads, "verifying the rollback shape must not count as a previous-image read")

	// The repair is idempotent and leaves the authoritative rows alone.
	var owners int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`, tenantID).Scan(ctx, &owners))
	report, err = RepairStudentOwnerCompatibility(ctx, db, StudentCompatibilityOptions{TenantIDs: []int64{tenantID}})
	require.NoError(t, err)
	require.Zero(t, report.Tenants[0].RepairedRows)
	var ownersAfter int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`, tenantID).Scan(ctx, &ownersAfter))
	require.Equal(t, owners, ownersAfter)
}

func TestStudentOwnerCompatibilityRepairRequiresTheCutover(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	_, err := RepairStudentOwnerCompatibility(t.Context(), db, StudentCompatibilityOptions{})
	require.ErrorContains(t, err, "not the committed compatibility view")
}

func TestStudentOwnerCutoverRefusesToRunTwice(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	studentOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	require.ErrorContains(t,
		finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility),
		"not a base table")
}

func TestStudentOwnerCutoverUpResumesWhenAlreadyAView(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	studentOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	require.NoError(t, studentOwnerCutoverUp(t.Context(), db),
		"an interrupted VALIDATE CONSTRAINT must let the next migrate finish and record the version")
	var unvalidated int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.student_profiles'::regclass AND contype = 'f' AND NOT convalidated`).
		Scan(t.Context(), &unvalidated))
	require.Zero(t, unvalidated)
	require.NoError(t, studentOwnerCutoverUp(t.Context(), db), "a second run must be a no-op once the view is in place")
}

func TestStudentOwnerCutoverTransitionStatusDoesNotRevertAlumnus(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	_, ids := studentOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	id := ids[0]
	ctx := t.Context()

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.students WHERE id = ? FOR UPDATE`, id)
	require.NoError(t, err)

	done := make(chan studentUpdateResult, 1)
	go func() {
		result, execErr := db.NewRaw(`UPDATE users.students SET status = 'inactive' WHERE id = ? AND status = 'active'`, id).Exec(ctx)
		if execErr != nil {
			done <- studentUpdateResult{err: execErr}
			return
		}
		n, nErr := result.RowsAffected()
		done <- studentUpdateResult{affected: n, err: nErr}
	}()
	requireStudentUpdateWaiting(t, db, ctx, "%SET status = 'inactive'%")
	_, err = holder.ExecContext(ctx, `UPDATE users.students SET status = 'alumnus' WHERE id = ?`, id)
	require.NoError(t, err)
	require.NoError(t, holder.Commit())

	got := waitStudentUpdate(t, done)
	require.NoError(t, got.err)
	require.Zero(t, got.affected, "EvalPlanQual of WHERE status = 'active' must skip the row")
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM users.students WHERE id = ?`, id).Scan(ctx, &status))
	require.Equal(t, "alumnus", status)
}

func TestStudentOwnerCutoverUpdateKeepsConcurrentColumns(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	_, ids := studentOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	id := ids[0]
	ctx := t.Context()

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.students WHERE id = ? FOR UPDATE`, id)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, execErr := db.NewRaw(`UPDATE users.students SET extra_info = 'kept' WHERE id = ?`, id).Exec(ctx)
		done <- execErr
	}()
	requireStudentUpdateWaiting(t, db, ctx, "%SET extra_info = 'kept'%")
	_, err = holder.ExecContext(ctx, `UPDATE users.students SET status = 'alumnus' WHERE id = ?`, id)
	require.NoError(t, err)
	require.NoError(t, holder.Commit())
	require.NoError(t, waitStudentUpdateErr(t, done))

	var status, extra string
	require.NoError(t, db.NewRaw(`SELECT status, extra_info FROM users.students WHERE id = ?`, id).Scan(ctx, &status, &extra))
	require.Equal(t, "alumnus", status)
	require.Equal(t, "kept", extra)
}

func TestStudentOwnerCutoverUpdateKeepsConcurrentArchiveColumns(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	_, ids := studentOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	id := ids[0]
	ctx := t.Context()

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.students WHERE id = ? FOR UPDATE`, id)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, execErr := db.NewRaw(`UPDATE users.students SET extra_info = 'kept' WHERE id = ?`, id).Exec(ctx)
		done <- execErr
	}()
	requireStudentUpdateWaiting(t, db, ctx, "%SET extra_info = 'kept'%")
	_, err = holder.ExecContext(ctx,
		`UPDATE users.students SET sick = true, guardian_email = 'kept@example.org' WHERE id = ?`, id)
	require.NoError(t, err)
	require.NoError(t, holder.Commit())
	require.NoError(t, waitStudentUpdateErr(t, done))

	var extra, email string
	var sick bool
	require.NoError(t, db.NewRaw(
		`SELECT extra_info, sick, guardian_email FROM users.students WHERE id = ?`, id,
	).Scan(ctx, &extra, &sick, &email))
	require.Equal(t, "kept", extra)
	require.True(t, sick, "a later extra_info write must not restore the snapshot sick flag")
	require.Equal(t, "kept@example.org", email)
}

func TestStudentOwnerCutoverSelectForUpdateAndUpdateShareLockOrder(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	_, ids := studentOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	id := ids[0]
	ctx := t.Context()

	readTx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = readTx.Rollback() }()
	updateTx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = updateTx.Rollback() }()

	_, err = readTx.ExecContext(ctx, `SELECT id FROM users.student_profiles WHERE id = ? FOR UPDATE`, id)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, execErr := updateTx.ExecContext(ctx, `UPDATE users.students SET extra_info = 'ordered' WHERE id = ?`, id)
		done <- execErr
	}()
	requireStudentUpdateWaiting(t, db, ctx, "%SET extra_info = 'ordered'%")
	_, err = readTx.ExecContext(ctx, `SELECT s.id FROM users.students s WHERE s.id = ? FOR UPDATE`, id)
	require.NoError(t, err, "SELECT FOR UPDATE must take the same lock order as a routed UPDATE")
	require.NoError(t, readTx.Commit())
	require.NoError(t, waitStudentUpdateErr(t, done))
	require.NoError(t, updateTx.Commit())
}

type studentUpdateResult struct {
	affected int64
	err      error
}

func requireStudentUpdateWaiting(t *testing.T, db *testpkg.DB, ctx context.Context, queryLike string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE ?)`,
			queryLike).Scan(ctx, &blocked)
		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
}

func waitStudentUpdate(t *testing.T, done <-chan studentUpdateResult) studentUpdateResult {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(10 * time.Second):
		t.Fatal("update did not finish after the holder committed")
		return studentUpdateResult{}
	}
}

func waitStudentUpdateErr(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("update did not finish after the holder committed")
		return nil
	}
}
