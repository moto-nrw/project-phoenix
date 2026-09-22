package migrations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The Expand and Backfill contracts describe a world in which users.staff is
// still the authoritative base table. Restore that world inside the disposable
// clone so those tests keep testing what they were written for; production
// rollback deliberately retains the compatibility shape instead.
func setupStaffStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.RestoreStaffStorageBeforeCutover(t, db)
	return db
}

func setupIsolatedStaffStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestoreStaffStorageBeforeCutover(t, db)
	return db
}

// staffOwnerCutoverFixture returns a backfilled tenant that is ready to be
// switched.
func staffOwnerCutoverFixture(t *testing.T, db *testpkg.DB, count int) (int64, []int64) {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	ids := staffOwnerFixture(t, db, tenantID, count)
	_, err := RunStaffOwnerBackfill(t.Context(), db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	return tenantID, ids
}

func staffRowsJSON(t *testing.T, db bun.IDB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(
		`SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY s.id)::text, '[]') FROM users.staff s WHERE s.tenant_id = ?`,
		tenantID).Scan(t.Context(), &rows))
	return rows
}

func staffOwnerCheckpointRow(t *testing.T, db *testpkg.DB, tenantID int64) StaffOwnerBackfillCheckpoint {
	t.Helper()
	cp := new(StaffOwnerBackfillCheckpoint)
	require.NoError(t, db.NewSelect().Model(cp).
		Where("backfill = ? AND tenant_id = ?", StaffOwnerBackfillName, tenantID).Scan(t.Context()))
	return *cp
}

func staffCompatibilityHits(t *testing.T, db bun.IDB) (reads, writes int64) {
	t.Helper()
	// last_value reports 1 for a sequence nextval has never touched, so the
	// counters are read through pg_sequence_last_value, which reports NULL.
	require.NoError(t, db.NewRaw(`SELECT
		coalesce(pg_sequence_last_value('users.staff_compatibility_reads'), 0),
		coalesce(pg_sequence_last_value('users.staff_compatibility_writes'), 0)`).Scan(t.Context(), &reads, &writes))
	return reads, writes
}

func TestStaffOwnerCutoverPreservesContractGolden(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, _ := staffOwnerCutoverFixture(t, db, 5)
	before := staffRowsJSON(t, db, tenantID)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	require.JSONEq(t, before, staffRowsJSON(t, db, tenantID),
		"the previous image must read the same columns, NULLs, soft deletions and JSON shapes through the compatibility view")

	var relkind string
	require.NoError(t, db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.staff'::regclass`).Scan(t.Context(), &relkind))
	require.Equal(t, "v", relkind)
	_, err := RunStaffOwnerBackfill(t.Context(), db, StaffOwnerBackfillOptions{})
	require.ErrorContains(t, err, "not a base table")
	require.ErrorContains(t, ResetStaffOwnerBackfill(t.Context(), db), "not a base table")

	cp := staffOwnerCheckpointRow(t, db, tenantID)
	require.True(t, cp.Stable)
	require.True(t, cp.Verified(), "the switch leaves its own verdict in the checkpoint")
}

func TestStaffOwnerCutoverRequiresCompletedBackfill(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	staffOwnerFixture(t, db, testpkg.Tenant(t), 2)
	switched := false
	err := finalizeStaffOwnerStorage(t.Context(), db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.ErrorContains(t, err, "requires a completed backfill pass")
	require.False(t, switched)
}

func TestStaffOwnerCutoverAppliesTheFinalDelta(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, ids := staffOwnerCutoverFixture(t, db, 3)
	ctx := t.Context()
	// Changes the backfill has not seen: an edit, a physical delete and a
	// newcomer. A target-only edit is overwritten from the source.
	_, err := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'last minute' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.staff WHERE id = ?`, ids[1])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET employment_type = 'minijob' WHERE membership_id = ?`, ids[2])
	require.NoError(t, err)
	newcomer := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Late", "Joiner")
	before := staffRowsJSON(t, db, tenantID)

	require.NoError(t, finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility))
	require.JSONEq(t, before, staffRowsJSON(t, db, tenantID))
	var notes string
	require.NoError(t, db.NewRaw(`SELECT staff_notes FROM users.staff_employment_profiles WHERE membership_id = ?`, ids[0]).Scan(ctx, &notes))
	require.Equal(t, "last minute", notes)
	var gone, joined bool
	require.NoError(t, db.NewRaw(`SELECT NOT EXISTS (SELECT 1 FROM users.staff_school_memberships WHERE id = ?),
		EXISTS (SELECT 1 FROM users.staff_school_memberships WHERE id = ?)`, ids[1], newcomer.ID).Scan(ctx, &gone, &joined))
	require.True(t, gone, "a membership whose source row is gone must not survive the switch")
	require.True(t, joined)

	// Identity allocation continues above every number users.staff issued.
	var next, lastStaff int64
	require.NoError(t, db.NewRaw(`SELECT last_value FROM users.staff_id_seq`).Scan(ctx, &lastStaff))
	require.NoError(t, db.NewRaw(`SELECT nextval('users.staff_school_memberships_id_seq')`).Scan(ctx, &next))
	require.Greater(t, next, lastStaff)
}

