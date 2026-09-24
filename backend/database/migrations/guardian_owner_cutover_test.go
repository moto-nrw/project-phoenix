package migrations

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// The Expand and Backfill contracts describe a world in which
// users.students_guardians is still authoritative. Restore that world inside
// the disposable clone so those tests keep testing what they were written
// for; production rollback deliberately retains the compatibility shape
// instead.
func setupGuardianStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestoreGuardianStorageBeforeCutover(t, db)
	return db
}

// guardianCutoverFixture returns a backfilled school that is ready to be
// switched, with the link IDs in insertion order.
func guardianCutoverFixture(t *testing.T, db *testpkg.DB, count int) (int64, []int64) {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, count)
	report, err := RunGuardianOwnerBackfill(t.Context(), db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	return tenantID, ids
}

// guardianOldShapeJSON is what the previous image reads: every column of every
// users.students_guardians row of the school except the two timestamps, which
// the owners maintain on their own after the switch.
func guardianOldShapeJSON(t *testing.T, db bun.IDB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(jsonb_build_array(sg.id, sg.tenant_id, sg.student_id,
			sg.guardian_profile_id, sg.relationship_type, sg.guardian_role, sg.is_primary, sg.is_emergency_contact,
			sg.can_pickup, sg.pickup_notes, sg.emergency_priority, sg.permissions, sg.is_payer) ORDER BY sg.id), '[]'::jsonb)::text
		FROM users.students_guardians AS sg WHERE sg.tenant_id = ?`, tenantID).Scan(t.Context(), &rows))
	return rows
}

// guardianOwnerShapeJSON is the same projection read from the three owners.
func guardianOwnerShapeJSON(t *testing.T, db bun.IDB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(jsonb_build_array(r.id, r.tenant_id, r.student_id,
			r.guardian_profile_id, r.relationship_type, r.guardian_role, r.is_primary, r.is_emergency_contact,
			p.can_pickup, p.pickup_notes, r.emergency_priority, a.permissions, r.is_payer) ORDER BY r.id), '[]'::jsonb)::text
		FROM users.student_guardian_relationships AS r
		JOIN users.student_guardian_pickup_permissions AS p ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
		JOIN auth.guardian_student_access AS a ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id
		WHERE r.tenant_id = ?`, tenantID).Scan(t.Context(), &rows))
	return rows
}

func guardianCompatibilityWrites(t *testing.T, db bun.IDB) int64 {
	t.Helper()
	var writes int64
	// last_value reports 1 for a sequence nextval has never touched, so the
	// counter is read through pg_sequence_last_value, which reports NULL.
	require.NoError(t, db.NewRaw(`SELECT coalesce(pg_sequence_last_value('users.students_guardians_compatibility_writes'), 0)`).Scan(t.Context(), &writes))
	return writes
}

// requireGuardianMirrorEqual holds the mirror against the owners, and every
// access row against the guardian's current account.
func requireGuardianMirrorEqual(t *testing.T, db bun.IDB, tenantID int64) {
	t.Helper()
	require.JSONEq(t, guardianOwnerShapeJSON(t, db, tenantID), guardianOldShapeJSON(t, db, tenantID),
		"the rollback mirror must equal the joined owners")
	var drift int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.guardian_student_access a
		JOIN users.student_guardian_relationships r ON r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		JOIN users.guardian_profiles g ON g.tenant_id = r.tenant_id AND g.id = r.guardian_profile_id
		WHERE a.tenant_id = ? AND a.account_id IS DISTINCT FROM g.account_id`, tenantID).Scan(t.Context(), &drift))
	require.Zero(t, drift, "every access row binds the guardian's current account")
}

func TestGuardianOwnerCutoverPreservesContractGolden(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, _ := guardianCutoverFixture(t, db, 6)
	before := guardianOldShapeJSON(t, db, tenantID)
	require.NoError(t, finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility))
	require.JSONEq(t, before, guardianOldShapeJSON(t, db, tenantID),
		"the previous image must read the same rows after the switch")
	require.JSONEq(t, before, guardianOwnerShapeJSON(t, db, tenantID), "the owners serve the same rows")

	installed, err := guardianCompatibilityInstalled(ctx, db)
	require.NoError(t, err)
	require.True(t, installed)
	require.Zero(t, guardianCompatibilityWrites(t, db), "the switch itself routes nothing")

	status, err := GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	cp := guardianOwnerCheckpoint(t, status, tenantID)
	require.True(t, cp.Verified(), "the switch leaves its own verdict in the checkpoint: %+v", cp)
	require.True(t, cp.Stable)

	_, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.ErrorContains(t, err, "already cut over")
	require.ErrorContains(t, ResetGuardianOwnerBackfill(ctx, db), "already cut over")
	require.ErrorContains(t, guardianOwnerBackfillDown(ctx, db), "already cut over")
	require.ErrorContains(t, finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility), "already cut over")
	require.ErrorContains(t, guardianOwnerCutoverDown(ctx, db), "previous application image")
	require.NoError(t, guardianOwnerCutoverUp(ctx, db), "a second migrate only resumes the validation")
	require.NoError(t, guardianOwnerCutoverPrecondition(ctx, db))

	var repointed int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint
		WHERE confrelid = 'users.student_guardian_relationships'::regclass AND contype = 'f' AND convalidated
		  AND conrelid IN ('meta.parent_student_consent_permission_grants'::regclass, 'meta.meal_participation_permission_grants'::regclass)`).Scan(ctx, &repointed))
	require.Equal(t, 2, repointed, "both grant keys name the relationship and are validated")
	var restrict string
	require.NoError(t, db.NewRaw(`SELECT confdeltype::text FROM pg_constraint
		WHERE conrelid = 'users.student_guardian_relationships'::regclass AND conname = 'fk_student_guardian_relationships_guardian'`).Scan(ctx, &restrict))
	require.Equal(t, "r", restrict, "deleting a linked guardian is refused again (#819)")
}

