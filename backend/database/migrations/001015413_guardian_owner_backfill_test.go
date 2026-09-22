package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// guardianOwnerFixture populates one tenant with student/guardian links
// covering every copied column: role presets with their portal permissions,
// the primary, emergency, payer and pickup flags, notes and, for every other
// guardian, a linked portal account. Every link gets its own child so the
// single-primary and single-payer keys never collide; tests that need two
// guardians of one child build that shape themselves. It returns the link IDs
// in insertion order.
func guardianOwnerFixture(t *testing.T, db *testpkg.DB, tenantID int64, count int) []int64 {
	t.Helper()
	ctx := context.Background()
	roles := []string{"primary_guardian", "legal_guardian", "co_guardian", "emergency_contact", "custom"}
	ids := make([]int64, 0, count)
	for i := range count {
		student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Backfill", fmt.Sprintf("Child %d", i), "3a")
		guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Backfill", fmt.Sprintf("Guardian %d", i), fmt.Sprintf("guardian-backfill-%d", i))
		if i%2 == 0 {
			account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("guardian-backfill-%d", i))
			_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, account.ID, guardian.ID)
			require.NoError(t, err)
		}
		link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, guardian.ID, roles[i%len(roles)])
		_, err := db.ExecContext(ctx, `
			UPDATE users.students_guardians SET
				is_primary = ?, is_emergency_contact = ?, emergency_priority = ?, is_payer = ?,
				can_pickup = ?, pickup_notes = ?, relationship_type = ?,
				updated_at = now() - make_interval(mins => ?)
			WHERE id = ?`,
			i%2 == 0, i%3 != 0, i%3+1, i%4 == 0, i%3 == 0, fmt.Sprintf("notes %d", i),
			[]string{"parent", "guardian", "relative"}[i%3], i+1, link.ID)
		require.NoError(t, err)
		ids = append(ids, link.ID)
	}
	return ids
}

func guardianOwnerCheckpoint(t *testing.T, report *GuardianOwnerBackfillReport, tenantID int64) GuardianOwnerBackfillCheckpoint {
	t.Helper()
	for _, tenant := range report.Tenants {
		if tenant.TenantID == tenantID {
			return tenant
		}
	}
	require.FailNowf(t, "missing checkpoint", "tenant %d has no checkpoint", tenantID)
	return GuardianOwnerBackfillCheckpoint{}
}

func requireGuardianOwnerTenantEqual(t *testing.T, db *testpkg.DB, report *GuardianOwnerBackfillReport, tenantID int64) GuardianOwnerBackfillCheckpoint {
	t.Helper()
	ctx := context.Background()
	cp := guardianOwnerCheckpoint(t, report, tenantID)
	require.True(t, cp.Stable, "tenant %d must be stable: %+v", tenantID, cp)
	require.True(t, cp.Verified())
	require.Zero(t, cp.MismatchCount)
	require.Zero(t, cp.GuardianAccessMismatchCount)
	require.Nil(t, cp.OldestUnmigratedAt)
	require.NotNil(t, cp.StableAt)
	var sourceCount, relationshipCount, pickupCount, accessCount int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.students_guardians WHERE tenant_id = ?`, tenantID).Scan(ctx, &sourceCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_guardian_relationships WHERE tenant_id = ?`, tenantID).Scan(ctx, &relationshipCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_guardian_pickup_permissions WHERE tenant_id = ?`, tenantID).Scan(ctx, &pickupCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.guardian_student_access WHERE tenant_id = ?`, tenantID).Scan(ctx, &accessCount))
	require.Equal(t, sourceCount, cp.SourceCount)
	require.Equal(t, sourceCount, relationshipCount, "relationship identity must be preserved")
	require.Equal(t, sourceCount, pickupCount, "every relationship carries exactly one pickup permission")
	require.Equal(t, sourceCount, accessCount, "every relationship carries exactly one access row")
	var orphans int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_guardian_relationships r
		WHERE r.tenant_id = ? AND NOT EXISTS (SELECT 1 FROM users.students_guardians sg WHERE sg.id = r.id AND sg.tenant_id = r.tenant_id)`, tenantID).Scan(ctx, &orphans))
	require.Zero(t, orphans)
	var rowMismatches int64
	require.NoError(t, db.NewRaw(guardianOwnerMismatch, tenantID, tenantID).Scan(ctx, &rowMismatches, new(*time.Time)))
	require.Zero(t, rowMismatches, "targets must reproduce every source row")
	var bindingDrift int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.guardian_student_access a
		JOIN users.student_guardian_relationships r ON r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		JOIN users.guardian_profiles g ON g.tenant_id = r.tenant_id AND g.id = r.guardian_profile_id
		WHERE a.tenant_id = ? AND a.account_id IS DISTINCT FROM g.account_id`, tenantID).Scan(ctx, &bindingDrift))
	require.Zero(t, bindingDrift, "every access row binds the guardian's current account")
	return cp
}

func guardianOwnerSecondTenant(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	return tenantID
}

