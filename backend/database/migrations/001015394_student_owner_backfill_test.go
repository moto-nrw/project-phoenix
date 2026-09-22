package migrations

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// studentOwnerFixture populates one tenant with students covering every copied
// column, including the JSONB departure plan, the consent timestamps, a group
// assignment and an enrollment window. It returns the student IDs in insertion
// order.
func studentOwnerFixture(t *testing.T, db *testpkg.DB, tenantID int64, count int) []int64 {
	t.Helper()
	ctx := context.Background()
	group := testpkg.CreateTestEducationGroupForTenant(t, db, tenantID, fmt.Sprintf("Backfill %d", tenantID))
	statuses := []string{"active", "pending", "inactive", "alumnus"}
	pickupStatuses := []string{"alone", "bus", "pickup"}
	ids := make([]int64, 0, count)
	for i := range count {
		student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Backfill", fmt.Sprintf("Student %d", i), "3a")
		_, err := db.ExecContext(ctx, `
			UPDATE users.students SET
				address_street = ?, address_city = ?, address_postal_code = ?, extra_info = ?,
				photo_path = ?, photo_consent_given_at = now() - make_interval(days => ?),
				agb_accepted_at = now() - make_interval(days => ?),
				data_processing_accepted_at = now() - make_interval(days => ?),
				email_contact_accepted_at = CASE WHEN ? THEN now() ELSE NULL END,
				group_id = ?, status = ?, enrolled_from = ?, enrolled_until = ?,
				supervisor_notes = ?, health_info = ?, pickup_status = ?,
				departure_days = ?::jsonb, allowed_departure_modes = ?::jsonb,
				departure_companion_note = ?, pickup_days = ?::jsonb, bus_days = ?::jsonb,
				updated_at = now() - make_interval(mins => ?)
			WHERE id = ?`,
			fmt.Sprintf("Street %d", i), fmt.Sprintf("City %d", i), fmt.Sprintf("%05d", 10000+i),
			fmt.Sprintf("extra %d", i), fmt.Sprintf("photos/%d.jpg", i), i+1, i+2, i+3, i%2 == 0,
			group.ID, statuses[i%len(statuses)], fmt.Sprintf("2026-01-%02d", i%28+1),
			fmt.Sprintf("2027-06-%02d", i%28+1), fmt.Sprintf("notes %d", i), fmt.Sprintf("health %d", i),
			pickupStatuses[i%len(pickupStatuses)], `{"mon": "pickup", "tue": "bus"}`,
			`{"mon": ["pickup", "bus"], "tue": ["bus"]}`, fmt.Sprintf("companion %d", i),
			`{"mon": true}`, `{"tue": true}`, i+1, student.ID)
		require.NoError(t, err)
		ids = append(ids, student.ID)
	}
	return ids
}

func studentOwnerCheckpoint(t *testing.T, report *StudentOwnerBackfillReport, tenantID int64) StudentOwnerBackfillCheckpoint {
	t.Helper()
	for _, tenant := range report.Tenants {
		if tenant.TenantID == tenantID {
			return tenant
		}
	}
	require.FailNowf(t, "missing checkpoint", "tenant %d has no checkpoint", tenantID)
	return StudentOwnerBackfillCheckpoint{}
}