func TestStaffOwnerCutoverFinalDeltaRollsBackWithTheSwitch(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, ids := staffOwnerCutoverFixture(t, db, 3)
	ctx := t.Context()
	_, err := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'last minute' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	before := staffOwnerCheckpointRow(t, db, tenantID)

	injected := errors.New("switch failed after the final delta")
	err = finalizeStaffOwnerStorage(ctx, db, func(ctx context.Context, tx bun.Tx) error {
		verification, verifyErr := verifyStaffOwnerTenant(ctx, tx, tenantID)
		require.NoError(t, verifyErr)
		require.True(t, verification.Equal(), verification.Describe())
		return injected
	})
	require.ErrorIs(t, err, injected)
	require.Equal(t, before, staffOwnerCheckpointRow(t, db, tenantID), "a failed switch must roll back its checkpoint evidence")
	var notes string
	require.NoError(t, db.NewRaw(`SELECT coalesce(staff_notes, '') FROM users.staff_employment_profiles WHERE membership_id = ?`, ids[0]).
		Scan(ctx, &notes))
	require.NotEqual(t, "last minute", notes, "the final delta must roll back together with the switch")

	// A failed switch leaves nothing half-installed; the retry succeeds.
	failAfterRename := errors.New("switch failed half-way")
	err = finalizeStaffOwnerStorage(ctx, db, func(ctx context.Context, tx bun.Tx) error {
		if err := repointStaffForeignKeys(ctx, tx); err != nil {
			return err
		}
		return failAfterRename
	})
	require.ErrorIs(t, err, failAfterRename)
	var relkind string
	require.NoError(t, db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.staff'::regclass`).Scan(ctx, &relkind))
	require.Equal(t, "r", relkind)
	require.NoError(t, finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility))
	require.NoError(t, db.NewRaw(`SELECT staff_notes FROM users.staff WHERE id = ?`, ids[0]).Scan(ctx, &notes))
	require.Equal(t, "last minute", notes)
}

