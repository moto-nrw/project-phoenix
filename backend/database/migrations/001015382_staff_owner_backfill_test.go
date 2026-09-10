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

func TestStaffOwnerVerificationUsesOneUTCSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := staffOwnerFixture(t, db, tenantID, 1)
	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := staffOwnerCheckpoint(t, report, tenantID)
	previousChecksum := cp.SourceChecksum
	run := staffOwnerTenantRun{db: db, tenantID: tenantID, opts: StaffOwnerBackfillOptions{
		afterSourceVerification: func(ctx context.Context, tx testpkg.Tx) error {
			var zone string
			if err := tx.NewRaw(`SHOW TIME ZONE`).Scan(ctx, &zone); err != nil {
				return err
			}
			require.Equal(t, "UTC", zone)
			_, err := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'changed between verification queries' WHERE id = ?`, ids[0])
			return err
		},
	}.withDefaults()}
	require.NoError(t, run.verify(ctx, &cp))
	require.True(t, cp.Verified(), "source and target evidence must describe the same pre-edit snapshot")
	require.Equal(t, previousChecksum, cp.SourceChecksum)
	require.NotEmpty(t, cp.VerificationSnapshot)
	var mismatches int64
	require.NoError(t, db.NewRaw(staffOwnerMismatch, tenantID, tenantID).Scan(ctx, &mismatches, new(*time.Time)))
	require.EqualValues(t, 1, mismatches, "the real source edit committed outside the verification snapshot")
	report, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	updated := requireStaffOwnerTenantEqual(t, db, report, tenantID)
	require.NotEqual(t, previousChecksum, updated.SourceChecksum)
}

func TestStaffOwnerTerminalFailuresPersistTelemetry(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"40P01", "40001", "55P03"} {
		t.Run(code, func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			db := testpkg.SetupTestDB(t)
			tenantID := testpkg.Tenant(t)
			staffOwnerFixture(t, db, tenantID, 1)
			report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{MaxAttempts: 2,
				injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
					if id != tenantID {
						return nil
					}
					_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected' USING ERRCODE = '%s'; END $$`, code))
					return err
				},
			})
			require.ErrorContains(t, err, "gave up after 2 attempts")
			cp := staffOwnerCheckpoint(t, report, tenantID)
			require.Zero(t, cp.HighWaterID)
			require.Zero(t, cp.RowsCopied)
			require.EqualValues(t, 1, cp.BatchesRetried)
			require.EqualValues(t, 2, cp.Deadlocks+cp.SerializationFailures+cp.LockTimeouts)
			switch code {
			case "40P01":
				require.EqualValues(t, 2, cp.Deadlocks)
			case "40001":
				require.EqualValues(t, 2, cp.SerializationFailures)
			case "55P03":
				require.EqualValues(t, 2, cp.LockTimeouts)
			}
			report, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
			require.NoError(t, err)
			resumed := requireStaffOwnerTenantEqual(t, db, report, tenantID)
			require.Equal(t, cp.BatchesRetried, resumed.BatchesRetried)
			require.Equal(t, cp.Deadlocks, resumed.Deadlocks)
			require.Equal(t, cp.SerializationFailures, resumed.SerializationFailures)
			require.Equal(t, cp.LockTimeouts, resumed.LockTimeouts)
		})
	}
}

func TestStaffOwnerRunExcludesOtherWriters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	staffOwnerFixture(t, db, tenantID, 1)
	checked := false
	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{afterBatch: func(id int64, _ staffOwnerBatch) error {
		if id != tenantID || checked {
			return nil
		}
		checked = true
		_, runErr := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
		require.ErrorContains(t, runErr, "another run, reset or rollback is active")
		require.ErrorContains(t, ResetStaffOwnerBackfill(ctx, db), "another run, reset or rollback is active")
		require.ErrorContains(t, staffOwnerBackfillDown(ctx, db), "another run, reset or rollback is active")
		return nil
	}})
	require.NoError(t, err)
	require.True(t, checked)
	requireStaffOwnerTenantEqual(t, db, report, tenantID)
	require.NoError(t, ResetStaffOwnerBackfill(ctx, db), "lock must be released after the completed run")
}