func requireStudentOwnerTenantEqual(t *testing.T, db *testpkg.DB, report *StudentOwnerBackfillReport, tenantID int64) StudentOwnerBackfillCheckpoint {
	t.Helper()
	ctx := context.Background()
	cp := studentOwnerCheckpoint(t, report, tenantID)
	require.True(t, cp.Stable, "tenant %d must be stable: %+v", tenantID, cp)
	require.True(t, cp.Verified())
	require.Zero(t, cp.MismatchCount)
	require.Zero(t, cp.GuardianMismatchCount)
	require.Zero(t, cp.CareStateMismatchCount)
	require.Nil(t, cp.OldestUnmigratedAt)
	require.NotNil(t, cp.StableAt)
	var sourceCount, profileCount, membershipCount, careCount int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.students WHERE tenant_id = ?`, tenantID).Scan(ctx, &sourceCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`, tenantID).Scan(ctx, &profileCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ?`, tenantID).Scan(ctx, &membershipCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_care_profiles WHERE tenant_id = ?`, tenantID).Scan(ctx, &careCount))
	require.Equal(t, sourceCount, cp.SourceCount)
	require.Equal(t, sourceCount, profileCount, "profile identity must be preserved")
	require.Equal(t, sourceCount, membershipCount, "every profile carries exactly one membership")
	require.Equal(t, sourceCount, careCount, "every membership carries exactly one care profile")
	var orphans int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles p
		WHERE p.tenant_id = ? AND NOT EXISTS (SELECT 1 FROM users.students s WHERE s.id = p.id AND s.tenant_id = p.tenant_id)`, tenantID).Scan(ctx, &orphans))
	require.Zero(t, orphans)
	var rowMismatches int64
	require.NoError(t, db.NewRaw(studentOwnerMismatch, tenantID, tenantID).Scan(ctx, &rowMismatches, new(*time.Time)))
	require.Zero(t, rowMismatches, "targets must reproduce every source row")
	return cp
}

func studentOwnerSecondTenant(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	return tenantID
}

func studentSourceRows(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(string_agg(row_to_json(s)::text, ',' ORDER BY id), '') FROM users.students s WHERE tenant_id = ?`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func studentTargetRows(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(string_agg(to_jsonb(r)::text, ',' ORDER BY r.id), '') FROM (`+studentOwnerTargetProjection+`) r`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func TestStudentOwnerBackfillCopiesAndVerifiesPerTenant(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := studentOwnerSecondTenant(t, db)
	idsA := studentOwnerFixture(t, db, tenantA, 7)
	idsB := studentOwnerFixture(t, db, tenantB, 3)
	sourceBefore := studentSourceRows(t, db, tenantA)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	require.True(t, report.Stable(), "unstable tenants: %v", report.Unstable())
	cpA := requireStudentOwnerTenantEqual(t, db, report, tenantA)
	cpB := requireStudentOwnerTenantEqual(t, db, report, tenantB)
	require.EqualValues(t, 7, cpA.RowsCopied)
	require.EqualValues(t, 3, cpB.RowsCopied)
	require.EqualValues(t, 14, cpA.RowsScanned, "the stabilising re-read pass scans every row again")
	require.EqualValues(t, 7, cpA.RowsSkipped)
	require.EqualValues(t, 2, cpA.Pass)
	require.Zero(t, cpA.RowsRejected)
	require.Zero(t, cpA.BatchesRetried)
	require.NotEqual(t, cpA.SourceChecksum, cpB.SourceChecksum, "checksums are canonical per tenant")
	require.Equal(t, sourceBefore, studentSourceRows(t, db, tenantA), "the old table is never modified")

	// Identity is preserved across all three targets: every foreign key that
	// names a student today points at the same number after Cutover.
	for _, id := range append(idsA, idsB...) {
		var careID int64
		require.NoError(t, db.NewRaw(`SELECT c.membership_id FROM users.student_profiles p
			JOIN users.student_school_memberships m ON m.id = p.id AND m.student_profile_id = p.id
			JOIN users.student_care_profiles c ON c.membership_id = m.id WHERE p.id = ?`, id).Scan(ctx, &careID))
		require.Equal(t, id, careID)
	}
	var retired int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_school_memberships WHERE deleted_at IS NOT NULL`).Scan(ctx, &retired))
	require.Zero(t, retired, "users.students has no soft deletion, so no membership is retired")
	var nextProfileID, nextMembershipID, maxStudentID int64
	require.NoError(t, db.NewRaw(`SELECT nextval('users.student_profiles_id_seq')`).Scan(ctx, &nextProfileID))
	require.NoError(t, db.NewRaw(`SELECT nextval('users.student_school_memberships_id_seq')`).Scan(ctx, &nextMembershipID))
	require.NoError(t, db.NewRaw(`SELECT max(id) FROM users.students`).Scan(ctx, &maxStudentID))
	require.Greater(t, nextProfileID, maxStudentID, "profile sequence must not collide with preserved identities")
	require.Greater(t, nextMembershipID, maxStudentID, "membership sequence must not collide with preserved identities")
}