func TestStaffOwnerCutoverRefusesAForeignWorkTimeModel(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, ids := staffOwnerCutoverFixture(t, db, 2)
	ctx := t.Context()
	other := staffOwnerSecondTenant(t, db)
	var foreignModel int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date)
		VALUES (?, 'Foreign', '2026-09-01') RETURNING id`, other).Scan(ctx, &foreignModel))
	_, err := db.ExecContext(ctx, `UPDATE users.staff SET work_time_model_id = ? WHERE id = ?`, foreignModel, ids[0])
	require.NoError(t, err)

	require.ErrorContains(t, staffOwnerCutoverPrecondition(ctx, db), fmt.Sprintf("staff rows [%d]", ids[0]),
		"the preflight must name the row before the deployment stops the application")
	switched := false
	err = finalizeStaffOwnerStorage(ctx, db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.ErrorContains(t, err, fmt.Sprintf("tenant %d is not reproducible (1 rows reference another school's work-time model)", tenantID))
	require.False(t, switched)

	_, err = db.ExecContext(ctx, `UPDATE users.staff SET work_time_model_id = NULL WHERE id = ?`, ids[0])
	require.NoError(t, err)
	require.NoError(t, staffOwnerCutoverPrecondition(ctx, db))
	require.NoError(t, finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility))
	require.NoError(t, staffOwnerCutoverPrecondition(ctx, db), "the preflight has nothing to check once the view is in place")
}

func TestStaffOwnerCompatibilityRoutesPreviousImageWrites(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, ids := staffOwnerCutoverFixture(t, db, 2)
	ctx := t.Context()
	require.NoError(t, finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility))

	// INSERT ... RETURNING through the view, the shape the previous image uses.
	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Previous", "Image")
	var id int64
	var optOut bool
	require.NoError(t, db.NewRaw(`INSERT INTO users.staff (tenant_id, person_id, employment_type, personnel_number)
		VALUES (?, ?, 'part_time', 'PI-1') RETURNING id, birthday_display_opt_out`, tenantID, person.ID).Scan(ctx, &id, &optOut))
	require.False(t, optOut)
	var membershipPerson int64
	var employment string
	require.NoError(t, db.NewRaw(`SELECT m.person_id, p.employment_type FROM users.staff_school_memberships m
		JOIN users.staff_employment_profiles p ON p.membership_id = m.id WHERE m.id = ?`, id).Scan(ctx, &membershipPerson, &employment))
	require.Equal(t, person.ID, membershipPerson)
	require.Equal(t, "part_time", employment)

	// UPDATE reaches the owner of every column and bumps updated_at as the
	// old table's trigger did.
	var before, after string
	require.NoError(t, db.NewRaw(`SELECT updated_at::text FROM users.staff WHERE id = ?`, id).Scan(ctx, &before))
	result, err := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'routed', rotation_anchor_date = '2026-03-02'
		WHERE id = ? AND deleted_at IS NULL`, id)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	var notes, anchor string
	require.NoError(t, db.NewRaw(`SELECT staff_notes, rotation_anchor_date::text FROM users.staff_employment_profiles WHERE membership_id = ?`, id).
		Scan(ctx, &notes, &anchor))
	require.Equal(t, "routed", notes)
	require.Equal(t, "2026-03-02", anchor)
	require.NoError(t, db.NewRaw(`SELECT updated_at::text FROM users.staff_school_memberships WHERE id = ?`, id).Scan(ctx, &after))
	require.NotEqual(t, before, after)

	// Identity is immutable, soft deletion lands on the membership, and a
	// physical delete removes both owners.
	_, err = db.ExecContext(ctx, `UPDATE users.staff SET id = id + 100000 WHERE id = ?`, id)
	require.ErrorContains(t, err, "staff identity is immutable")
	_, err = db.ExecContext(ctx, `UPDATE users.staff SET deleted_at = now() WHERE id = ? AND deleted_at IS NULL`, ids[0])
	require.NoError(t, err)
	var retired bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, ids[0]).Scan(ctx, &retired))
	require.True(t, retired)
	_, err = db.ExecContext(ctx, `DELETE FROM users.staff WHERE id = ?`, id)
	require.NoError(t, err)
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.staff_school_memberships WHERE id = ?0)
		+ (SELECT count(*) FROM users.staff_employment_profiles WHERE membership_id = ?0)`, id).Scan(ctx, &remaining))
	require.Zero(t, remaining)

	// The previous image locks through the view.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var locked int64
	require.NoError(t, tx.NewRaw(`SELECT id FROM users.staff WHERE id = ? FOR UPDATE`, ids[0]).Scan(ctx, &locked))
	require.Equal(t, ids[0], locked)
}

func TestStaffOwnerCompatibilityClassifiesConflictsByTheOldNames(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, ids := staffOwnerCutoverFixture(t, db, 2)
	ctx := t.Context()
	require.NoError(t, finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility))
	var taken string
	require.NoError(t, db.NewRaw(`SELECT personnel_number FROM users.staff WHERE id = ?`, ids[0]).Scan(ctx, &taken))

	constraintOf := func(err error) string {
		t.Helper()
		var pgErr pgdriver.Error
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "23505", pgErr.Field('C'))
		return pgErr.Field('n')
	}
	// A second live staff member for the same person.
	var person int64
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.staff WHERE id = ?`, ids[0]).Scan(ctx, &person))
	_, err := db.ExecContext(ctx, `INSERT INTO users.staff (tenant_id, person_id) VALUES (?, ?)`, tenantID, person)
	require.Equal(t, "idx_staff_tenant_person", constraintOf(err))

	// A taken personnel number, through the view and on the owner table.
	newcomer := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Number", "Clash")
	_, err = db.ExecContext(ctx, `INSERT INTO users.staff (tenant_id, person_id, personnel_number) VALUES (?, ?, ?)`,
		tenantID, newcomer.ID, taken)
	require.Equal(t, "uq_staff_tenant_personnel_number", constraintOf(err))
	var freeID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.staff (tenant_id, person_id) VALUES (?, ?) RETURNING id`, tenantID, newcomer.ID).Scan(ctx, &freeID))
	_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET personnel_number = ? WHERE membership_id = ?`, taken, freeID)
	require.Equal(t, "uq_staff_tenant_personnel_number", constraintOf(err))

	// Once the holder leaves, the number is free again, as with the old
	// partial index.
	_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = now() WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET personnel_number = ? WHERE membership_id = ?`, taken, freeID)
	require.NoError(t, err)
}

func TestStaffOwnerCutoverRepointsAndValidatesForeignKeys(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, _ := staffOwnerCutoverFixture(t, db, 2)
	ctx := t.Context()
	var before int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.staff'::regclass AND contype = 'f'`).Scan(ctx, &before))
	require.Positive(t, before)

	require.NoError(t, finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility))
	var archived, unvalidated int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.staff_legacy'::regclass AND contype = 'f'`).Scan(ctx, &archived))
	require.Zero(t, archived, "no dependent table may keep pointing at the rollback archive")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.staff_school_memberships'::regclass AND contype = 'f' AND NOT convalidated`).Scan(ctx, &unvalidated))
	require.Equal(t, before, unvalidated, "the switch itself must not scan the dependent tables")

	require.NoError(t, ValidateStaffOwnerForeignKeys(ctx, db))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.staff_school_memberships'::regclass AND contype = 'f' AND NOT convalidated`).Scan(ctx, &unvalidated))
	require.Zero(t, unvalidated)
	require.NoError(t, ValidateStaffOwnerForeignKeys(ctx, db), "validation must be resumable")

	// A staff member who joins after the switch exists only in the owner
	// tables; their dependent rows have to be accepted all the same, and a
	// work-time model the frozen archive once named can still be deleted.
	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Owner", "Only")
	var membershipID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?) RETURNING id`,
		tenantID, person.ID).Scan(ctx, &membershipID))
	_, err := db.ExecContext(ctx, `INSERT INTO users.staff_employment_profiles (membership_id, tenant_id) VALUES (?, ?)`, membershipID, tenantID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users.teachers (tenant_id, staff_id) VALUES (?, ?)`, tenantID, membershipID)
	require.NoError(t, err, "a dependent row of a staff member created after the switch must be accepted")
	_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET work_time_model_id = NULL WHERE tenant_id = ?`, tenantID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM config.work_time_models WHERE tenant_id = ?`, tenantID)
	require.NoError(t, err, "the archive must not keep a detached work-time model alive")
}