func guardianSourceRows(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(string_agg(row_to_json(sg)::text, ',' ORDER BY id), '') FROM users.students_guardians sg WHERE tenant_id = ?`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func guardianTargetRows(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(string_agg(to_jsonb(r)::text, ',' ORDER BY r.id), '') FROM (`+guardianOwnerTargetProjection+`) r`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func TestGuardianOwnerBackfillCopiesAndVerifiesPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := guardianOwnerSecondTenant(t, db)
	idsA := guardianOwnerFixture(t, db, tenantA, 7)
	guardianOwnerFixture(t, db, tenantB, 3)
	sourceA := guardianSourceRows(t, db, tenantA)
	sourceB := guardianSourceRows(t, db, tenantB)

	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	cpA := requireGuardianOwnerTenantEqual(t, db, report, tenantA)
	cpB := requireGuardianOwnerTenantEqual(t, db, report, tenantB)
	require.EqualValues(t, 7, cpA.RowsCopied)
	require.EqualValues(t, 3, cpB.RowsCopied)
	require.EqualValues(t, 7, cpA.SourceCount)
	require.Equal(t, cpA.SourceChecksum, cpA.TargetChecksum)
	require.NotEqual(t, cpA.SourceChecksum, cpB.SourceChecksum)
	require.EqualValues(t, 21, cpA.RowsVisibleToTenant, "one relationship, pickup permission and access row per link")
	require.Zero(t, cpA.RowsRejected)
	require.Zero(t, cpA.RowsRemoved)
	require.EqualValues(t, 2, cpA.Pass, "a second pass proves the first changed everything and the second nothing")
	require.Equal(t, sourceA, guardianSourceRows(t, db, tenantA), "the old table is never modified")
	require.Equal(t, sourceB, guardianSourceRows(t, db, tenantB))

	// Column mapping: the People row keeps the link id, the Care Plan and
	// Identity rows hang off it, and the account binding mirrors the profile.
	for i, id := range idsA {
		var role, notes string
		var primary, payer, pickup bool
		var accountID *int64
		var permissions string
		require.NoError(t, db.NewRaw(`
			SELECT r.guardian_role, r.is_primary, r.is_payer, p.can_pickup, p.pickup_notes, a.account_id, a.permissions::text
			FROM users.student_guardian_relationships r
			JOIN users.student_guardian_pickup_permissions p ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
			JOIN auth.guardian_student_access a ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id
			WHERE r.id = ?`, id).Scan(ctx, &role, &primary, &payer, &pickup, &notes, &accountID, &permissions))
		var sourceRole, sourcePermissions string
		var sourceAccount *int64
		require.NoError(t, db.NewRaw(`SELECT sg.guardian_role, sg.permissions::text, g.account_id
			FROM users.students_guardians sg JOIN users.guardian_profiles g ON g.id = sg.guardian_profile_id
			WHERE sg.id = ?`, id).Scan(ctx, &sourceRole, &sourcePermissions, &sourceAccount))
		require.Equal(t, sourceRole, role)
		require.Equal(t, sourcePermissions, permissions)
		require.Equal(t, sourceAccount, accountID)
		require.Equal(t, i%2 == 0, primary)
		require.Equal(t, i%4 == 0, payer)
		require.Equal(t, i%3 == 0, pickup)
		require.Equal(t, fmt.Sprintf("notes %d", i), notes)
		if i%2 == 0 {
			require.NotNil(t, accountID, "linked portal accounts are bound")
		} else {
			require.Nil(t, accountID)
		}
	}

	var nextRelationshipID, maxLinkID int64
	require.NoError(t, db.NewRaw(`SELECT nextval('users.student_guardian_relationships_id_seq')`).Scan(ctx, &nextRelationshipID))
	require.NoError(t, db.NewRaw(`SELECT max(id) FROM users.students_guardians`).Scan(ctx, &maxLinkID))
	require.Greater(t, nextRelationshipID, maxLinkID, "relationship sequence must not collide with preserved identities")
}

func TestGuardianOwnerBackfillInterruptsAndResumesAtEveryBatchBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 5)
	errInterrupt := errors.New("interrupted after batch")
	var batches []guardianOwnerBatch
	opts := GuardianOwnerBackfillOptions{BatchSize: 2, afterBatch: func(id int64, batch guardianOwnerBatch) error {
		if id != tenantID {
			return nil
		}
		batches = append(batches, batch)
		if batch.Scanned > 0 {
			return errInterrupt
		}
		return nil
	}}
	runs := 0
	var report *GuardianOwnerBackfillReport
	for {
		runs++
		require.Less(t, runs, 20, "resume must converge")
		var err error
		report, err = RunGuardianOwnerBackfill(ctx, db, opts)
		if err == nil {
			break
		}
		require.ErrorIs(t, err, errInterrupt)
		status, statusErr := GuardianOwnerBackfillStatus(ctx, db)
		require.NoError(t, statusErr)
		cp := guardianOwnerCheckpoint(t, status, tenantID)
		require.False(t, cp.Stable)
		require.Equal(t, batches[len(batches)-1].LastID, cp.HighWaterID, "the committed batch persists its high-water mark")
	}
	// Two passes over five rows in batches of two: (2,2,1) rows each, and
	// each pass is closed by an empty batch that is not interrupted.
	require.Equal(t, 7, runs)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 8, cp.BatchesCompleted)
	require.EqualValues(t, 10, cp.RowsScanned)
	require.EqualValues(t, 5, cp.RowsCopied)
	require.EqualValues(t, 5, cp.RowsSkipped)
	for i, batch := range batches[:3] {
		require.EqualValues(t, []int64{2, 2, 1}[i], batch.Copied, "first pass copies each batch exactly once")
	}
	for _, batch := range batches[3:] {
		require.Zero(t, batch.Copied, "resumed passes never rewrite unchanged rows")
	}
}