func TestGuardianOwnerCutoverRequiresCompletedBackfill(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	guardianOwnerFixture(t, db, testpkg.Tenant(t), 2)
	switched := false
	err := finalizeGuardianOwnerStorage(t.Context(), db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.ErrorContains(t, err, "requires a completed backfill pass")
	require.False(t, switched)
}

func TestGuardianOwnerCutoverPreflightNamesBlockingRows(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 3)
	require.ErrorContains(t, guardianOwnerCutoverPrecondition(ctx, db), "no completed guardian owner backfill pass")
	_, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	require.NoError(t, guardianOwnerCutoverPrecondition(ctx, db))

	// Re-parenting a primary row leaves a child with two primaries, which the
	// copy can never reproduce.
	var childID int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &childID))
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET student_id = ? WHERE id = ?`, childID, ids[2])
	require.NoError(t, err)
	require.ErrorContains(t, guardianOwnerCutoverPrecondition(ctx, db), "cannot be copied")
	err = finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility)
	require.ErrorContains(t, err, "is not reproducible")
	installed, err := guardianCompatibilityInstalled(ctx, db)
	require.NoError(t, err)
	require.False(t, installed, "a refused switch installs nothing")
}

// A release that carries Backfill and Cutover together is preflighted before
// the Backfill's Up has run the pass the Cutover needs.
func TestGuardianOwnerCutoverPreflightLeavesThePassToAPendingBackfill(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	ids := guardianOwnerFixture(t, db, testpkg.Tenant(t), 3)
	require.ErrorContains(t, guardianOwnerCutoverPrecondition(ctx, db), "no completed guardian owner backfill pass")

	backfillPending := withPendingMigrations(ctx, migrate.MigrationSlice{{Name: "001015413"}})
	require.True(t, migrationPending(backfillPending, guardianOwnerBackfillVersion))
	require.NoError(t, guardianOwnerCutoverPrecondition(backfillPending, db))

	var childID int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &childID))
	_, err := db.ExecContext(ctx, `UPDATE users.students_guardians SET student_id = ? WHERE id = ?`, childID, ids[2])
	require.NoError(t, err)
	require.ErrorContains(t, guardianOwnerCutoverPrecondition(backfillPending, db), "cannot be copied",
		"no later Up can correct a rejected row")
}

func TestGuardianOwnerCutoverAppliesTheFinalDelta(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, ids := guardianCutoverFixture(t, db, 4)
	// Changes the backfill has not seen: a pickup and permission edit, a
	// physical delete, a newcomer, a primary moved to another guardian of the
	// same child, a guardian gaining a portal account and a target-only edit
	// the source overwrites.
	_, err := db.ExecContext(ctx, `UPDATE users.students_guardians SET can_pickup = NOT can_pickup, pickup_notes = 'nur dienstags',
		permissions = permissions || '{"parent_portal.notes.write": true}' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.students_guardians WHERE id = ?`, ids[1])
	require.NoError(t, err)
	var childID, guardianID int64
	require.NoError(t, db.NewRaw(`SELECT student_id, guardian_profile_id FROM users.students_guardians WHERE id = ?`, ids[2]).Scan(ctx, &childID, &guardianID))
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Delta", "Second", "guardian-cutover-delta")
	newcomer := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, childID, second.ID, "legal_guardian")
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET is_primary = true WHERE id = ?`, newcomer.ID)
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "guardian-cutover-delta-account")
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, account.ID, second.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.student_guardian_relationships SET relationship_type = 'other' WHERE id = ?`, ids[3])
	require.NoError(t, err)
	before := guardianOldShapeJSON(t, db, tenantID)

	require.NoError(t, finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility))
	require.JSONEq(t, before, guardianOwnerShapeJSON(t, db, tenantID), "the owners reproduce every source change")
	requireGuardianMirrorEqual(t, db, tenantID)
	var primaryOnly bool
	require.NoError(t, db.NewRaw(`SELECT bool_and(is_primary = (id = ?)) FROM users.student_guardian_relationships
		WHERE tenant_id = ? AND student_id = ?`, newcomer.ID, tenantID, childID).Scan(ctx, &primaryOnly))
	require.True(t, primaryOnly, "the moved primary lands on the new guardian alone")
	var deleted bool
	require.NoError(t, db.NewRaw(`SELECT NOT EXISTS (SELECT 1 FROM users.student_guardian_relationships WHERE id = ?)`, ids[1]).Scan(ctx, &deleted))
	require.True(t, deleted)

	// Identity allocation continues above every id the old table issued.
	var nextID, lastOld int64
	require.NoError(t, db.NewRaw(`SELECT last_value FROM users.students_guardians_id_seq`).Scan(ctx, &lastOld))
	require.NoError(t, db.NewRaw(`SELECT nextval('users.student_guardian_relationships_id_seq')`).Scan(ctx, &nextID))
	require.Greater(t, nextID, lastOld)
}

func TestGuardianOwnerCutoverRollsBackWithAFailingSwitch(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, ids := guardianCutoverFixture(t, db, 3)
	_, err := db.ExecContext(ctx, `UPDATE users.students_guardians SET pickup_notes = 'nach dem Backfill' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	targetsBefore := guardianOwnerShapeJSON(t, db, tenantID)
	failure := errors.New("switch failed after its first statement")
	err = finalizeGuardianOwnerStorage(ctx, db, func(ctx context.Context, tx bun.Tx) error {
		if err := installGuardianOwnerCompatibility(ctx, tx); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	require.JSONEq(t, targetsBefore, guardianOwnerShapeJSON(t, db, tenantID), "the final delta rolls back with the switch")
	installed, err := guardianCompatibilityInstalled(ctx, db)
	require.NoError(t, err)
	require.False(t, installed)
	var relkind string
	require.NoError(t, db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students_guardians'::regclass`).Scan(ctx, &relkind))
	require.Equal(t, "r", relkind)

	// A clean retry switches.
	require.NoError(t, finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility))
	requireGuardianMirrorEqual(t, db, tenantID)
}

// cutOverGuardianFixture switches a backfilled school and returns it with the
// link IDs, ready for the rollback-window contracts.
func cutOverGuardianFixture(t *testing.T, db *testpkg.DB, count int) (int64, []int64) {
	t.Helper()
	tenantID, ids := guardianCutoverFixture(t, db, count)
	require.NoError(t, finalizeGuardianOwnerStorage(t.Context(), db, installGuardianOwnerCompatibility))
	return tenantID, ids
}

func TestGuardianCompatibilityRoutesPreviousImageWrites(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, ids := cutOverGuardianFixture(t, db, 2)
	var childID int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &childID))
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Routing", "Second", "guardian-routing-second")
	account := testpkg.CreateTestAccount(t, db, "guardian-routing-second-account")
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, account.ID, second.ID)
	require.NoError(t, err)
	writesBefore := guardianCompatibilityWrites(t, db)

	// Link, as the previous image writes it, including a re-link that must
	// stay a no-op: ON CONFLICT is why the old name had to stay a base table.
	var linkID int64
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		for range 2 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO users.students_guardians
				(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role, is_primary,
				 is_emergency_contact, can_pickup, pickup_notes, emergency_priority, is_payer, permissions)
				VALUES (?, ?, ?, 'parent', 'legal_guardian', false, true, true, 'Mama', 2, false,
				        '{"parent_portal.access": true, "parent_portal.consent.manage": true}')
				ON CONFLICT (tenant_id, student_id, guardian_profile_id) DO NOTHING`, tenantID, childID, second.ID); err != nil {
				return err
			}
		}
		return tx.NewRaw(`SELECT id FROM users.students_guardians WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?`,
			tenantID, childID, second.ID).Scan(ctx, &linkID)
	}))
	requireGuardianMirrorEqual(t, db, tenantID)
	require.Equal(t, writesBefore+1, guardianCompatibilityWrites(t, db), "the conflicting re-link writes nothing")

	// Promote, as the previous image does: the old single-primary trigger
	// demotes the previous primary first, and both rows are routed.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE users.students_guardians SET is_primary = true WHERE tenant_id = ? AND id = ?`, tenantID, linkID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)
	require.Equal(t, writesBefore+3, guardianCompatibilityWrites(t, db))
	var primary int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND is_primary`, childID).Scan(ctx, &primary))
	require.Equal(t, linkID, primary)
	var boundAccount *int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.guardian_student_access WHERE relationship_id = ?`, linkID).Scan(ctx, &boundAccount))
	require.NotNil(t, boundAccount)
	require.Equal(t, account.ID, *boundAccount, "a routed link binds the guardian's account")

	// A permission withdrawal invalidates the consent grant, now on the
	// Identity row.
	_, err = db.ExecContext(ctx, `INSERT INTO meta.parent_student_consent_permission_grants (student_guardian_id) VALUES (?)`, linkID)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE users.students_guardians SET permissions = permissions - 'parent_portal.consent.manage',
			can_pickup = false, pickup_notes = NULL, relationship_type = 'guardian' WHERE tenant_id = ? AND id = ?`, tenantID, linkID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)
	var grants int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM meta.parent_student_consent_permission_grants WHERE student_guardian_id = ?`, linkID).Scan(ctx, &grants))
	require.Zero(t, grants, "withdrawing the permission drops the recorded grant")

	// Unlink.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM users.students_guardians WHERE tenant_id = ? AND id = ?`, tenantID, linkID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)
	var gone bool
	require.NoError(t, db.NewRaw(`SELECT NOT EXISTS (SELECT 1 FROM users.student_guardian_pickup_permissions WHERE relationship_id = ?)
		AND NOT EXISTS (SELECT 1 FROM auth.guardian_student_access WHERE relationship_id = ?)`, linkID, linkID).Scan(ctx, &gone))
	require.True(t, gone, "the owner halves follow the relationship")
}

