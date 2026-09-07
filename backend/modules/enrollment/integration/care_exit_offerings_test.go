package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type careExitOfferingSchool struct {
	schoolID       int64
	studentID      int64
	requestChildID int64
	running        int64
	future         int64
	ended          int64
}

// TestCareExitOfferingLinksTenantScopeRollbackAndRestore covers the offering
// link projection, lock, snapshot, end and restore Enrollment serves to the
// care-exit cleanup and booking consistency audit (#2695).
func TestCareExitOfferingLinksTenantScopeRollbackAndRestore(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	const validUntil = enrollment.Date("2031-02-01")
	schools := make([]careExitOfferingSchool, 0, 2)
	for _, school := range []string{"first", "second"} {
		t.Run(school, func(t *testing.T) {
			testpkg.OwnTenant(t)
			schoolID := testpkg.Tenant(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			student := testpkg.CreateTestStudent(t, db, "Care", "Exit", "2b")
			running := testpkg.CreateTestCareOffering(t, db, phase.ID, "running")
			future := testpkg.CreateTestCareOffering(t, db, phase.ID, "future")
			ended := testpkg.CreateTestCareOffering(t, db, phase.ID, "ended")
			fixture := careExitOfferingSchool{schoolID: schoolID, studentID: student.ID}
			ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolID)
			err := testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, tx bun.Tx) error {
				var requestID int64
				err := tx.NewRaw(`INSERT INTO enrollment.requests (tenant_id,phase_id,guardian_first_name,guardian_last_name,guardian_email,status_token,consent_flags,custom_data)
				 VALUES (?,?,'First','Last','care-exit@example.test',?,'{}'::jsonb,'{}'::jsonb) RETURNING id`, schoolID, phase.ID, fmt.Sprintf("care-exit-offerings-%d", schoolID)).Scan(txCtx, &requestID)
				if err != nil {
					return err
				}
				err = tx.NewRaw(`INSERT INTO enrollment.request_children
				 (tenant_id,request_id,first_name,last_name,date_of_birth,status,activation_mode,sort_order,custom_data,created_student_id)
				 VALUES (?,?,'Care','Exit','2018-04-15','approved','scheduled',0,'{}'::jsonb,?) RETURNING id`, schoolID, requestID, student.ID).Scan(txCtx, &fixture.requestChildID)
				if err != nil {
					return err
				}
				rows := []struct {
					offeringID int64
					from, to   *string
				}{
					{running.ID, nil, nil},
					{future.ID, new("2031-03-01"), nil},
					{ended.ID, nil, new("2031-01-01")},
				}
				ids := []*int64{&fixture.running, &fixture.future, &fixture.ended}
				for i, row := range rows {
					err := tx.NewRaw(`INSERT INTO enrollment.request_child_offerings (tenant_id,request_child_id,care_offering_id,selected_days,valid_from,valid_until)
					 VALUES (?,?,?,'["mon","wed"]'::jsonb,?::date,?::date) RETURNING id`, schoolID, fixture.requestChildID, row.offeringID, row.from, row.to).Scan(txCtx, ids[i])
					if err != nil {
						return err
					}
				}
				return nil
			})
			require.NoError(t, err)
			schools = append(schools, fixture)
		})
	}
	for i, school := range schools {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), school.schoolID)
		foreignCtx := testpkg.ContextForTenant(testpkg.Ctx(t), schools[1-i].schoolID)

		audit, err := module.ApprovedBookingOfferingLinks(ctx)
		require.NoError(t, err)
		require.Len(t, audit, 3, "the audit projection holds every link of the approved child")
		for _, link := range audit {
			require.Equal(t, school.schoolID, link.TenantID)
			require.Equal(t, school.requestChildID, link.RequestChildID)
			require.Equal(t, []string{"mon", "wed"}, link.SelectedDays)
		}
		require.Nil(t, audit[0].ValidFrom)
		require.Equal(t, enrollment.Date("2031-03-01"), *audit[1].ValidFrom)
		require.Equal(t, enrollment.Date("2031-01-01"), *audit[2].ValidUntil)

		links, err := module.CareExitOfferingLinks(ctx, []int64{school.studentID, schools[1-i].studentID})
		require.NoError(t, err)
		require.Len(t, links, 3, "selected students are filtered to the caller's school")
		all, err := module.CareExitOfferingLinks(ctx, nil)
		require.NoError(t, err)
		require.Len(t, all, 3, "the scheduled evaluation sees the whole school and nothing else")
		foreign, err := module.CareExitOfferingLinks(foreignCtx, []int64{school.studentID})
		require.NoError(t, err)
		require.Empty(t, foreign, "another school's students have no links here")

		snapshots, err := module.CareExitOfferingSnapshots(ctx, []int64{school.studentID}, validUntil, nil)
		require.NoError(t, err)
		require.Len(t, snapshots, 2, "links that already ended are not part of the exit")
		require.Equal(t, school.running, snapshots[0].SourceRowID)
		require.False(t, snapshots[0].WasDeleted)
		require.Equal(t, school.future, snapshots[1].SourceRowID)
		require.True(t, snapshots[1].WasDeleted)
		for _, snapshot := range snapshots {
			require.Equal(t, school.schoolID, snapshot.TenantID)
			require.Equal(t, school.studentID, snapshot.StudentID)
			require.Equal(t, school.requestChildID, snapshot.RequestChildID)
			require.Contains(t, string(snapshot.Snapshot), `"care_offering_id"`)
		}
		none, err := module.CareExitOfferingSnapshots(ctx, []int64{school.studentID}, validUntil, new(school.requestChildID+1_000_000))
		require.NoError(t, err)
		require.Empty(t, none, "a source application filter narrows the snapshot")
		foreignSnapshots, err := module.CareExitOfferingSnapshots(foreignCtx, []int64{school.studentID}, validUntil, nil)
		require.NoError(t, err)
		require.Empty(t, foreignSnapshots)

		endFailure := errors.New("after ending offering links")
		err = testpkg.WithTenantTx(t, ctx, db, school.schoolID, func(txCtx context.Context, _ bun.Tx) error {
			require.NoError(t, module.LockCareExitOfferingLinks(txCtx, []int64{school.requestChildID}, validUntil))
			changed, err := module.EndCareExitOfferingLinks(txCtx, []int64{school.requestChildID}, nil, validUntil)
			require.NoError(t, err)
			require.EqualValues(t, 2, changed)
			return endFailure
		})
		require.ErrorIs(t, err, endFailure)
		afterRollback, err := module.CareExitOfferingLinks(ctx, []int64{school.studentID})
		require.NoError(t, err)
		require.Len(t, afterRollback, 3, "a failed workflow keeps every link")
		require.Nil(t, afterRollback[0].ValidUntil)

		foreignChanged, err := module.EndCareExitOfferingLinks(foreignCtx, []int64{school.requestChildID}, nil, validUntil)
		require.NoError(t, err)
		require.Zero(t, foreignChanged, "another school cannot end these links")

		changed, err := module.EndCareExitOfferingLinks(ctx, []int64{school.requestChildID}, nil, validUntil)
		require.NoError(t, err)
		require.EqualValues(t, 2, changed)
		afterEnd, err := module.CareExitOfferingLinks(ctx, []int64{school.studentID})
		require.NoError(t, err)
		require.Len(t, afterEnd, 2, "the future link is deleted")
		require.Equal(t, school.running, afterEnd[0].ID)
		require.Equal(t, validUntil, *afterEnd[0].ValidUntil, "the running link is capped")
		require.Equal(t, school.ended, afterEnd[1].ID)

		restores := make([]enrollment.CareExitOfferingSnapshotRestore, 0, len(snapshots))
		for _, snapshot := range snapshots {
			restores = append(restores, enrollment.CareExitOfferingSnapshotRestore{SourceRowID: snapshot.SourceRowID, WasDeleted: snapshot.WasDeleted, Snapshot: snapshot.Snapshot})
		}
		foreignRestored, err := module.RestoreCareExitOfferingLinks(foreignCtx, restores)
		require.NoError(t, err)
		require.Zero(t, foreignRestored, "another school cannot replay this ledger")
		stillEnded, err := module.CareExitOfferingLinks(ctx, []int64{school.studentID})
		require.NoError(t, err)
		require.Len(t, stillEnded, 2)
		require.Equal(t, validUntil, *stillEnded[0].ValidUntil)

		restoreFailure := errors.New("after restoring offering links")
		err = testpkg.WithTenantTx(t, ctx, db, school.schoolID, func(txCtx context.Context, _ bun.Tx) error {
			restored, err := module.RestoreCareExitOfferingLinks(txCtx, restores)
			require.NoError(t, err)
			require.EqualValues(t, 1, restored)
			return restoreFailure
		})
		require.ErrorIs(t, err, restoreFailure)
		stillEnded, err = module.CareExitOfferingLinks(ctx, []int64{school.studentID})
		require.NoError(t, err)
		require.Len(t, stillEnded, 2, "a failed restore persists nothing")

		restored, err := module.RestoreCareExitOfferingLinks(ctx, restores)
		require.NoError(t, err)
		require.EqualValues(t, 1, restored, "the deleted link is recreated")
		afterRestore, err := module.CareExitOfferingLinks(ctx, []int64{school.studentID})
		require.NoError(t, err)
		require.Len(t, afterRestore, 3)
		require.Equal(t, school.running, afterRestore[0].ID)
		require.Nil(t, afterRestore[0].ValidUntil, "the capped link recovers its open end")
		require.Equal(t, school.future, afterRestore[1].ID, "the deleted link keeps its original id")
		require.Equal(t, enrollment.Date("2031-03-01"), *afterRestore[1].ValidFrom)
		restored, err = module.RestoreCareExitOfferingLinks(ctx, restores)
		require.NoError(t, err)
		require.Zero(t, restored, "replaying the ledger again is a no-op")
	}

	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schools[0].schoolID)
	err := testpkg.WithTenantTx(t, ctx, db, schools[0].schoolID, func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw("SELECT 1 / 0").Exec(txCtx)
		require.Error(t, err)
		_, auditErr := module.ApprovedBookingOfferingLinks(txCtx)
		require.ErrorContains(t, auditErr, "list approved booking offering links")
		_, linksErr := module.CareExitOfferingLinks(txCtx, nil)
		require.ErrorContains(t, linksErr, "list care-exit offering links")
		require.ErrorContains(t, module.LockCareExitOfferingLinks(txCtx, []int64{schools[0].requestChildID}, validUntil), "lock care-exit offering links")
		_, snapshotErr := module.CareExitOfferingSnapshots(txCtx, []int64{schools[0].studentID}, validUntil, nil)
		require.ErrorContains(t, snapshotErr, "snapshot care-exit offering links")
		_, endErr := module.EndCareExitOfferingLinks(txCtx, []int64{schools[0].requestChildID}, nil, validUntil)
		require.ErrorContains(t, endErr, "delete future care-exit offering links")
		_, restoreErr := module.RestoreCareExitOfferingLinks(txCtx, []enrollment.CareExitOfferingSnapshotRestore{{SourceRowID: schools[0].running}})
		require.ErrorContains(t, restoreErr, "restore capped care-exit offering links")
		return err
	})
	require.Error(t, err)
	links, err := module.CareExitOfferingLinks(ctx, []int64{schools[0].studentID})
	require.NoError(t, err)
	require.Len(t, links, 3, "retry succeeds after the failed transaction rolls back")
}