func TestStudentOwnerBackfillInterruptsAndResumesAtEveryBatchBoundary(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 5)
	errInterrupt := errors.New("interrupted after batch")
	var batches []studentOwnerBatch
	opts := StudentOwnerBackfillOptions{BatchSize: 2, afterBatch: func(id int64, batch studentOwnerBatch) error {
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
	var report *StudentOwnerBackfillReport
	for {
		runs++
		require.Less(t, runs, 20, "resume must converge")
		var err error
		report, err = RunStudentOwnerBackfill(ctx, db, opts)
		if err == nil {
			break
		}
		require.ErrorIs(t, err, errInterrupt)
		status, statusErr := StudentOwnerBackfillStatus(ctx, db)
		require.NoError(t, statusErr)
		cp := studentOwnerCheckpoint(t, status, tenantID)
		require.False(t, cp.Stable)
		require.Equal(t, batches[len(batches)-1].LastID, cp.HighWaterID, "the committed batch persists its high-water mark")
	}
	// Two passes over five rows in batches of two: (2,2,1) rows each, and
	// each pass is closed by an empty batch that is not interrupted.
	require.Equal(t, 7, runs)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
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

func TestStudentOwnerBackfillRerunsCompletedBatchesIdempotently(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 4)
	first, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	before := requireStudentOwnerTenantEqual(t, db, first, tenantID)
	targetBefore := studentTargetRows(t, db, tenantID)

	// Rewind the persisted high-water mark as if the last checkpoint write
	// had been lost, so every completed batch runs again.
	_, err = db.ExecContext(ctx, `UPDATE platform.storage_backfill_checkpoints SET high_water_id = 0, pass_completed = false, stable = false WHERE backfill = ? AND tenant_id = ?`, StudentOwnerBackfillName, tenantID)
	require.NoError(t, err)
	second, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	after := requireStudentOwnerTenantEqual(t, db, second, tenantID)
	require.Equal(t, before.RowsCopied, after.RowsCopied, "rerunning completed batches copies nothing")
	require.Equal(t, before.RowsSkipped+4, after.RowsSkipped)
	require.Equal(t, before.SourceChecksum, after.TargetChecksum)
	require.Equal(t, targetBefore, studentTargetRows(t, db, tenantID))
	require.Equal(t, before.Pass, after.Pass, "a rewound pass stabilises without starting another")
}

func TestStudentOwnerBackfillRetriesInjectedFailures(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherTenant := studentOwnerSecondTenant(t, db)
	studentOwnerFixture(t, db, tenantID, 3)
	studentOwnerFixture(t, db, otherTenant, 2)
	raise := func(ctx context.Context, tx testpkg.Tx, code string) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected %s' USING ERRCODE = '%s'; END $$`, code, code))
		return err
	}
	codes := map[int]string{1: "40P01", 2: "40001", 3: "55P03"}
	opts := StudentOwnerBackfillOptions{BatchSize: 10, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, attempt int) error {
		if id != tenantID {
			return nil
		}
		if code, ok := codes[attempt]; ok {
			return raise(ctx, tx, code)
		}
		return nil
	}}
	report, err := RunStudentOwnerBackfill(ctx, db, opts)
	require.NoError(t, err)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	// One batch per pass, two passes, each retried three times before success.
	require.EqualValues(t, 2, cp.BatchesCompleted)
	require.EqualValues(t, 6, cp.BatchesRetried)
	require.EqualValues(t, 2, cp.Deadlocks)
	require.EqualValues(t, 2, cp.SerializationFailures)
	require.EqualValues(t, 2, cp.LockTimeouts)
	require.EqualValues(t, 3, cp.RowsCopied, "rolled-back attempts must not double count")
	require.EqualValues(t, 6, cp.RowsScanned)

	_, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxAttempts: 2, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		return raise(ctx, tx, "40P01")
	}})
	require.ErrorContains(t, err, "gave up after 2 attempts")
	status, err := StudentOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Equal(t, cp.RowsCopied, studentOwnerCheckpoint(t, status, tenantID).RowsCopied)

	_, err = db.ExecContext(ctx, `UPDATE users.students SET supervisor_notes = 'changed while the other school fails' WHERE tenant_id = ?`, otherTenant)
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		return raise(ctx, tx, "22023")
	}})
	require.ErrorContains(t, err, "22023", "non-transient failures surface immediately")
	require.NotNil(t, report, "the other schools are still visited and reported")
	require.Equal(t, []int64{tenantID}, report.Unstable())
	other := requireStudentOwnerTenantEqual(t, db, report, otherTenant)
	require.EqualValues(t, 4, other.RowsCopied, "a failing school does not block the remaining schools")
}

func TestStudentOwnerTerminalFailuresPersistTelemetry(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"40P01", "40001", "55P03"} {
		t.Run(code, func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			db := setupStudentStorageBeforeCutover(t)
			tenantID := testpkg.Tenant(t)
			studentOwnerFixture(t, db, tenantID, 1)
			report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxAttempts: 2,
				injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
					if id != tenantID {
						return nil
					}
					_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected' USING ERRCODE = '%s'; END $$`, code))
					return err
				},
			})
			require.ErrorContains(t, err, "gave up after 2 attempts")
			cp := studentOwnerCheckpoint(t, report, tenantID)
			require.Zero(t, cp.HighWaterID)
			require.Zero(t, cp.RowsCopied)
			require.EqualValues(t, 1, cp.BatchesRetried)
			require.EqualValues(t, 2, cp.Deadlocks+cp.SerializationFailures+cp.LockTimeouts)
			report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
			require.NoError(t, err)
			resumed := requireStudentOwnerTenantEqual(t, db, report, tenantID)
			require.Equal(t, cp.BatchesRetried, resumed.BatchesRetried)
			require.Equal(t, cp.Deadlocks, resumed.Deadlocks)
			require.Equal(t, cp.SerializationFailures, resumed.SerializationFailures)
			require.Equal(t, cp.LockTimeouts, resumed.LockTimeouts)
		})
	}
}