func TestGuardianCompatibilityMirrorsOwnerWrites(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, ids := cutOverGuardianFixture(t, db, 2)
	var childID int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM users.student_guardian_relationships WHERE id = ?`, ids[0]).Scan(ctx, &childID))
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Mirror", "Second", "guardian-mirror-second")
	writesBefore := guardianCompatibilityWrites(t, db)

	// The owners' unit of work: relationship first (demoting the previous
	// primary and payer), then Care Plan and Identity.
	var linkID int64
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users.student_guardian_relationships SET is_primary = false, is_payer = false
			WHERE tenant_id = ? AND student_id = ?`, tenantID, childID); err != nil {
			return err
		}
		if err := tx.NewRaw(`INSERT INTO users.student_guardian_relationships
			(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role, is_primary, is_emergency_contact, emergency_priority, is_payer)
			VALUES (?, ?, ?, 'relative', 'pickup_only', true, true, 3, true) RETURNING id`, tenantID, childID, second.ID).Scan(ctx, &linkID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id, can_pickup, pickup_notes)
			VALUES (?, ?, true, 'Oma')`, tenantID, linkID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id, permissions)
			VALUES (?, ?, NULL, '{"parent_portal.access": false}')`, tenantID, linkID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users.student_guardian_pickup_permissions SET pickup_notes = 'Oma, nur freitags'
			WHERE tenant_id = ? AND relationship_id = ?`, tenantID, linkID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.access": true}'
			WHERE tenant_id = ? AND relationship_id = ?`, tenantID, linkID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)

	// A portal account linked on the profile rebinds every access row.
	account := testpkg.CreateTestAccount(t, db, "guardian-mirror-second-account")
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE tenant_id = ? AND id = ?`,
			account.ID, tenantID, second.ID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM users.student_guardian_relationships WHERE tenant_id = ? AND id = ?`, tenantID, linkID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)
	require.Equal(t, writesBefore, guardianCompatibilityWrites(t, db), "owner writes never count as previous-image hits")

	// Deleting the child cascades into the relationships and the mirror
	// separately; neither is a previous-image write.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM users.student_profiles WHERE tenant_id = ? AND id = ?`, tenantID, childID)
		return err
	}))
	requireGuardianMirrorEqual(t, db, tenantID)
	require.Equal(t, writesBefore, guardianCompatibilityWrites(t, db))

	// A linked guardian cannot be deleted blindly (#819).
	var linkedGuardian int64
	require.NoError(t, db.NewRaw(`SELECT guardian_profile_id FROM users.student_guardian_relationships WHERE id = ?`, ids[1]).Scan(ctx, &linkedGuardian))
	_, err := db.ExecContext(ctx, `DELETE FROM users.guardian_profiles WHERE id = ?`, linkedGuardian)
	requirePresenceSQLState(t, err, "23503")
}