func TestStaffOwnerCutoverMovesTheAuditLogActorOntoTheMembership(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	staffOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_viewdef('audit.time_tracking_audit_log'::regclass)`).Scan(t.Context(), &definition))
	require.Contains(t, definition, "staff_school_memberships st")
	require.NotContains(t, definition, "staff_legacy")
}

func TestStaffOwnerCompatibilityIsTenantIsolated(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID := testpkg.Tenant(t)
	otherTenant := staffOwnerSecondTenant(t, db)
	staffOwnerFixture(t, db, tenantID, 2)
	staffOwnerFixture(t, db, otherTenant, 3)
	_, err := RunStaffOwnerBackfill(t.Context(), db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))

	for scoped, want := range map[int64]int64{tenantID: 2, otherTenant: 3} {
		require.NoError(t, withPhoenixTenantTx(t, db, scoped, func(ctx context.Context, tx testpkg.Tx) error {
			for _, relation := range []string{"users.staff", "users.staff_school_memberships", "users.staff_employment_profiles"} {
				var visible, foreign int64
				if err := tx.NewRaw(fmt.Sprintf(`SELECT count(*), count(*) FILTER (WHERE tenant_id <> ?) FROM %s`, relation), scoped).
					Scan(ctx, &visible, &foreign); err != nil {
					return err
				}
				require.Equal(t, want, visible, "tenant %d must see exactly its own staff in %s", scoped, relation)
				require.Zero(t, foreign)
			}
			return nil
		}))
	}
	person := testpkg.CreateTestPersonForTenant(t, db, otherTenant, "Foreign", "Staff")
	require.Error(t, withPhoenixTenantTx(t, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.NewRaw(`INSERT INTO users.staff (tenant_id, person_id) VALUES (?, ?)`, otherTenant, person.ID).Exec(ctx)
		return err
	}), "the view must not let one school write another school's staff member")
	require.Error(t, withPhoenixTenantTx(t, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.NewRaw(`INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?)`, otherTenant, person.ID).Exec(ctx)
		return err
	}), "the membership must not let one school write another school's staff member")
}

func TestStaffOwnerCompatibilityCountsHits(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	tenantID, ids := staffOwnerCutoverFixture(t, db, 2)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	reads, writes := staffCompatibilityHits(t, db)

	var scanned int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff WHERE tenant_id = ?`, tenantID).Scan(t.Context(), &scanned))
	_, err := db.ExecContext(t.Context(), `UPDATE users.staff SET staff_notes = 'observed' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	readsAfter, writesAfter := staffCompatibilityHits(t, db)
	require.Greater(t, readsAfter, reads, "a previous-image read must be observable")
	require.Greater(t, writesAfter, writes, "a previous-image write must be observable")

	// Owner storage is not the compatibility shape: using it moves nothing.
	reads, writes = readsAfter, writesAfter
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships m
		JOIN users.staff_employment_profiles p ON p.membership_id = m.id WHERE m.tenant_id = ?`, tenantID).Scan(t.Context(), &scanned))
	_, err = db.ExecContext(t.Context(), `UPDATE users.staff_employment_profiles SET staff_notes = 'owner' WHERE membership_id = ?`, ids[0])
	require.NoError(t, err)
	readsAfter, writesAfter = staffCompatibilityHits(t, db)
	require.Equal(t, reads, readsAfter)
	require.Equal(t, writes, writesAfter)
}