func TestGuardianOwnerBackfillRerunsCompletedBatchesIdempotently(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 4)
	first, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	before := requireGuardianOwnerTenantEqual(t, db, first, tenantID)
	targetBefore := guardianTargetRows(t, db, tenantID)

	// Rewind the persisted high-water mark as if the last checkpoint write
	// had been lost, so every completed batch runs again.
	_, err = db.ExecContext(ctx, `UPDATE platform.storage_backfill_checkpoints SET high_water_id = 0, pass_completed = false, stable = false WHERE backfill = ? AND tenant_id = ?`, GuardianOwnerBackfillName, tenantID)
	require.NoError(t, err)
	second, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	after := requireGuardianOwnerTenantEqual(t, db, second, tenantID)
	require.Equal(t, before.RowsCopied, after.RowsCopied, "rerunning completed batches copies nothing")
	require.Equal(t, before.RowsSkipped+4, after.RowsSkipped)
	require.Equal(t, before.SourceChecksum, after.TargetChecksum)
	require.Equal(t, targetBefore, guardianTargetRows(t, db, tenantID))
	require.Equal(t, before.Pass, after.Pass, "a rewound pass stabilises without starting another")
}

func TestGuardianOwnerBackfillRetriesInjectedFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherTenant := guardianOwnerSecondTenant(t, db)
	guardianOwnerFixture(t, db, tenantID, 3)
	guardianOwnerFixture(t, db, otherTenant, 2)
	raise := func(ctx context.Context, tx testpkg.Tx, code string) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected %s' USING ERRCODE = '%s'; END $$`, code, code))
		return err
	}
	codes := map[int]string{1: "40P01", 2: "40001", 3: "55P03"}
	opts := GuardianOwnerBackfillOptions{BatchSize: 10, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, attempt int) error {
		if id != tenantID {
			return nil
		}
		if code, ok := codes[attempt]; ok {
			return raise(ctx, tx, code)
		}
		return nil
	}}
	report, err := RunGuardianOwnerBackfill(ctx, db, opts)
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	// One batch per pass, two passes, each retried three times before success.
	require.EqualValues(t, 2, cp.BatchesCompleted)
	require.EqualValues(t, 6, cp.BatchesRetried)
	require.EqualValues(t, 2, cp.Deadlocks)
	require.EqualValues(t, 2, cp.SerializationFailures)
	require.EqualValues(t, 2, cp.LockTimeouts)
	require.EqualValues(t, 3, cp.RowsCopied, "rolled-back attempts must not double count")
	require.EqualValues(t, 6, cp.RowsScanned)

	_, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{MaxAttempts: 2, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		return raise(ctx, tx, "40P01")
	}})
	require.ErrorContains(t, err, "gave up after 2 attempts")
	status, err := GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Equal(t, cp.RowsCopied, guardianOwnerCheckpoint(t, status, tenantID).RowsCopied)

	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET pickup_notes = 'changed while the other school fails' WHERE tenant_id = ?`, otherTenant)
	require.NoError(t, err)
	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		return raise(ctx, tx, "22023")
	}})
	require.ErrorContains(t, err, "22023", "non-transient failures surface immediately")
	require.NotNil(t, report, "the other schools are still visited and reported")
	require.Equal(t, []int64{tenantID}, report.Unstable())
	other := requireGuardianOwnerTenantEqual(t, db, report, otherTenant)
	require.EqualValues(t, 4, other.RowsCopied, "a failing school does not block the remaining schools")
}

func TestGuardianOwnerTerminalFailuresPersistTelemetry(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"40P01", "40001", "55P03"} {
		t.Run(code, func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			db := testpkg.SetupTestDB(t)
			tenantID := testpkg.Tenant(t)
			guardianOwnerFixture(t, db, tenantID, 1)
			report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{MaxAttempts: 2,
				injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
					if id != tenantID {
						return nil
					}
					_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected' USING ERRCODE = '%s'; END $$`, code))
					return err
				},
			})
			require.ErrorContains(t, err, "gave up after 2 attempts")
			cp := guardianOwnerCheckpoint(t, report, tenantID)
			require.Zero(t, cp.HighWaterID)
			require.Zero(t, cp.RowsCopied)
			require.EqualValues(t, 1, cp.BatchesRetried)
			require.EqualValues(t, 2, cp.Deadlocks+cp.SerializationFailures+cp.LockTimeouts)
			report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
			require.NoError(t, err)
			resumed := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
			require.Equal(t, cp.BatchesRetried, resumed.BatchesRetried)
			require.Equal(t, cp.Deadlocks, resumed.Deadlocks)
			require.Equal(t, cp.SerializationFailures, resumed.SerializationFailures)
			require.Equal(t, cp.LockTimeouts, resumed.LockTimeouts)
		})
	}
}