func TestGuardianCompatibilityTenantIsolation(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, ids := guardianCutoverFixture(t, db, 2)
	other := guardianOwnerSecondTenant(t, db)
	otherIDs := guardianOwnerFixture(t, db, other, 2)
	_, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	require.NoError(t, finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility))
	otherBefore := guardianOwnerShapeJSON(t, db, other)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		var visible, foreign int64
		if err := tx.NewRaw(`SELECT
			(SELECT count(*) FROM users.student_guardian_relationships) + (SELECT count(*) FROM users.student_guardian_pickup_permissions)
			+ (SELECT count(*) FROM auth.guardian_student_access) + (SELECT count(*) FROM users.students_guardians),
			(SELECT count(*) FROM users.student_guardian_relationships WHERE tenant_id <> ?)`, tenantID).Scan(ctx, &visible, &foreign); err != nil {
			return err
		}
		require.EqualValues(t, 4*len(ids), visible, "the school sees its own rows in all four tables")
		require.Zero(t, foreign)
		// A previous-image write naming the other school's link changes
		// nothing: RLS hides the mirror row, so nothing is routed.
		result, err := tx.ExecContext(ctx, `UPDATE users.students_guardians SET pickup_notes = 'fremd' WHERE id IN (?, ?)`, otherIDs[0], otherIDs[1])
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		require.NoError(t, err)
		require.Zero(t, affected)
		return nil
	}))
	require.JSONEq(t, otherBefore, guardianOwnerShapeJSON(t, db, other))
	requireGuardianMirrorEqual(t, db, other)
	requireGuardianMirrorEqual(t, db, tenantID)
}

