package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionEndRejectsVisitsCommittedWhileWaitingForTheSession(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	group := newSession(t, db, "Changed visit cohort")
	openVisit(t, ctx, module, db, group.ID, "Original")
	arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Cohort", "3a")
	holder := startTx(t, ctx, db)
	awaitStep(t, holder.run(func(txCtx context.Context) error {
		var id int64
		if err := holder.tx.NewRaw("SELECT id FROM active.groups WHERE tenant_id = ? AND id = ? FOR NO KEY UPDATE", testpkg.Tenant(t), group.ID).Scan(txCtx, &id); err != nil {
			return err
		}
		_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: group.ID, EntryTime: time.Now()})
		return err
	}))
	ending := startTx(t, ctx, db)
	ended := ending.run(func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, time.Now())
		return err
	})
	awaitBlockedBy(t, db, ending, holder)
	holder.commit(t)
	select {
	case err := <-ended:
		require.ErrorIs(t, err, studentpresence.ErrVisitWriteConflict)
	case <-time.After(10 * time.Second):
		t.Fatal("session end did not finish after the session holder committed")
	}
	// The actor rolls back on the command's error. Both visits and the
	// group's lifecycle must remain untouched, not partially closed.
	select {
	case <-ending.done:
		require.ErrorIs(t, ending.err, studentpresence.ErrVisitWriteConflict)
	case <-time.After(10 * time.Second):
		t.Fatal("failed session end did not release its transaction")
	}
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{group.ID}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, visits, 2)
	groups, err := module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: []int64{group.ID}})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Nil(t, groups[0].EndTime)
}