func TestGuardianOwnerBackfillRereadsChangedRowsAndRemovesOrphans(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 5)
	first, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	before := requireGuardianOwnerTenantEqual(t, db, first, tenantID)

	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET pickup_notes = 'changed after backfill', can_pickup = NOT can_pickup WHERE id = ?`, ids[0])
	require.NoError(t, err)
	testpkg.SetTestStudentGuardianLinkRole(t, db, ids[1], "emergency_contact")
	_, err = db.ExecContext(ctx, `DELETE FROM users.students_guardians WHERE id = ?`, ids[2])
	require.NoError(t, err)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Joined", "Later", "3a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Joined", "Later", "guardian-joined-later")
	added := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, guardian.ID, "co_guardian")

	second, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	after := requireGuardianOwnerTenantEqual(t, db, second, tenantID)
	require.Equal(t, before.RowsCopied+3, after.RowsCopied, "changed and new rows are copied once")
	require.EqualValues(t, 1, after.RowsRemoved, "physically deleted source rows leave no orphan")
	require.EqualValues(t, 5, after.SourceCount)
	var notes string
	require.NoError(t, db.NewRaw(`SELECT pickup_notes FROM users.student_guardian_pickup_permissions WHERE relationship_id = ?`, ids[0]).Scan(ctx, &notes))
	require.Equal(t, "changed after backfill", notes)
	var role string
	var permissions string
	require.NoError(t, db.NewRaw(`SELECT r.guardian_role, a.permissions::text FROM users.student_guardian_relationships r
		JOIN auth.guardian_student_access a ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id WHERE r.id = ?`, ids[1]).Scan(ctx, &role, &permissions))
	require.Equal(t, "emergency_contact", role)
	var sourcePermissions string
	require.NoError(t, db.NewRaw(`SELECT permissions::text FROM users.students_guardians WHERE id = ?`, ids[1]).Scan(ctx, &sourcePermissions))
	require.Equal(t, sourcePermissions, permissions, "portal permissions follow the re-filed role")
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.student_guardian_relationships WHERE id = ?)`, ids[2]).Scan(ctx, &exists))
	require.False(t, exists)
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.guardian_student_access WHERE relationship_id = ?)`, ids[2]).Scan(ctx, &exists))
	require.False(t, exists, "the cascade removed the pickup permission and access row with it")
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.guardian_student_access WHERE relationship_id = ?)`, added.ID).Scan(ctx, &exists))
	require.True(t, exists)
}

// A guardian who gains a portal account after the copy edits
// users.guardian_profiles, not the link. The Identity row must follow the
// binding: it is reported as a guardian-access mismatch until the next pass
// rebinds it, and an account unlinked later is dropped the same way.
func TestGuardianOwnerBackfillFollowsAccountBinding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 2)
	first, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, first, tenantID)

	// ids[1] belongs to a guardian without an account; ids[0] to one with.
	var unlinkedGuardian, linkedGuardian int64
	require.NoError(t, db.NewRaw(`SELECT guardian_profile_id FROM users.students_guardians WHERE id = ?`, ids[1]).Scan(ctx, &unlinkedGuardian))
	require.NoError(t, db.NewRaw(`SELECT guardian_profile_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &linkedGuardian))
	account := testpkg.CreateTestAccount(t, db, "guardian-bound-later")
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, account.ID, unlinkedGuardian)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = NULL, has_account = false WHERE id = ?`, linkedGuardian)
	require.NoError(t, err)

	var drift int64
	var oldest *time.Time
	require.NoError(t, db.NewRaw(guardianOwnerAccessMismatch, tenantID).Scan(ctx, &drift, &oldest))
	require.EqualValues(t, 2, drift, "both bindings drifted without any source row changing")
	require.NotNil(t, oldest, "the drift is dated by the profile edit")

	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 4, cp.RowsCopied, "only the two rebound access rows are rewritten")
	var boundTo, released sql.NullInt64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.guardian_student_access WHERE relationship_id = ?`, ids[1]).Scan(ctx, &boundTo))
	require.True(t, boundTo.Valid)
	require.Equal(t, account.ID, boundTo.Int64)
	require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.guardian_student_access WHERE relationship_id = ?`, ids[0]).Scan(ctx, &released))
	require.False(t, released.Valid, "an unlinked account is released")
}