func TestGuardianOwnerCutoverMovesThePushAudience(t *testing.T) {
	t.Parallel()
	db := setupGuardianStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID, ids := cutOverGuardianFixture(t, db, 1)
	var childID, guardianID int64
	require.NoError(t, db.NewRaw(`SELECT student_id, guardian_profile_id FROM users.student_guardian_relationships WHERE id = ?`, ids[0]).Scan(ctx, &childID, &guardianID))
	var accountID *int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM users.guardian_profiles WHERE id = ?`, guardianID).Scan(ctx, &accountID))
	require.NotNil(t, accountID, "the first fixture guardian holds a portal account")
	_, err := db.ExecContext(ctx, `INSERT INTO iot.push_subscriptions (tenant_id, account_id, portal, endpoint, p256dh, auth)
		VALUES (?, ?, 'parent', 'https://push.example.test/guardian-cutover', 'p256dh', 'auth')`, tenantID, *accountID)
	require.NoError(t, err)
	audience := func() []int64 {
		var ids []int64
		require.NoError(t, db.NewRaw(`SELECT unnest(guardian_student_ids) FROM platform.delivery_push_subscriptions
			WHERE tenant_id = ? AND account_id = ?`, tenantID, *accountID).Scan(ctx, &ids))
		return ids
	}
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = permissions || '{"parent_portal.access": true}'
		WHERE relationship_id = ?`, ids[0])
	require.NoError(t, err)
	require.Equal(t, []int64{childID}, audience())
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = permissions || '{"parent_portal.access": false}'
		WHERE relationship_id = ?`, ids[0])
	require.NoError(t, err)
	require.Empty(t, audience(), "the push audience follows the Identity permission")
}
