package schedule_test

// Freeze tests for the care-request decision snapshot (#2430, ADR 0002):
// deciding a request persists the alt → neu diff the reviewer saw, and the
// history keeps replaying that frozen state even after the live data moves on.
// Hermetic: real test DB, fixtures with cleanup, no hardcoded IDs.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

// upsertMondayPickup writes the child's live Monday pickup time directly —
// the "data changed after the decision" step of the freeze scenario.
func upsertMondayPickup(t *testing.T, f *careFixture, hour, minute int) {
	t.Helper()
	require.NoError(t, f.sf.PickupSchedule.UpsertStudentPickupSchedule(
		f.staffCtx(f.staffAccount),
		&scheduleModels.StudentPickupSchedule{
			StudentID:  f.chain.StudentID,
			Weekday:    1,
			PickupTime: time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC),
			CreatedBy:  f.staffID,
		},
	))
}

func historyItemByID(t *testing.T, f *careFixture, id int64) *schedule.CareRequestHistoryItem {
	t.Helper()
	items, _, err := f.svc.ListHistory(f.staffCtx(f.staffAccount), time.Time{}, 0, 25)
	require.NoError(t, err)
	for _, item := range items {
		if item.Request.ID == id {
			return item
		}
	}
	t.Fatalf("request %d not found in history", id)
	return nil
}

// TestDecide_FreezesAltNeuDiffInHistory is the #2430 acceptance scenario:
// approve a request, then change the live data — the history keeps showing
// the alt → neu comparison that existed at decision time, not a live diff.
func TestDecide_FreezesAltNeuDiffInHistory(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)

	// Live plan before the request: Monday pickup 15:00.
	upsertMondayPickup(t, f, 15, 0)

	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "pickup": "16:00"},
	))
	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	// The live data moves on after the decision.
	upsertMondayPickup(t, f, 17, 0)

	item := historyItemByID(t, f, req.ID)
	require.Equal(t, []schedule.RequestDiffEntry{{
		Label:    "Montag · Abholzeit",
		Old:      "15:00",
		New:      "16:00",
		Weekday:  1,
		CareKind: schedule.DiffCareKindPickup,
	}}, item.Diff, "the frozen diff must survive later data changes")
	assert.NotEmpty(t, item.Requested, "the payload summary stays available alongside the snapshot")
}

// TestDecide_RejectAlsoFreezesDiff: a rejection freezes the comparison the
// reviewer declined, even though nothing was applied.
func TestDecide_RejectAlsoFreezesDiff(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)

	upsertMondayPickup(t, f, 15, 0)
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "pickup": "18:00"},
	))
	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: false, Reason: "zu spät", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	upsertMondayPickup(t, f, 14, 30)

	item := historyItemByID(t, f, req.ID)
	require.Equal(t, []schedule.RequestDiffEntry{{
		Label:    "Montag · Abholzeit",
		Old:      "15:00",
		New:      "18:00",
		Weekday:  1,
		CareKind: schedule.DiffCareKindPickup,
	}}, item.Diff)
}

// TestWithdraw_LeavesNoSnapshot: a guardian withdrawal is not a staff
// decision — the row keeps no frozen diff and the history falls back to the
// payload-derived requested summary.
func TestWithdraw_LeavesNoSnapshot(t *testing.T) {
	f := newCareFixture(t)

	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 2, "arrival": "07:45"},
	))
	_, err := f.svc.WithdrawRequest(f.staffCtx(f.chain.AccountID), req.ID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)

	item := historyItemByID(t, f, req.ID)
	assert.Nil(t, item.Diff, "withdrawals carry no frozen diff")
	assert.NotEmpty(t, item.Requested)
}

// TestDecide_PickupChangeFreezesDiff covers the pickup-change kind: the
// one-day exception's alt → neu is frozen on decision too.
func TestDecide_PickupChangeFreezesDiff(t *testing.T) {
	f := newCareFixture(t)
	upsertMondayPickup(t, f, 15, 0)

	// Next Monday (always in the future), so the exception's weekday fallback
	// (the live Monday plan) supplies the old side.
	date := timezone.TodayDate().AddDays(1)
	for date.Weekday() != time.Monday {
		date = date.AddDays(1)
	}
	req, err := f.svc.CreatePickupChangeRequest(
		f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		date, time.Date(0, 1, 1, 16, 30, 0, 0, time.UTC), "Arzttermin",
	)
	require.NoError(t, err)
	_, err = f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	// Change the live Monday plan after the decision.
	upsertMondayPickup(t, f, 13, 0)

	item := historyItemByID(t, f, req.ID)
	require.Equal(t, []schedule.RequestDiffEntry{{
		Label:    date.String() + " · Abholzeit",
		Old:      "15:00",
		New:      "16:30",
		CareKind: schedule.DiffCareKindPickup,
	}}, item.Diff)
}