func TestStudentOwnerBackfillRereadsChangedRowsAndRemovesOrphans(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 5)
	first, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	before := requireStudentOwnerTenantEqual(t, db, first, tenantID)

	_, err = db.ExecContext(ctx, `UPDATE users.students SET supervisor_notes = 'changed after backfill', health_info = NULL WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET school_class = '4c', status = 'inactive' WHERE id = ?`, ids[1])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.students WHERE id = ?`, ids[2])
	require.NoError(t, err)
	added := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Joined", "Later", "3a")

	second, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	after := requireStudentOwnerTenantEqual(t, db, second, tenantID)
	require.Equal(t, before.RowsCopied+3, after.RowsCopied, "changed and new rows are copied once")
	require.EqualValues(t, 1, after.RowsRemoved, "physically deleted source rows leave no orphan")
	require.EqualValues(t, 5, after.SourceCount)
	var notes string
	require.NoError(t, db.NewRaw(`SELECT supervisor_notes FROM users.student_care_profiles WHERE membership_id = ?`, ids[0]).Scan(ctx, &notes))
	require.Equal(t, "changed after backfill", notes)
	var schoolClass string
	require.NoError(t, db.NewRaw(`SELECT school_class FROM users.student_school_memberships WHERE id = ?`, ids[1]).Scan(ctx, &schoolClass))
	require.Equal(t, "4c", schoolClass)
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.student_profiles WHERE id = ?)`, ids[2]).Scan(ctx, &exists))
	require.False(t, exists, "the cascade removed the membership and care profile with it")
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.student_care_profiles WHERE membership_id = ?)`, added.ID).Scan(ctx, &exists))
	require.True(t, exists)
}

func TestStudentOwnerBackfillRejectsCrossTenantPerson(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := studentOwnerSecondTenant(t, db)
	idsA := studentOwnerFixture(t, db, tenantA, 2)
	studentOwnerFixture(t, db, tenantB, 1)
	foreign := testpkg.CreateTestPersonForTenant(t, db, tenantB, "Foreign", "Person")
	// users.students still carries the single-column person FK, so a superuser
	// write can point a row at another school's person. The People target
	// references users.persons(tenant_id, id) and must not take the row.
	_, err := db.ExecContext(ctx, `UPDATE users.students SET person_id = ? WHERE id = ?`, foreign.ID, idsA[0])
	require.NoError(t, err)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 2})
	require.NoError(t, err)
	require.False(t, report.Stable())
	require.Equal(t, []int64{tenantA}, report.Unstable())
	requireStudentOwnerTenantEqual(t, db, report, tenantB)
	cp := studentOwnerCheckpoint(t, report, tenantA)
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
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.student_profiles WHERE id = ?)`, idsA[0]).Scan(ctx, &copied))
	require.False(t, copied)

	own := testpkg.CreateTestPersonForTenant(t, db, tenantA, "Own", "Person")
	_, err = db.ExecContext(ctx, `UPDATE users.students SET person_id = ? WHERE id = ?`, own.ID, idsA[0])
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, report, tenantA)
}