func TestStaffOwnerCancellationFlushesFailedAttempt(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	staffOwnerFixture(t, db, tenantID, 1)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	defer cancel()
	_, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		_, err := tx.ExecContext(ctx, `DO $$ BEGIN RAISE EXCEPTION 'injected cancellation' USING ERRCODE = '40P01'; END $$`)
		cancel()
		return err
	}})
	require.ErrorIs(t, err, context.Canceled)
	report, err := StaffOwnerBackfillStatus(t.Context(), db)
	require.NoError(t, err)
	cp := staffOwnerCheckpoint(t, report, tenantID)
	require.EqualValues(t, 1, cp.Deadlocks)
	require.Zero(t, cp.BatchesRetried, "cancellation prevented another attempt from starting")
	require.Zero(t, cp.HighWaterID)
	require.Zero(t, cp.RowsCopied)
	require.NoError(t, ResetStaffOwnerBackfill(t.Context(), db), "cancelled run must release exclusion lock")
}

func TestStaffOwnerMeasuresSuccessfulLockWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := staffOwnerFixture(t, db, tenantID, 1)
	_, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'requires target update' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT membership_id FROM users.staff_employment_profiles WHERE membership_id = ? FOR UPDATE`, ids[0])
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, runErr := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{}); done <- runErr }()
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
	report, err := StaffOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	cp := requireStaffOwnerTenantEqual(t, db, report, tenantID)
	require.Greater(t, cp.LockWaitMs, float64(50))
	require.Zero(t, cp.LockTimeouts, "a real lock wait need not time out")
}

// staffOwnerFixture populates one tenant with staff rows covering every
// copied column, including a soft-deleted row and a tenant-owned work-time
// model. It returns the staff IDs in insertion order.
func staffOwnerFixture(t *testing.T, db *testpkg.DB, tenantID int64, count int) []int64 {
	t.Helper()
	ctx := context.Background()
	var modelID int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date)
		VALUES (?, 'Backfill fixture', '2026-09-01') RETURNING id`, tenantID).Scan(ctx, &modelID))
	ids := make([]int64, 0, count)
	employmentTypes := []string{"full_time", "part_time", "minijob"}
	for i := range count {
		staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Backfill", fmt.Sprintf("Staff %d", i))
		_, err := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = ?, employment_type = ?, personnel_number = ?,
			rotation_anchor_date = ?, birthday_display_opt_out = ?, work_time_model_id = ?, updated_at = now() - make_interval(mins => ?)
			WHERE id = ?`,
			fmt.Sprintf("notes %d", i), employmentTypes[i%len(employmentTypes)], fmt.Sprintf("PN-%d-%d", tenantID, i),
			fmt.Sprintf("2026-01-%02d", i%28+1), i%2 == 0, modelID, i+1, staff.ID)
		require.NoError(t, err)
		ids = append(ids, staff.ID)
	}
	if count > 1 {
		_, err := db.ExecContext(ctx, `UPDATE users.staff SET deleted_at = now() WHERE id = ?`, ids[count-1])
		require.NoError(t, err)
	}
	return ids
}

func staffOwnerCheckpoint(t *testing.T, report *StaffOwnerBackfillReport, tenantID int64) StaffOwnerBackfillCheckpoint {
	t.Helper()
	for _, tenant := range report.Tenants {
		if tenant.TenantID == tenantID {
			return tenant
		}
	}
	require.FailNowf(t, "missing checkpoint", "tenant %d has no checkpoint", tenantID)
	return StaffOwnerBackfillCheckpoint{}
}

func requireStaffOwnerTenantEqual(t *testing.T, db *testpkg.DB, report *StaffOwnerBackfillReport, tenantID int64) StaffOwnerBackfillCheckpoint {
	t.Helper()
	ctx := context.Background()
	cp := staffOwnerCheckpoint(t, report, tenantID)
	require.True(t, cp.Stable, "tenant %d must be stable: %+v", tenantID, cp)
	require.True(t, cp.Verified())
	require.Zero(t, cp.MismatchCount)
	require.Nil(t, cp.OldestUnmigratedAt)
	require.NotNil(t, cp.StableAt)
	var sourceCount, membershipCount, profileCount, orphanProfiles, orphanMemberships int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff WHERE tenant_id = ?`, tenantID).Scan(ctx, &sourceCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships WHERE tenant_id = ?`, tenantID).Scan(ctx, &membershipCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_employment_profiles WHERE tenant_id = ?`, tenantID).Scan(ctx, &profileCount))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_employment_profiles p
		WHERE p.tenant_id = ? AND NOT EXISTS (SELECT 1 FROM users.staff_school_memberships m WHERE m.id = p.membership_id)`, tenantID).Scan(ctx, &orphanProfiles))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships m
		WHERE m.tenant_id = ? AND NOT EXISTS (SELECT 1 FROM users.staff s WHERE s.id = m.id AND s.tenant_id = m.tenant_id)`, tenantID).Scan(ctx, &orphanMemberships))
	require.Equal(t, sourceCount, cp.SourceCount)
	require.Equal(t, sourceCount, membershipCount, "membership identity must be preserved")
	require.Equal(t, sourceCount, profileCount, "every membership carries exactly one profile")
	require.Zero(t, orphanProfiles)
	require.Zero(t, orphanMemberships)
	var rowMismatches int64
	require.NoError(t, db.NewRaw(staffOwnerMismatch, tenantID, tenantID).Scan(ctx, &rowMismatches, new(*time.Time)))
	require.Zero(t, rowMismatches, "targets must reproduce every source row")
	return cp
}