// A link whose guardian belongs to another school cannot normally exist: the
// old table carries the composite guardian foreign key. Once such a row is
// forced in, the copy must not launder it into Identity storage with an empty
// binding; it is rejected and keeps the school unverified until corrected.
func TestGuardianOwnerBackfillRejectsCrossTenantGuardian(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := guardianOwnerSecondTenant(t, db)
	idsA := guardianOwnerFixture(t, db, tenantA, 2)
	guardianOwnerFixture(t, db, tenantB, 1)
	foreign := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantB, "Foreign", "Guardian", "guardian-foreign")
	_, err := db.ExecContext(ctx, `ALTER TABLE users.students_guardians DISABLE TRIGGER ALL`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET guardian_profile_id = ? WHERE id = ?`, foreign.ID, idsA[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `ALTER TABLE users.students_guardians ENABLE TRIGGER ALL`)
	require.NoError(t, err)

	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{MaxPasses: 2})
	require.NoError(t, err)
	require.Equal(t, []int64{tenantA}, report.Unstable())
	requireGuardianOwnerTenantEqual(t, db, report, tenantB)
	cp := guardianOwnerCheckpoint(t, report, tenantA)
	require.False(t, cp.Stable)
	require.False(t, cp.Verified())
	require.EqualValues(t, 1, cp.MismatchCount)
	require.EqualValues(t, 2, cp.RowsRejected, "the rejected row is reported on every pass")
	require.EqualValues(t, 2, cp.SourceCount)
	require.EqualValues(t, 1, cp.TargetCount)
	require.NotNil(t, cp.OldestUnmigratedAt)
	require.Greater(t, cp.OldestUnmigratedAge(time.Now()), time.Duration(0))
	require.EqualValues(t, 2, cp.Pass)
	require.Equal(t, idsA[1], cp.HighWaterID, "the pass limit keeps the completed pass and its high-water mark")
	var copied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.student_guardian_relationships WHERE id = ?)`, idsA[0]).Scan(ctx, &copied))
	require.False(t, copied)

	own := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantA, "Own", "Guardian", "guardian-own")
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET guardian_profile_id = ? WHERE id = ?`, own.ID, idsA[0])
	require.NoError(t, err)
	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantA)
}

// A link deleted and recreated keeps its (student, guardian) pair, so the new
// id collides with the relationship of the deleted one on the pair key. The
// pass rewinds, removes the orphan and converges.
func TestGuardianOwnerBackfillRestartsPassWhenLinkIsRecreatedMidPass(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherTenant := guardianOwnerSecondTenant(t, db)
	ids := guardianOwnerFixture(t, db, tenantID, 2)
	guardianOwnerFixture(t, db, otherTenant, 1)
	first, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, first, tenantID)

	var studentID, guardianID int64
	require.NoError(t, db.NewRaw(`SELECT student_id, guardian_profile_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &studentID, &guardianID))
	_, err = db.ExecContext(ctx, `DELETE FROM users.students_guardians WHERE id = ?`, ids[0])
	require.NoError(t, err)
	replacement := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, studentID, guardianID, "legal_guardian")
	require.Greater(t, replacement.ID, ids[1])
	sourceAfterRecreate := guardianSourceRows(t, db, tenantID)
	otherSource := guardianSourceRows(t, db, otherTenant)

	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 1})
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 2, cp.SourceCount)
	require.EqualValues(t, 1, cp.RowsRemoved)
	require.EqualValues(t, 1, cp.BatchesRetried)
	var pairOwner int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?`, tenantID, studentID, guardianID).Scan(ctx, &pairOwner))
	require.Equal(t, replacement.ID, pairOwner)
	require.Equal(t, sourceAfterRecreate, guardianSourceRows(t, db, tenantID), "recovery must not modify authoritative source rows")
	require.Equal(t, otherSource, guardianSourceRows(t, db, otherTenant))
	requireGuardianOwnerTenantEqual(t, db, report, otherTenant)
}

// Two guardians of one child swapping the primary and payer flags is the
// conflict no orphan sweep can clear: both source rows are alive and keep
// their pairs, but the batch reaches the newly flagged, lower id before the
// demoted, higher one and meets the stale claim on the per-child unique keys.
// The rewind must release that claim itself or the pass restarts until it
// gives up.
func TestGuardianOwnerBackfillRecoversFromPrimaryAndPayerReassignment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Shared", "Child", "2b")
	first := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "First", "Guardian", "guardian-first")
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Second", "Guardian", "guardian-second")
	lower := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, first.ID, "co_guardian")
	higher := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, second.ID, "primary_guardian")
	require.Less(t, lower.ID, higher.ID)
	_, err := db.ExecContext(ctx, `UPDATE users.students_guardians SET is_primary = true, is_payer = true WHERE id = ?`, higher.ID)
	require.NoError(t, err)
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantID)

	// The old table's trigger demotes the previous primary; the payer key
	// needs the explicit two-step swap a real edit would do.
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET is_payer = false WHERE id = ?`, higher.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET is_primary = true, is_payer = true WHERE id = ?`, lower.ID)
	require.NoError(t, err)
	var demoted bool
	require.NoError(t, db.NewRaw(`SELECT is_primary FROM users.students_guardians WHERE id = ?`, higher.ID).Scan(ctx, &demoted))
	require.False(t, demoted, "the source trigger moved the primary flag")
	sourceAfterSwap := guardianSourceRows(t, db, tenantID)

	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{BatchSize: 1})
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 2, cp.SourceCount)
	require.Positive(t, cp.BatchesRetried, "the conflict must be observed, not avoided by luck")
	require.Positive(t, cp.RowsRemoved, "the stale claim was released by the rewind")
	var primaryOwner, payerOwner int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE tenant_id = ? AND student_id = ? AND is_primary`, tenantID, student.ID).Scan(ctx, &primaryOwner))
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE tenant_id = ? AND student_id = ? AND is_payer`, tenantID, student.ID).Scan(ctx, &payerOwner))
	require.Equal(t, lower.ID, primaryOwner)
	require.Equal(t, lower.ID, payerOwner)
	require.Equal(t, sourceAfterSwap, guardianSourceRows(t, db, tenantID), "recovery must not modify authoritative source rows")
}

func TestGuardianOwnerVerificationUsesOneUTCSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 1)
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := guardianOwnerCheckpoint(t, report, tenantID)
	previousChecksum := cp.SourceChecksum
	run := guardianOwnerTenantRun{db: db, tenantID: tenantID, opts: GuardianOwnerBackfillOptions{
		afterSourceVerification: func(ctx context.Context, tx testpkg.Tx) error {
			var zone string
			if err := tx.NewRaw(`SHOW TIME ZONE`).Scan(ctx, &zone); err != nil {
				return err
			}
			require.Equal(t, "UTC", zone)
			_, err := db.ExecContext(ctx, `UPDATE users.students_guardians SET pickup_notes = 'changed between verification queries' WHERE id = ?`, ids[0])
			return err
		},
	}.withDefaults()}
	require.NoError(t, run.verify(ctx, &cp))
	require.True(t, cp.Verified(), "source and target evidence must describe the same pre-edit snapshot")
	require.Equal(t, previousChecksum, cp.SourceChecksum)
	require.NotEmpty(t, cp.VerificationSnapshot)
	var mismatches int64
	require.NoError(t, db.NewRaw(guardianOwnerMismatch, tenantID, tenantID).Scan(ctx, &mismatches, new(*time.Time)))
	require.EqualValues(t, 1, mismatches, "the real source edit committed outside the verification snapshot")
	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	updated := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.NotEqual(t, previousChecksum, updated.SourceChecksum)
}

func TestGuardianOwnerRunExcludesOtherWriters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 1)
	checked := false
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{afterBatch: func(id int64, _ guardianOwnerBatch) error {
		if id != tenantID || checked {
			return nil
		}
		checked = true
		_, runErr := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
		require.ErrorContains(t, runErr, "another run, reset or rollback is active")
		require.ErrorContains(t, ResetGuardianOwnerBackfill(ctx, db), "another run, reset or rollback is active")
		require.ErrorContains(t, guardianOwnerBackfillDown(ctx, db), "another run, reset or rollback is active")
		return nil
	}})
	require.NoError(t, err)
	require.True(t, checked)
	requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.NoError(t, ResetGuardianOwnerBackfill(ctx, db), "lock must be released after the completed run")
}

func TestGuardianOwnerCancellationFlushesFailedAttempt(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 1)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	defer cancel()
	_, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		_, err := tx.ExecContext(ctx, `DO $$ BEGIN RAISE EXCEPTION 'injected cancellation' USING ERRCODE = '40P01'; END $$`)
		cancel()
		return err
	}})
	require.ErrorIs(t, err, context.Canceled)
	report, err := GuardianOwnerBackfillStatus(t.Context(), db)
	require.NoError(t, err)
	cp := guardianOwnerCheckpoint(t, report, tenantID)
	require.EqualValues(t, 1, cp.Deadlocks)
	require.Zero(t, cp.BatchesRetried, "cancellation prevented another attempt from starting")
	require.Zero(t, cp.HighWaterID)
	require.Zero(t, cp.RowsCopied)
	require.NoError(t, ResetGuardianOwnerBackfill(t.Context(), db), "cancelled run must release exclusion lock")
}

func TestGuardianOwnerMeasuresSuccessfulLockWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 1)
	_, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET pickup_notes = 'requires target update' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT id FROM users.student_guardian_relationships WHERE id = ? FOR UPDATE`, ids[0])
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, runErr := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{}); done <- runErr }()
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname = current_database()
			AND wait_event_type = 'Lock' AND query LIKE '%eligible AS%')`).Scan(ctx, &blocked)
		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	require.NoError(t, tx.Commit())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("backfill did not finish after releasing the lock")
	}
	report, err := GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.Greater(t, cp.LockWaitMs, float64(50))
	require.Zero(t, cp.LockTimeouts, "a real lock wait need not time out")
}

func TestGuardianOwnerBackfillTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := guardianOwnerSecondTenant(t, db)
	guardianOwnerFixture(t, db, tenantA, 2)
	guardianOwnerFixture(t, db, tenantB, 3)
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantA)
	requireGuardianOwnerTenantEqual(t, db, report, tenantB)
	for tenantID, expected := range map[int64]int64{tenantA: 2, tenantB: 3} {
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
				return err
			}
			for _, table := range []string{"users.student_guardian_relationships", "users.student_guardian_pickup_permissions", "auth.guardian_student_access"} {
				var tenants []int64
				var count int64
				require.NoError(t, tx.NewRaw("SELECT DISTINCT tenant_id FROM "+table).Scan(ctx, &tenants))
				require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+table).Scan(ctx, &count))
				require.Equal(t, []int64{tenantID}, tenants)
				require.Equal(t, expected, count)
			}
			_, err := tx.ExecContext(ctx, `SELECT count(*) FROM platform.storage_backfill_checkpoints`)
			require.ErrorContains(t, err, "42501", "application roles must not read backfill checkpoints")
			return nil
		})
		require.NoError(t, err)
	}
}

func TestGuardianOwnerBackfillVerifiesTenantPolicies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := guardianOwnerSecondTenant(t, db)
	guardianOwnerFixture(t, db, tenantA, 2)
	guardianOwnerFixture(t, db, tenantB, 3)
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	cpA := requireGuardianOwnerTenantEqual(t, db, report, tenantA)
	cpB := requireGuardianOwnerTenantEqual(t, db, report, tenantB)
	require.EqualValues(t, 6, cpA.RowsVisibleToTenant, "one relationship, pickup permission and access row per link")
	require.EqualValues(t, 9, cpB.RowsVisibleToTenant)

	// A policy dropped after Expand leaves the target default-deny under FORCE
	// row-level security: the tenant role stops seeing its own rows.
	_, err = db.ExecContext(ctx, `DROP POLICY tenant_isolation_auth_guardian_student_access ON auth.guardian_student_access`)
	require.NoError(t, err)
	_, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.ErrorContains(t, err, "row-level security leaked")
	require.ErrorContains(t, err, "own rows")
	status, err := GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Equal(t, []int64{tenantA, tenantB}, status.Unstable(), "an unproven boundary blocks Cutover for every school")

	// A policy widened to every school is the opposite failure and must not
	// pass as "the role can read its rows".
	_, err = db.ExecContext(ctx, `CREATE POLICY tenant_isolation_auth_guardian_student_access
		ON auth.guardian_student_access FOR ALL USING (true) WITH CHECK (true)`)
	require.NoError(t, err)
	_, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.ErrorContains(t, err, "foreign rows")
}

func TestGuardianOwnerBackfillIndexesAndQueryPlans(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 3)
	_, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	var invalidIndexes int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid
		WHERE c.oid IN ('users.student_guardian_relationships'::regclass, 'users.student_guardian_pickup_permissions'::regclass,
			'auth.guardian_student_access'::regclass, 'users.students_guardians'::regclass)
		  AND NOT (i.indisvalid AND i.indisready)`).Scan(ctx, &invalidIndexes))
	require.Zero(t, invalidIndexes)
	var constraintViolations int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_guardian_relationships r
		LEFT JOIN users.guardian_profiles g ON g.id = r.guardian_profile_id AND g.tenant_id = r.tenant_id
		LEFT JOIN users.student_profiles s ON s.id = r.student_id AND s.tenant_id = r.tenant_id
		WHERE g.id IS NULL OR s.id IS NULL`).Scan(ctx, &constraintViolations))
	require.Zero(t, constraintViolations)
	var danglingDependents int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.student_guardian_pickup_permissions p
			WHERE NOT EXISTS (SELECT 1 FROM users.student_guardian_relationships r WHERE r.tenant_id = p.tenant_id AND r.id = p.relationship_id))
		+ (SELECT count(*) FROM auth.guardian_student_access a
			WHERE NOT EXISTS (SELECT 1 FROM users.student_guardian_relationships r WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id))`).Scan(ctx, &danglingDependents))
	require.Zero(t, danglingDependents)

	// The keyset batch must be index-driven on the old table: with sequential
	// scans disabled the planner still finds an index path, proving that the
	// tenant/id predicate is covered.
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			return err
		}
		for name, query := range map[string]struct {
			sql  string
			args []any
		}{
			"batch":           {guardianOwnerCopyBatch, []any{tenantID, int64(0), 100, int64(0)}},
			"source checksum": {guardianOwnerSourceChecksum, []any{tenantID}},
			"target checksum": {guardianOwnerTargetChecksum, []any{tenantID}},
			"mismatch":        {guardianOwnerMismatch, []any{tenantID, tenantID}},
			"access":          {guardianOwnerAccessMismatch, []any{tenantID}},
		} {
			var lines []string
			if err := tx.NewRaw(`EXPLAIN `+query.sql, query.args...).Scan(ctx, &lines); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			plan := strings.Join(lines, "\n")
			require.Contains(t, plan, "Index", "%s plan: %s", name, plan)
			require.NotContains(t, plan, "Seq Scan", "%s must be index-driven on every tenant-scoped table: %s", name, plan)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGuardianOwnerBackfillResetAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 3)
	sourceBefore := guardianSourceRows(t, db, tenantID)
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantID)

	require.NoError(t, ResetGuardianOwnerBackfill(ctx, db))
	requireGuardianOwnerTargetsEmpty(t, db)
	status, err := GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Empty(t, status.Tenants)
	require.Contains(t, status.MissingTenants, tenantID)
	require.False(t, status.Stable(), "unvisited schools block Cutover")
	require.Equal(t, sourceBefore, guardianSourceRows(t, db, tenantID))

	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 3, cp.RowsCopied, "reset restarts from zero")

	// The checkpoint table is shared with the other storage backfills: rolling
	// back this one keeps their progress and drops the table only once empty.
	_, err = db.ExecContext(ctx, `INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id) VALUES ('other-backfill', ?)`, tenantID)
	require.NoError(t, err)
	require.NoError(t, guardianOwnerBackfillDown(ctx, db))
	requireGuardianOwnerTargetsEmpty(t, db)
	var remaining []string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT backfill FROM platform.storage_backfill_checkpoints`).Scan(ctx, &remaining))
	require.Contains(t, remaining, "other-backfill")
	require.NotContains(t, remaining, GuardianOwnerBackfillName,
		"this rollback removes exactly its own rows and leaves every other backfill's progress")
	_, err = db.ExecContext(ctx, `DELETE FROM platform.storage_backfill_checkpoints`)
	require.NoError(t, err)
	require.NoError(t, guardianOwnerBackfillDown(ctx, db))
	var checkpointsAbsent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('platform.storage_backfill_checkpoints') IS NULL`).Scan(ctx, &checkpointsAbsent))
	require.True(t, checkpointsAbsent)
	require.Equal(t, sourceBefore, guardianSourceRows(t, db, tenantID))
	require.NoError(t, guardianOwnerBackfillUp(ctx, db), "the migration recreates the dropped checkpoint table with its own columns")
	status, err = GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, status, tenantID)
	require.NoError(t, guardianOwnerBackfillUp(ctx, db), "the migration is idempotent for resumed deployments")

	// After Cutover users.students_guardians becomes a compatibility view over
	// the targets; target-only truncation would then destroy authoritative rows.
	_, err = db.ExecContext(ctx, `ALTER TABLE users.students_guardians RENAME TO students_guardians_legacy; CREATE VIEW users.students_guardians AS SELECT * FROM users.students_guardians_legacy`)
	require.NoError(t, err)
	require.ErrorContains(t, ResetGuardianOwnerBackfill(ctx, db), "not a base table")
	require.ErrorContains(t, guardianOwnerBackfillDown(ctx, db), "not a base table")
	_, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.ErrorContains(t, err, "not a base table")
	var relationshipCount int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_guardian_relationships WHERE tenant_id = ?`, tenantID).Scan(ctx, &relationshipCount))
	require.EqualValues(t, 3, relationshipCount)
	_, err = db.ExecContext(ctx, `DROP VIEW users.students_guardians; ALTER TABLE users.students_guardians_legacy RENAME TO students_guardians`)
	require.NoError(t, err)
}

// The Expand rollback follows once the targets are empty and the checkpoint
// table is gone; the pair of migrations then replays cleanly on top. Expand
// still names users.students in its foreign key, so this runs on the
// historical pre-Cutover student schema like the Expand tests themselves.
func TestGuardianOwnerBackfillRollbackPermitsExpandRollback(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	guardianOwnerFixture(t, db, tenantID, 2)
	_, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	require.ErrorContains(t, guardianStorageExpandDown(ctx, db), "requires empty target tables")
	_, err = db.ExecContext(ctx, `DELETE FROM platform.storage_backfill_checkpoints WHERE backfill <> ?`, GuardianOwnerBackfillName)
	require.NoError(t, err)
	require.NoError(t, guardianOwnerBackfillDown(ctx, db))
	require.NoError(t, guardianStorageExpandDown(ctx, db), "Expand rollback follows once the targets are empty")
	require.NoError(t, guardianStorageExpandUp(ctx, db))
	require.NoError(t, guardianOwnerBackfillUp(ctx, db))
	status, err := GuardianOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, status, tenantID)
	require.EqualValues(t, 2, cp.RowsCopied)
}

// The old table's single-primary rule lives in a trigger that only fires on
// writes of is_primary; re-parenting a primary row to another child leaves
// that child with two primaries. The target's partial unique index refuses
// both, and a rewind can never release a claim that both source rows hold, so
// the copy must leave them out and report them instead of restarting forever.
func TestGuardianOwnerBackfillRejectsDuplicatePrimaries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 3)
	var childID int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &childID))
	_, err := db.ExecContext(ctx, `UPDATE users.students_guardians SET student_id = ? WHERE id = ?`, childID, ids[2])
	require.NoError(t, err)
	var primaries int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.students_guardians WHERE student_id = ? AND is_primary`, childID).Scan(ctx, &primaries))
	require.EqualValues(t, 2, primaries, "the source trigger does not fire on a student_id change")

	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{MaxPasses: 2})
	require.NoError(t, err, "duplicate primaries must not fail the run")
	require.Equal(t, []int64{tenantID}, report.Unstable())
	cp := guardianOwnerCheckpoint(t, report, tenantID)
	require.False(t, cp.Verified())
	require.EqualValues(t, 4, cp.RowsRejected, "both primaries are rejected on both passes")
	require.EqualValues(t, 2, cp.MismatchCount)
	require.EqualValues(t, 3, cp.SourceCount)
	require.EqualValues(t, 1, cp.TargetCount)
	require.Zero(t, cp.BatchesRetried, "rejecting avoids the unique-conflict rewind")

	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET is_primary = false WHERE id = ?`, ids[2])
	require.NoError(t, err)
	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantID)
}

// "Komplett löschen" deletes a guardian's old links and then the profile in
// one transaction. The copied relationship must not refuse that delete: while
// the old table is authoritative the copy follows the profile, and rollback
// restores the RESTRICT action the authoritative target will need.
func TestGuardianOwnerBackfillDoesNotBlockGuardianDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := guardianOwnerFixture(t, db, tenantID, 2)
	report, err := RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	var guardianID int64
	require.NoError(t, db.NewRaw(`SELECT guardian_profile_id FROM users.students_guardians WHERE id = ?`, ids[0]).Scan(ctx, &guardianID))
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM users.students_guardians WHERE guardian_profile_id = ?`, guardianID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM users.guardian_profiles WHERE id = ?`, guardianID)
		return err
	}), "the copied relationship must not refuse the guardian delete")
	var remaining int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_guardian_relationships WHERE id = ?`, ids[0]).Scan(ctx, &remaining))
	require.Zero(t, remaining, "the copy follows the deleted profile")
	report, err = RunGuardianOwnerBackfill(ctx, db, GuardianOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := requireGuardianOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 1, cp.SourceCount)
	require.Zero(t, cp.RowsRemoved, "nothing is left for the orphan sweep")

	deleteAction := func() string {
		var action string
		require.NoError(t, db.NewRaw(`SELECT confdeltype::text FROM pg_constraint
			WHERE conrelid = 'users.student_guardian_relationships'::regclass
			  AND conname = 'fk_student_guardian_relationships_guardian'`).Scan(ctx, &action))
		return action
	}
	require.Equal(t, "c", deleteAction())
	require.NoError(t, guardianOwnerBackfillDown(ctx, db))
	require.Equal(t, "r", deleteAction(), "rollback restores the Expand action")
	require.NoError(t, guardianOwnerBackfillUp(ctx, db))
	require.Equal(t, "c", deleteAction())
	require.NoError(t, guardianOwnerBackfillUp(ctx, db), "the rewrite is idempotent")
}

func requireGuardianOwnerTargetsEmpty(t *testing.T, db *testpkg.DB) {
	t.Helper()
	for _, table := range []string{"users.student_guardian_relationships", "users.student_guardian_pickup_permissions", "auth.guardian_student_access"} {
		var count int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+table).Scan(context.Background(), &count))
		require.Zero(t, count, "%s must be empty", table)
	}
}