// A child whose enrollment is deleted and recreated keeps their person row, so
// the new student ID collides with the profile of the deleted one on the
// per-person unique key. The pass rewinds, removes the orphan and converges.
func TestStudentOwnerBackfillRestartsPassWhenChildRejoinsMidPass(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherTenant := studentOwnerSecondTenant(t, db)
	ids := studentOwnerFixture(t, db, tenantID, 2)
	studentOwnerFixture(t, db, otherTenant, 1)
	first, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, first, tenantID)

	var personID int64
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.students WHERE id = ?`, ids[0]).Scan(ctx, &personID))
	_, err = db.ExecContext(ctx, `DELETE FROM users.students WHERE id = ?`, ids[0])
	require.NoError(t, err)
	var replacementID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.students (tenant_id, person_id, school_class)
		VALUES (?, ?, '1a') RETURNING id`, tenantID, personID).Scan(ctx, &replacementID))
	require.Greater(t, replacementID, ids[1])
	sourceAfterRejoin := studentSourceRows(t, db, tenantID)
	otherSource := studentSourceRows(t, db, otherTenant)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 1})
	require.NoError(t, err)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 2, cp.SourceCount)
	require.EqualValues(t, 1, cp.RowsRemoved)
	require.EqualValues(t, 1, cp.BatchesRetried)
	var profileOwner int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_profiles WHERE tenant_id = ? AND person_id = ?`, tenantID, personID).Scan(ctx, &profileOwner))
	require.Equal(t, replacementID, profileOwner)
	require.Equal(t, sourceAfterRejoin, studentSourceRows(t, db, tenantID), "recovery must not modify authoritative source rows")
	require.Equal(t, otherSource, studentSourceRows(t, db, otherTenant))
	requireStudentOwnerTenantEqual(t, db, report, otherTenant)
}

func TestStudentOwnerBackfillReportsUnreconciledGuardianValues(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 2)

	// The first child keeps every legacy value on a linked guardian; the
	// second child's legacy e-mail belongs to nobody.
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Erika", "Musterfrau", "erika")
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, ids[0], guardian.ID, "primary_guardian")
	_, err := db.ExecContext(ctx, `INSERT INTO users.guardian_phone_numbers (tenant_id, guardian_profile_id, phone_number, phone_type, is_primary, priority)
		VALUES (?, ?, '+49 170 1234567', 'mobile', true, 1)`, tenantID, guardian.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET guardian_name = 'Erika Musterfrau', guardian_contact = ?,
		guardian_email = ?, guardian_phone = '+49 170 1234567' WHERE id = ?`, *guardian.Email, *guardian.Email, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET guardian_email = 'nobody@example.test' WHERE id = ?`, ids[1])
	require.NoError(t, err)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 1})
	require.NoError(t, err)
	cp := studentOwnerCheckpoint(t, report, tenantID)
	require.False(t, cp.Stable)
	require.False(t, cp.Verified())
	require.Zero(t, cp.MismatchCount, "the copied columns still match column for column")
	require.EqualValues(t, 1, cp.GuardianMismatchCount,
		"the reconciled child is clean; only the unlinked e-mail is reported")
	require.NotNil(t, cp.OldestUnmigratedAt)

	// The same number written the national way is reported rather than assumed
	// equal: normalization stops at digits, so the correction stays a decision.
	_, err = db.ExecContext(ctx, `UPDATE users.students SET guardian_phone = '0170 1234567' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, studentOwnerCheckpoint(t, report, tenantID).GuardianMismatchCount)

	// Linking the values the guardians actually carry clears the report; the
	// legacy columns are never copied into a student-owned table.
	_, err = db.ExecContext(ctx, `UPDATE users.students SET guardian_phone = '+49 170 1234567' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET guardian_email = NULL WHERE id = ?`, ids[1])
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, report, tenantID)
	var targetColumns int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name IN ('student_profiles', 'student_school_memberships', 'student_care_profiles')
		AND column_name LIKE 'guardian%'`).Scan(ctx, &targetColumns))
	require.Zero(t, targetColumns)
}

func TestStudentOwnerBackfillReportsStaleAbsenceFlags(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 3)

	// One child is sick with the status day the authority expects, one is
	// excused by a class trip, and one carries only the legacy flag.
	for _, row := range []struct {
		student int64
		status  string
	}{{ids[0], "sick"}, {ids[1], "class_trip"}} {
		_, err := db.ExecContext(ctx, `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
			VALUES (?, ?, (now() AT TIME ZONE 'Europe/Berlin')::date, ?, now(), 'manual')`, tenantID, row.student, row.status)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, `UPDATE users.students SET sick = true, sick_since = now() WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET excused = true, excused_since = now() WHERE id = ?`, ids[1])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET sick = true, sick_since = now() WHERE id = ?`, ids[2])
	require.NoError(t, err)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 1})
	require.NoError(t, err)
	cp := studentOwnerCheckpoint(t, report, tenantID)
	require.False(t, cp.Stable)
	require.Zero(t, cp.MismatchCount, "the flags are not copied, so the mapped columns still match")
	require.EqualValues(t, 1, cp.CareStateMismatchCount,
		"only the flag without an equivalent open status day is reported")
	require.Nil(t, cp.OldestUnmigratedAt, "preserved flags are not unmigrated owner data")
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	require.True(t, studentOwnerCheckpoint(t, report, tenantID).Stable,
		"the status-day diagnostic must not block a stable copy")

	// A cleared status day no longer covers the flag.
	_, err = db.ExecContext(ctx, `UPDATE active.student_status_days SET cleared_at = now() WHERE student_id = ?`, ids[0])
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, studentOwnerCheckpoint(t, report, tenantID).CareStateMismatchCount)

	// Draining the flags the way the end-of-day archiver does clears it.
	_, err = db.ExecContext(ctx, `UPDATE users.students SET sick = false, excused = false WHERE tenant_id = ?`, tenantID)
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, report, tenantID)
}

func TestStudentOwnerVerificationUsesOneUTCSnapshot(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 1)
	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := studentOwnerCheckpoint(t, report, tenantID)
	previousChecksum := cp.SourceChecksum
	run := studentOwnerTenantRun{db: db, tenantID: tenantID, opts: StudentOwnerBackfillOptions{
		afterSourceVerification: func(ctx context.Context, tx testpkg.Tx) error {
			var zone string
			if err := tx.NewRaw(`SHOW TIME ZONE`).Scan(ctx, &zone); err != nil {
				return err
			}
			require.Equal(t, "UTC", zone)
			_, err := db.ExecContext(ctx, `UPDATE users.students SET extra_info = 'changed between verification queries' WHERE id = ?`, ids[0])
			return err
		},
	}.withDefaults()}
	require.NoError(t, run.verify(ctx, &cp))
	require.True(t, cp.Verified(), "source and target evidence must describe the same pre-edit snapshot")
	require.Equal(t, previousChecksum, cp.SourceChecksum)
	require.NotEmpty(t, cp.VerificationSnapshot)
	var mismatches int64
	require.NoError(t, db.NewRaw(studentOwnerMismatch, tenantID, tenantID).Scan(ctx, &mismatches, new(*time.Time)))
	require.EqualValues(t, 1, mismatches, "the real source edit committed outside the verification snapshot")
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	updated := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.NotEqual(t, previousChecksum, updated.SourceChecksum)
}