func TestStaffOwnerCutoverRefusesToRunTwice(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	staffOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	require.ErrorContains(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility), "not a base table")
}

func TestStaffOwnerCutoverUpResumesWhenAlreadyAView(t *testing.T) {
	t.Parallel()
	db := setupStaffStorageBeforeCutover(t)
	staffOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	require.NoError(t, staffOwnerCutoverUp(t.Context(), db),
		"an interrupted VALIDATE CONSTRAINT must let the next migrate finish and record the version")
	var unvalidated int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.staff_school_memberships'::regclass AND contype = 'f' AND NOT convalidated`).Scan(t.Context(), &unvalidated))
	require.Zero(t, unvalidated)
	require.NoError(t, staffOwnerCutoverUp(t.Context(), db), "a second run must be a no-op once the view is in place")
	require.ErrorContains(t, staffOwnerCutoverDown(t.Context(), db), "previous application image")
}

func TestStaffOwnerCompatibilityUpdateKeepsConcurrentOwnerWrites(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStaffStorageBeforeCutover(t)
	_, ids := staffOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	id := ids[0]
	ctx := t.Context()

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.staff WHERE id = ? FOR UPDATE`, id)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, execErr := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'kept' WHERE id = ?`, id)
		done <- execErr
	}()
	requireStudentUpdateWaiting(t, db, ctx, "%SET staff_notes = 'kept'%")
	_, err = holder.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET employment_type = 'minijob' WHERE membership_id = ?`, id)
	require.NoError(t, err)
	require.NoError(t, holder.Commit())
	require.NoError(t, waitStudentUpdateErr(t, done))

	var employment, notes string
	require.NoError(t, db.NewRaw(`SELECT employment_type, staff_notes FROM users.staff WHERE id = ?`, id).Scan(ctx, &employment, &notes))
	require.Equal(t, "minijob", employment, "the routed update must not restore its unlocked snapshot")
	require.Equal(t, "kept", notes)
}

func TestStaffOwnerCompatibilityUpdateSkipsARowRetiredUnderIt(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStaffStorageBeforeCutover(t)
	_, ids := staffOwnerCutoverFixture(t, db, 1)
	require.NoError(t, finalizeStaffOwnerStorage(t.Context(), db, installStaffOwnerCompatibility))
	id := ids[0]
	ctx := t.Context()

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, id)
	require.NoError(t, err)

	done := make(chan error, 1)
	affected := make(chan int64, 1)
	go func() {
		result, execErr := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'late' WHERE id = ? AND deleted_at IS NULL`, id)
		if execErr == nil {
			rows, _ := result.RowsAffected()
			affected <- rows
		}
		done <- execErr
	}()
	requireStudentUpdateWaiting(t, db, ctx, "%SET staff_notes = 'late'%")
	_, err = holder.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = now() WHERE id = ?`, id)
	require.NoError(t, err)
	require.NoError(t, holder.Commit())
	require.NoError(t, waitStudentUpdateErr(t, done))
	require.Zero(t, <-affected, "like the heap recheck of deleted_at IS NULL, a retired row is no row updated")
	var notes string
	require.NoError(t, db.NewRaw(`SELECT coalesce(staff_notes, '') FROM users.staff_employment_profiles WHERE membership_id = ?`, id).Scan(ctx, &notes))
	require.NotEqual(t, "late", notes)
}