func staffOwnerSecondTenant(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	return tenantID
}

func TestStaffOwnerBackfillCopiesAndVerifiesPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := staffOwnerSecondTenant(t, db)
	idsA := staffOwnerFixture(t, db, tenantA, 7)
	idsB := staffOwnerFixture(t, db, tenantB, 3)
	sourceBefore := staffSourceRows(t, db, tenantA)

	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	require.True(t, report.Stable(), "unstable tenants: %v", report.Unstable())
	cpA := requireStaffOwnerTenantEqual(t, db, report, tenantA)
	cpB := requireStaffOwnerTenantEqual(t, db, report, tenantB)
	require.EqualValues(t, 7, cpA.RowsCopied)
	require.EqualValues(t, 3, cpB.RowsCopied)
	require.EqualValues(t, 14, cpA.RowsScanned, "the stabilising re-read pass scans every row again")
	require.EqualValues(t, 7, cpA.RowsSkipped)
	require.EqualValues(t, 2, cpA.Pass)
	require.Zero(t, cpA.RowsRejected)
	require.Zero(t, cpA.BatchesRetried)
	require.NotEqual(t, cpA.SourceChecksum, cpB.SourceChecksum, "checksums are canonical per tenant")
	require.Equal(t, sourceBefore, staffSourceRows(t, db, tenantA), "the old table is never modified")

	for _, id := range append(idsA, idsB...) {
		var membershipID int64
		require.NoError(t, db.NewRaw(`SELECT m.id FROM users.staff_school_memberships m
			JOIN users.staff_employment_profiles p ON p.membership_id = m.id WHERE m.id = ?`, id).Scan(ctx, &membershipID))
		require.Equal(t, id, membershipID)
	}
	var deletedCopied bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, idsA[len(idsA)-1]).Scan(ctx, &deletedCopied))
	require.True(t, deletedCopied, "soft deletion is membership lifecycle and must be copied")
	var nextMembershipID, maxStaffID int64
	require.NoError(t, db.NewRaw(`SELECT nextval('users.staff_school_memberships_id_seq')`).Scan(ctx, &nextMembershipID))
	require.NoError(t, db.NewRaw(`SELECT max(id) FROM users.staff`).Scan(ctx, &maxStaffID))
	require.Greater(t, nextMembershipID, maxStaffID, "membership sequence must not collide with preserved identities")
}