func TestStudentOwnerRunExcludesOtherWriters(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 1)
	checked := false
	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{afterBatch: func(id int64, _ studentOwnerBatch) error {
		if id != tenantID || checked {
			return nil
		}
		checked = true
		_, runErr := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
		require.ErrorContains(t, runErr, "another run, reset or rollback is active")
		require.ErrorContains(t, ResetStudentOwnerBackfill(ctx, db), "another run, reset or rollback is active")
		require.ErrorContains(t, studentOwnerBackfillDown(ctx, db), "another run, reset or rollback is active")
		return nil
	}})
	require.NoError(t, err)
	require.True(t, checked)
	requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.NoError(t, ResetStudentOwnerBackfill(ctx, db), "lock must be released after the completed run")
}

func TestStudentOwnerCancellationFlushesFailedAttempt(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 1)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	defer cancel()
	_, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		_, err := tx.ExecContext(ctx, `DO $$ BEGIN RAISE EXCEPTION 'injected cancellation' USING ERRCODE = '40P01'; END $$`)
		cancel()
		return err
	}})
	require.ErrorIs(t, err, context.Canceled)
	report, err := StudentOwnerBackfillStatus(t.Context(), db)
	require.NoError(t, err)
	cp := studentOwnerCheckpoint(t, report, tenantID)
	require.EqualValues(t, 1, cp.Deadlocks)
	require.Zero(t, cp.BatchesRetried, "cancellation prevented another attempt from starting")
	require.Zero(t, cp.HighWaterID)
	require.Zero(t, cp.RowsCopied)
	require.NoError(t, ResetStudentOwnerBackfill(t.Context(), db), "cancelled run must release exclusion lock")
}

func TestStudentOwnerMeasuresSuccessfulLockWait(t *testing.T) {
	t.Parallel()
	db := setupIsolatedStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 1)
	_, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET extra_info = 'requires target update' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT id FROM users.student_profiles WHERE id = ? FOR UPDATE`, ids[0])
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, runErr := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{}); done <- runErr }()
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
	report, err := StudentOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.Greater(t, cp.LockWaitMs, float64(50))
	require.Zero(t, cp.LockTimeouts, "a real lock wait need not time out")
}

func TestStudentOwnerBackfillTenantIsolation(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := studentOwnerSecondTenant(t, db)
	studentOwnerFixture(t, db, tenantA, 2)
	studentOwnerFixture(t, db, tenantB, 3)
	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	require.True(t, report.Stable())
	for tenantID, expected := range map[int64]int64{tenantA: 2, tenantB: 3} {
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, strconv.FormatInt(tenantID, 10)); err != nil {
				return err
			}
			for _, table := range []string{"users.student_profiles", "users.student_school_memberships", "users.student_care_profiles"} {
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

func TestStudentOwnerBackfillIndexesAndQueryPlans(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 3)
	_, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	var invalidIndexes int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid
		WHERE c.oid IN ('users.student_profiles'::regclass, 'users.student_school_memberships'::regclass,
			'users.student_care_profiles'::regclass, 'users.students'::regclass)
		  AND NOT (i.indisvalid AND i.indisready)`).Scan(ctx, &invalidIndexes))
	require.Zero(t, invalidIndexes)
	var constraintViolations int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles p
		LEFT JOIN users.persons s ON s.id = p.person_id AND s.tenant_id = p.tenant_id WHERE s.id IS NULL`).Scan(ctx, &constraintViolations))
	require.Zero(t, constraintViolations)

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
			"batch":           {studentOwnerCopyBatch, []any{tenantID, int64(0), 100, int64(0)}},
			"source checksum": {studentOwnerSourceChecksum, []any{tenantID}},
			"target checksum": {studentOwnerTargetChecksum, []any{tenantID}},
			"mismatch":        {studentOwnerMismatch, []any{tenantID, tenantID}},
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

func TestStudentOwnerBackfillResetAndRollback(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 3)
	sourceBefore := studentSourceRows(t, db, tenantID)
	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, report, tenantID)

	require.NoError(t, ResetStudentOwnerBackfill(ctx, db))
	requireStudentOwnerTargetsEmpty(t, db)
	status, err := StudentOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Empty(t, status.Tenants)
	require.Contains(t, status.MissingTenants, tenantID)
	require.False(t, status.Stable(), "unvisited schools block Cutover")
	require.Equal(t, sourceBefore, studentSourceRows(t, db, tenantID))

	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 3, cp.RowsCopied, "reset restarts from zero")

	// The checkpoint table is shared with the other storage backfills: rolling
	// back this one keeps their progress and drops the table only once empty.
	_, err = db.ExecContext(ctx, `INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id) VALUES ('other-backfill', ?)`, tenantID)
	require.NoError(t, err)
	require.NoError(t, studentOwnerBackfillDown(ctx, db))
	requireStudentOwnerTargetsEmpty(t, db)
	var remaining []string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT backfill FROM platform.storage_backfill_checkpoints`).Scan(ctx, &remaining))
	require.ElementsMatch(t, []string{"other-backfill", StaffOwnerBackfillName, GuardianOwnerBackfillName}, remaining,
		"this rollback removes exactly its own rows and leaves every other backfill's progress")
	_, err = db.ExecContext(ctx, `DELETE FROM platform.storage_backfill_checkpoints WHERE backfill IN (?, ?, ?)`,
		"other-backfill", StaffOwnerBackfillName, GuardianOwnerBackfillName)
	require.NoError(t, err)
	require.NoError(t, studentOwnerBackfillDown(ctx, db))
	var checkpointsAbsent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('platform.storage_backfill_checkpoints') IS NULL`).Scan(ctx, &checkpointsAbsent))
	require.True(t, checkpointsAbsent)
	require.Equal(t, sourceBefore, studentSourceRows(t, db, tenantID))
	require.NoError(t, studentOwnerStorageExpandDown(ctx, db), "Expand rollback follows once the targets are empty")
	require.NoError(t, studentOwnerStorageExpandUp(ctx, db))
	require.NoError(t, studentOwnerBackfillUp(ctx, db), "the migration recreates the dropped checkpoint table with its own columns")
	status, err = StudentOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, status, tenantID)
	require.NoError(t, studentOwnerBackfillUp(ctx, db), "the migration is idempotent for resumed deployments")

	// After Cutover users.students becomes a compatibility view over the
	// targets; target-only truncation would then destroy authoritative rows.
	_, err = db.ExecContext(ctx, `ALTER TABLE users.students RENAME TO students_legacy; CREATE VIEW users.students AS SELECT * FROM users.students_legacy`)
	require.NoError(t, err)
	require.ErrorContains(t, ResetStudentOwnerBackfill(ctx, db), "not a base table")
	require.ErrorContains(t, studentOwnerBackfillDown(ctx, db), "not a base table")
	_, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.ErrorContains(t, err, "not a base table")
	var profileCount int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`, tenantID).Scan(ctx, &profileCount))
	require.EqualValues(t, 3, profileCount)
	_, err = db.ExecContext(ctx, `DROP VIEW users.students; ALTER TABLE users.students_legacy RENAME TO students`)
	require.NoError(t, err)
}

func requireStudentOwnerTargetsEmpty(t *testing.T, db *testpkg.DB) {
	t.Helper()
	for _, table := range []string{"users.student_profiles", "users.student_school_memberships", "users.student_care_profiles"} {
		var count int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+table).Scan(context.Background(), &count))
		require.Zero(t, count, "%s must be empty", table)
	}
}