func staffSourceRows(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(string_agg(row_to_json(s)::text, ',' ORDER BY id), '') FROM users.staff s WHERE tenant_id = ?`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func TestStaffOwnerBackfillInterruptsAndResumesAtEveryBatchBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	staffOwnerFixture(t, db, tenantID, 5)
	errInterrupt := errors.New("interrupted after batch")
	var batches []staffOwnerBatch
	opts := StaffOwnerBackfillOptions{BatchSize: 2, afterBatch: func(id int64, batch staffOwnerBatch) error {
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
	var report *StaffOwnerBackfillReport
	for {
		runs++
		require.Less(t, runs, 20, "resume must converge")
		var err error
		report, err = RunStaffOwnerBackfill(ctx, db, opts)
		if err == nil {
			break
		}
		require.ErrorIs(t, err, errInterrupt)
		status, statusErr := StaffOwnerBackfillStatus(ctx, db)
		require.NoError(t, statusErr)
		cp := staffOwnerCheckpoint(t, status, tenantID)
		require.False(t, cp.Stable)
		require.Equal(t, batches[len(batches)-1].LastID, cp.HighWaterID, "the committed batch persists its high-water mark")
	}
	// Two passes over five rows in batches of two: (2,2,1) rows each, and
	// each pass is closed by an empty batch that is not interrupted.
	require.Equal(t, 7, runs)
	cp := requireStaffOwnerTenantEqual(t, db, report, tenantID)
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

func TestStaffOwnerBackfillRerunsCompletedBatchesIdempotently(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	staffOwnerFixture(t, db, tenantID, 4)
	first, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	before := requireStaffOwnerTenantEqual(t, db, first, tenantID)
	targetBefore := staffTargetRows(t, db, tenantID)

	// Rewind the persisted high-water mark as if the last checkpoint write
	// had been lost, so every completed batch runs again.
	_, err = db.ExecContext(ctx, `UPDATE platform.storage_backfill_checkpoints SET high_water_id = 0, pass_completed = false, stable = false WHERE backfill = ? AND tenant_id = ?`, StaffOwnerBackfillName, tenantID)
	require.NoError(t, err)
	second, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{BatchSize: 3})
	require.NoError(t, err)
	after := requireStaffOwnerTenantEqual(t, db, second, tenantID)
	require.Equal(t, before.RowsCopied, after.RowsCopied, "rerunning completed batches copies nothing")
	require.Equal(t, before.RowsSkipped+4, after.RowsSkipped)
	require.Equal(t, before.SourceChecksum, after.TargetChecksum)
	require.Equal(t, targetBefore, staffTargetRows(t, db, tenantID))
	require.Equal(t, before.Pass, after.Pass, "a rewound pass stabilises without starting another")
}

func staffTargetRows(t *testing.T, db *testpkg.DB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT coalesce(string_agg(to_jsonb(r)::text, ',' ORDER BY r.id), '') FROM (`+staffOwnerTargetProjection+`) r`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func TestStaffOwnerBackfillRetriesInjectedFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherTenant := staffOwnerSecondTenant(t, db)
	staffOwnerFixture(t, db, tenantID, 3)
	staffOwnerFixture(t, db, otherTenant, 2)
	raise := func(ctx context.Context, tx testpkg.Tx, code string) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected %s' USING ERRCODE = '%s'; END $$`, code, code))
		return err
	}
	codes := map[int]string{1: "40P01", 2: "40001", 3: "55P03"}
	opts := StaffOwnerBackfillOptions{BatchSize: 10, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, attempt int) error {
		if id != tenantID {
			return nil
		}
		if code, ok := codes[attempt]; ok {
			return raise(ctx, tx, code)
		}
		return nil
	}}
	report, err := RunStaffOwnerBackfill(ctx, db, opts)
	require.NoError(t, err)
	cp := requireStaffOwnerTenantEqual(t, db, report, tenantID)
	// One batch per pass, two passes, each retried three times before success.
	require.EqualValues(t, 2, cp.BatchesCompleted)
	require.EqualValues(t, 6, cp.BatchesRetried)
	require.EqualValues(t, 2, cp.Deadlocks)
	require.EqualValues(t, 2, cp.SerializationFailures)
	require.EqualValues(t, 2, cp.LockTimeouts)
	require.EqualValues(t, 3, cp.RowsCopied, "rolled-back attempts must not double count")
	require.EqualValues(t, 6, cp.RowsScanned)

	_, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{MaxAttempts: 2, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		return raise(ctx, tx, "40P01")
	}})
	require.ErrorContains(t, err, "gave up after 2 attempts")
	status, err := StaffOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Equal(t, cp.RowsCopied, staffOwnerCheckpoint(t, status, tenantID).RowsCopied)
	require.Equal(t, staffTargetRows(t, db, tenantID), staffTargetRows(t, db, tenantID))

	_, err = db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'changed while the other school fails' WHERE tenant_id = ?`, otherTenant)
	require.NoError(t, err)
	report, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID {
			return nil
		}
		return raise(ctx, tx, "22023")
	}})
	require.ErrorContains(t, err, "22023", "non-transient failures surface immediately")
	require.NotNil(t, report, "the other schools are still visited and reported")
	require.Equal(t, []int64{tenantID}, report.Unstable())
	other := requireStaffOwnerTenantEqual(t, db, report, otherTenant)
	require.EqualValues(t, 4, other.RowsCopied, "a failing school does not block the remaining schools")
}

func TestStaffOwnerBackfillRereadsChangedRowsAndRemovesOrphans(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	ids := staffOwnerFixture(t, db, tenantID, 5)
	first, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	before := requireStaffOwnerTenantEqual(t, db, first, tenantID)

	_, err = db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'changed after backfill', personnel_number = NULL WHERE id = ?`, ids[0])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users.staff SET deleted_at = now() WHERE id = ?`, ids[1])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.staff WHERE id = ?`, ids[2])
	require.NoError(t, err)
	added := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Joined", "Later")

	second, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{BatchSize: 2})
	require.NoError(t, err)
	after := requireStaffOwnerTenantEqual(t, db, second, tenantID)
	require.Equal(t, before.RowsCopied+3, after.RowsCopied, "changed, soft-deleted and new rows are copied once")
	require.EqualValues(t, 1, after.RowsRemoved, "physically deleted source rows leave no orphan")
	require.EqualValues(t, 5, after.SourceCount)
	var notes string
	require.NoError(t, db.NewRaw(`SELECT staff_notes FROM users.staff_employment_profiles WHERE membership_id = ?`, ids[0]).Scan(ctx, &notes))
	require.Equal(t, "changed after backfill", notes)
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.staff_school_memberships WHERE id = ?)`, ids[2]).Scan(ctx, &exists))
	require.False(t, exists)
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.staff_employment_profiles WHERE membership_id = ?)`, added.ID).Scan(ctx, &exists))
	require.True(t, exists)
}