// Nothing stops both legacy flags being raised at once, and the effective
// absence read is an if/else-if chain in which sick wins. A child who stays
// sick after the split therefore loses nothing when their excused flag goes,
// and reporting them would cost an operator a flag cleared purely to satisfy
// the verifier.
func TestStudentOwnerBackfillAcceptsInertExcusedFlagUnderSickness(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 1)
	_, err := db.ExecContext(ctx, `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, (now() AT TIME ZONE 'Europe/Berlin')::date, 'sick', now(), 'manual')`, tenantID, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET sick = true, sick_since = now(),
		excused = true, excused_since = now() WHERE id = ?`, ids[0])
	require.NoError(t, err)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.Zero(t, cp.CareStateMismatchCount,
		"the child is sick on both sides of the split, so the excused flag carries no state to lose")

	// Drop the sickness and the excused flag becomes load-bearing again: it is
	// now the only thing marking the child absent, and it has no status day.
	_, err = db.ExecContext(ctx, `UPDATE users.students SET sick = false WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE active.student_status_days SET cleared_at = now() WHERE student_id = ?`, ids[0])
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 1})
	require.NoError(t, err)
	require.EqualValues(t, 1, studentOwnerCheckpoint(t, report, tenantID).CareStateMismatchCount)
}

// The copy runs as superuser and bypasses every policy it writes through, so
// the run proves the tenant boundary of its own targets on each pass instead of
// trusting that Expand's policies are still there.
func TestStudentOwnerBackfillVerifiesTenantPolicies(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := studentOwnerSecondTenant(t, db)
	studentOwnerFixture(t, db, tenantA, 2)
	studentOwnerFixture(t, db, tenantB, 3)
	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	cpA := requireStudentOwnerTenantEqual(t, db, report, tenantA)
	cpB := requireStudentOwnerTenantEqual(t, db, report, tenantB)
	require.EqualValues(t, 6, cpA.RowsVisibleToTenant, "one profile, membership and care profile per student")
	require.EqualValues(t, 9, cpB.RowsVisibleToTenant)

	// A policy dropped after Expand leaves the target default-deny under FORCE
	// row-level security: the tenant role stops seeing its own rows.
	_, err = db.ExecContext(ctx, `DROP POLICY tenant_isolation_users_student_care_profiles ON users.student_care_profiles`)
	require.NoError(t, err)
	_, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.ErrorContains(t, err, "row-level security leaked")
	require.ErrorContains(t, err, "own rows")
	status, err := StudentOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Equal(t, []int64{tenantA, tenantB}, status.Unstable(), "an unproven boundary blocks Cutover for every school")

	// A policy widened to every school is the opposite failure and must not
	// pass as "the role can read its rows".
	_, err = db.ExecContext(ctx, `CREATE POLICY tenant_isolation_users_student_care_profiles
		ON users.student_care_profiles FOR ALL USING (true) WITH CHECK (true)`)
	require.NoError(t, err)
	_, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.ErrorContains(t, err, "foreign rows")
}

// Two children swapping their person rows is the conflict no orphan sweep can
// clear: both source rows are alive, so the rewind must release the stale
// per-person claim itself or the pass restarts until it gives up.
func TestStudentOwnerBackfillRecoversFromPersonReassignment(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 2)
	first, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, first, tenantID)

	var personA, personB int64
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.students WHERE id = ?`, ids[0]).Scan(ctx, &personA))
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.students WHERE id = ?`, ids[1]).Scan(ctx, &personB))
	// The unique key forbids an intermediate duplicate, so the swap parks one
	// child on a third person first — exactly how a real merge would do it.
	parked := testpkg.CreateTestPersonForTenant(t, db, tenantID, "Parked", "Person")
	_, err = db.ExecContext(ctx, `UPDATE users.students SET person_id = ? WHERE id = ?`, parked.ID, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET person_id = ? WHERE id = ?`, personA, ids[1])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET person_id = ? WHERE id = ?`, personB, ids[0])
	require.NoError(t, err)
	sourceAfterSwap := studentSourceRows(t, db, tenantID)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{BatchSize: 1})
	require.NoError(t, err)
	cp := requireStudentOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 2, cp.SourceCount)
	require.Positive(t, cp.BatchesRetried, "the conflict must be observed, not avoided by luck")
	var swappedA, swappedB int64
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.student_profiles WHERE id = ?`, ids[0]).Scan(ctx, &swappedA))
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.student_profiles WHERE id = ?`, ids[1]).Scan(ctx, &swappedB))
	require.Equal(t, personB, swappedA)
	require.Equal(t, personA, swappedB)
	require.Equal(t, sourceAfterSwap, studentSourceRows(t, db, tenantID), "recovery must not modify authoritative source rows")
}

// The legacy flags carry no date, so an open status day on an earlier date does
// not reproduce them: the flag makes the child absent today and tomorrow, the
// dated row does not. Keep the diagnostic, but do not block the lossless
// migration of the flag and timestamp into Care Plan.
func TestStudentOwnerBackfillReportsAbsenceFlagWithOnlyAnEarlierStatusDay(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 1)
	_, err := db.ExecContext(ctx, `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, (now() AT TIME ZONE 'Europe/Berlin')::date - 1, 'sick', now() - interval '1 day', 'manual')`, tenantID, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET sick = true, sick_since = now() - interval '1 day' WHERE id = ?`, ids[0])
	require.NoError(t, err)

	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{MaxPasses: 1})
	require.NoError(t, err)
	cp := studentOwnerCheckpoint(t, report, tenantID)
	require.EqualValues(t, 1, cp.CareStateMismatchCount,
		"yesterday's open day does not cover a flag that still makes the child absent today")
	require.False(t, cp.Stable)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	cp = studentOwnerCheckpoint(t, report, tenantID)
	require.True(t, cp.Stable, "a date rollover does not imply storage loss")
	require.EqualValues(t, 1, cp.CareStateMismatchCount)

	// The end-of-day archiver writes the day already cleared and clears the
	// flag; a child drained that way is not a candidate at all.
	_, err = db.ExecContext(ctx, `UPDATE active.student_status_days SET cleared_at = now() WHERE student_id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.students SET sick = false WHERE id = ?`, ids[0])
	require.NoError(t, err)
	report, err = RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStudentOwnerTenantEqual(t, db, report, tenantID)
}