func TestStaffOwnerBackfillRejectsCrossTenantWorkTimeModel(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := staffOwnerSecondTenant(t, db)
	idsA := staffOwnerFixture(t, db, tenantA, 2)
	staffOwnerFixture(t, db, tenantB, 1)
	var foreignModel int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date) VALUES (?, 'Foreign', '2026-09-01') RETURNING id`, tenantB).Scan(ctx, &foreignModel))
	// Superuser writes bypass the tenant policy; the backfill must not carry
	// the cross-tenant reference into Workforce storage.
	_, err := db.ExecContext(ctx, `UPDATE users.staff SET work_time_model_id = ? WHERE id = ?`, foreignModel, idsA[0])
	require.NoError(t, err)

	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{MaxPasses: 2})
	require.NoError(t, err)
	require.False(t, report.Stable())
	require.Equal(t, []int64{tenantA}, report.Unstable())
	requireStaffOwnerTenantEqual(t, db, report, tenantB)
	cp := staffOwnerCheckpoint(t, report, tenantA)
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
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.staff_school_memberships WHERE id = ?)`, idsA[0]).Scan(ctx, &copied))
	require.False(t, copied)

	_, err = db.ExecContext(ctx, `UPDATE users.staff SET work_time_model_id = NULL WHERE id = ?`, idsA[0])
	require.NoError(t, err)
	report, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStaffOwnerTenantEqual(t, db, report, tenantA)
}

func TestStaffOwnerBackfillRestartsPassWhenPersonRejoinsMidPass(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	first := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Rejoin", "Person")
	testpkg.CreateTestStaffForTenant(t, db, tenantID, "Other", "Person")
	rejoined, rewound := false, false
	opts := StaffOwnerBackfillOptions{BatchSize: 1, injectFault: func(ctx context.Context, tx testpkg.Tx, id int64, _ int) error {
		if id != tenantID || !rejoined || rewound {
			return nil
		}
		// Read the committed checkpoint from outside the batch transaction:
		// the first batch after the conflict must start from a persisted 0.
		var persisted int64
		require.NoError(t, db.NewRaw(`SELECT high_water_id FROM platform.storage_backfill_checkpoints WHERE backfill = ? AND tenant_id = ?`, StaffOwnerBackfillName, id).Scan(ctx, &persisted))
		rewound = persisted == 0
		return nil
	}, afterBatch: func(id int64, batch staffOwnerBatch) error {
		if id != tenantID || rejoined || batch.LastID != first.ID {
			return nil
		}
		rejoined = true
		// Offboard the copied row and let the same person rejoin with a new
		// row behind the high-water mark: the target still holds the old
		// active membership, so the partial unique index rejects the new one
		// until the pass re-reads the soft deletion.
		_, err := db.ExecContext(ctx, `UPDATE users.staff SET deleted_at = now() WHERE id = ?`, first.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO users.staff (tenant_id, person_id) VALUES (?, ?)`, tenantID, first.PersonID)
		require.NoError(t, err)
		return nil
	}}
	report, err := RunStaffOwnerBackfill(ctx, db, opts)
	require.NoError(t, err)
	cp := requireStaffOwnerTenantEqual(t, db, report, tenantID)
	require.True(t, rejoined)
	require.GreaterOrEqual(t, cp.BatchesRetried, int64(1), "the unique conflict restarts the pass instead of failing")
	require.True(t, rewound, "the rewind is persisted before the restarted pass commits its first batch")
	require.EqualValues(t, 3, cp.SourceCount)
	var active int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships WHERE tenant_id = ? AND person_id = ? AND deleted_at IS NULL`, tenantID, first.PersonID).Scan(ctx, &active))
	require.EqualValues(t, 1, active)
}

func TestStaffOwnerBackfillRestartsAfterDeletedPersonRejoins(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	first := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Deleted", "Rejoin")
	otherTenant := staffOwnerSecondTenant(t, db)
	staffOwnerFixture(t, db, otherTenant, 1)
	otherSource := staffSourceRows(t, db, otherTenant)
	rejoined, recovered := false, false
	var replacementID int64
	var sourceAfterRejoin string
	opts := StaffOwnerBackfillOptions{BatchSize: 1, MaxAttempts: 2,
		afterBatch: func(id int64, batch staffOwnerBatch) error {
			if id != tenantID || rejoined || batch.LastID != first.ID {
				return nil
			}
			rejoined = true
			_, err := db.ExecContext(ctx, `DELETE FROM users.staff WHERE tenant_id = ? AND id = ?`, tenantID, first.ID)
			require.NoError(t, err)
			require.NoError(t, db.NewRaw(`INSERT INTO users.staff (tenant_id, person_id) VALUES (?, ?) RETURNING id`, tenantID, first.PersonID).Scan(ctx, &replacementID))
			sourceAfterRejoin = staffSourceRows(t, db, tenantID)
			return nil
		},
		injectFault: func(ctx context.Context, _ testpkg.Tx, id int64, _ int) error {
			if id != tenantID || !rejoined || recovered {
				return nil
			}
			var persisted int64
			require.NoError(t, db.NewRaw(`SELECT high_water_id FROM platform.storage_backfill_checkpoints WHERE backfill = ? AND tenant_id = ?`, StaffOwnerBackfillName, id).Scan(ctx, &persisted))
			if persisted == 0 {
				var orphanRows int64
				require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships WHERE tenant_id = ? AND id = ?`, tenantID, first.ID).Scan(ctx, &orphanRows))
				require.Zero(t, orphanRows, "blocking orphan must be removed before the rewound copy commits")
				recovered = true
			}
			return nil
		},
	}
	report, err := RunStaffOwnerBackfill(ctx, db, opts)
	require.NoError(t, err)
	cp := requireStaffOwnerTenantEqual(t, db, report, tenantID)
	require.True(t, recovered, "orphan cleanup and rewind must be durable before retry")
	require.Greater(t, replacementID, first.ID)
	require.EqualValues(t, 1, cp.SourceCount)
	require.EqualValues(t, 1, cp.RowsRemoved)
	require.EqualValues(t, 1, cp.BatchesRetried)
	require.Equal(t, sourceAfterRejoin, staffSourceRows(t, db, tenantID), "recovery must not modify authoritative source rows")
	require.Equal(t, otherSource, staffSourceRows(t, db, otherTenant))
	requireStaffOwnerTenantEqual(t, db, report, otherTenant)
}

func TestStaffOwnerBackfillTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := staffOwnerSecondTenant(t, db)
	staffOwnerFixture(t, db, tenantA, 2)
	staffOwnerFixture(t, db, tenantB, 3)
	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
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
			for _, table := range []string{"users.staff_school_memberships", "users.staff_employment_profiles"} {
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

func TestStaffOwnerBackfillIndexesAndQueryPlans(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	staffOwnerFixture(t, db, tenantID, 3)
	_, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	var invalidIndexes int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid
		WHERE c.oid IN ('users.staff_school_memberships'::regclass, 'users.staff_employment_profiles'::regclass, 'users.staff'::regclass)
		  AND NOT (i.indisvalid AND i.indisready)`).Scan(ctx, &invalidIndexes))
	require.Zero(t, invalidIndexes)
	var constraintViolations int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships m
		LEFT JOIN users.persons p ON p.id = m.person_id AND p.tenant_id = m.tenant_id WHERE p.id IS NULL`).Scan(ctx, &constraintViolations))
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
			"batch":           {staffOwnerCopyBatch, []any{tenantID, int64(0), 100, int64(0)}},
			"source checksum": {staffOwnerSourceChecksum, []any{tenantID}},
			"target checksum": {staffOwnerTargetChecksum, []any{tenantID}},
			"mismatch":        {staffOwnerMismatch, []any{tenantID, tenantID}},
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

func TestStaffOwnerBackfillResetAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	staffOwnerFixture(t, db, tenantID, 3)
	sourceBefore := staffSourceRows(t, db, tenantID)
	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	requireStaffOwnerTenantEqual(t, db, report, tenantID)

	require.NoError(t, ResetStaffOwnerBackfill(ctx, db))
	requireStaffOwnerTargetsEmpty(t, db)
	status, err := StaffOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	require.Empty(t, status.Tenants)
	require.Contains(t, status.MissingTenants, tenantID)
	require.False(t, status.Stable(), "unvisited schools block Cutover")
	require.Equal(t, sourceBefore, staffSourceRows(t, db, tenantID))

	report, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.NoError(t, err)
	cp := requireStaffOwnerTenantEqual(t, db, report, tenantID)
	require.EqualValues(t, 3, cp.RowsCopied, "reset restarts from zero")

	// The checkpoint table is shared with later backfills: rolling back this
	// one keeps their progress and drops the table only once it is empty.
	_, err = db.ExecContext(ctx, `INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id) VALUES ('other-backfill', ?)`, tenantID)
	require.NoError(t, err)
	require.NoError(t, staffOwnerBackfillDown(ctx, db))
	requireStaffOwnerTargetsEmpty(t, db)
	var remaining []string
	require.NoError(t, db.NewRaw(`SELECT backfill FROM platform.storage_backfill_checkpoints`).Scan(ctx, &remaining))
	require.Equal(t, []string{"other-backfill"}, remaining)
	_, err = db.ExecContext(ctx, `DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = 'other-backfill'`)
	require.NoError(t, err)
	require.NoError(t, staffOwnerBackfillDown(ctx, db))
	var checkpointsAbsent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('platform.storage_backfill_checkpoints') IS NULL`).Scan(ctx, &checkpointsAbsent))
	require.True(t, checkpointsAbsent)
	require.Equal(t, sourceBefore, staffSourceRows(t, db, tenantID))
	require.NoError(t, staffOwnerStorageDown(ctx, db), "Expand rollback follows once the targets are empty")
	require.NoError(t, staffOwnerStorageUp(ctx, db))
	require.NoError(t, staffOwnerBackfillUp(ctx, db))
	status, err = StaffOwnerBackfillStatus(ctx, db)
	require.NoError(t, err)
	requireStaffOwnerTenantEqual(t, db, status, tenantID)
	require.NoError(t, staffOwnerBackfillUp(ctx, db), "the migration is idempotent for resumed deployments")

	// After Cutover users.staff becomes a compatibility view over the targets;
	// target-only truncation would then destroy the authoritative rows.
	_, err = db.ExecContext(ctx, `ALTER TABLE users.staff RENAME TO staff_legacy; CREATE VIEW users.staff AS SELECT * FROM users.staff_legacy`)
	require.NoError(t, err)
	require.ErrorContains(t, ResetStaffOwnerBackfill(ctx, db), "not a base table")
	require.ErrorContains(t, staffOwnerBackfillDown(ctx, db), "not a base table")
	_, err = RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	require.ErrorContains(t, err, "not a base table")
	var membershipCount int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships WHERE tenant_id = ?`, tenantID).Scan(ctx, &membershipCount))
	require.EqualValues(t, 3, membershipCount)
	_, err = db.ExecContext(ctx, `DROP VIEW users.staff; ALTER TABLE users.staff_legacy RENAME TO staff`)
	require.NoError(t, err)
}

func requireStaffOwnerTargetsEmpty(t *testing.T, db *testpkg.DB) {
	t.Helper()
	for _, table := range []string{"users.staff_school_memberships", "users.staff_employment_profiles"} {
		var count int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+table).Scan(context.Background(), &count))
		require.Zero(t, count, "%s must be empty", table)
	}
}
