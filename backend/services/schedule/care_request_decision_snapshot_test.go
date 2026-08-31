package schedule_test

// Freeze tests for the care-request decision snapshot (#2430, ADR 0002):
// deciding a request persists the alt → neu diff the reviewer saw, and the
// history keeps replaying that frozen state even after the live data moves on.
// Hermetic: real test DB, fixtures with cleanup, no hardcoded IDs.

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

type failingPickupReadService struct {
	schedule.PickupScheduleService
	err error
}

func (s failingPickupReadService) GetStudentPickupSchedules(context.Context, int64) ([]*scheduleModels.StudentPickupSchedule, error) {
	return nil, s.err
}

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
	items, _, err := f.svc.ListHistory(f.staffCtx(f.staffAccount), modelBase.RequestQueueFilters{Limit: 25})
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
	t.Parallel()

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
	t.Parallel()

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

func TestDecide_SkipsSnapshotWhenPickupPlanReadFails(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	readErr := errors.New("pickup plan unavailable")
	rejectingService := newCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		failingPickupReadService{PickupScheduleService: f.sf.PickupSchedule, err: readErr},
		f.sf,
		nil,
		nil,
		slog.Default(),
	)
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "pickup": "16:00"},
	))

	_, err := rejectingService.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: false, Reason: "nicht möglich", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	item := historyItemByID(t, f, req.ID)
	assert.Nil(t, item.Diff, "an unavailable pickup plan must not produce a fabricated snapshot")
	assert.NotEmpty(t, item.Requested, "history must fall back to the payload summary")
}

func TestListPending_FallsBackToRequestedWhenPickupPlanReadFails(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	readErr := errors.New("pickup plan unavailable")
	service := newCareScheduleRequestService(
		f.repos.CareScheduleChangeRequest,
		f.repos.Student,
		f.repos.Person,
		f.sf.ArrivalSchedule,
		failingPickupReadService{PickupScheduleService: f.sf.PickupSchedule, err: readErr},
		f.sf,
		nil,
		nil,
		slog.Default(),
	)
	f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "pickup": "16:00"},
	))

	items, _, err := service.ListPending(f.staffCtx(f.staffAccount), modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, []schedule.RequestDiffEntry{{
		Label:    "Montag · Abholzeit",
		New:      "16:00",
		Weekday:  1,
		CareKind: schedule.DiffCareKindPickup,
	}}, items[0].Diff)
}

// TestWithdraw_LeavesNoSnapshot: a guardian withdrawal is not a staff
// decision — the row keeps no frozen diff and the history falls back to the
// payload-derived requested summary.
func TestWithdraw_LeavesNoSnapshot(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)

	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 2, "arrival": "07:45"},
	))
	// Guardian withdrawal was retired in #2267 (guardians edit instead); the
	// withdrawn STATUS survives for historic rows, so it is written straight
	// through the repository here.
	err := f.repos.CareScheduleChangeRequest.Decide(
		f.staffCtx(f.chain.AccountID), req.ID, scheduleModels.CareRequestStatusWithdrawn, nil, nil, false,
	)
	require.NoError(t, err)

	item := historyItemByID(t, f, req.ID)
	assert.Nil(t, item.Diff, "withdrawals carry no frozen diff")
	assert.NotEmpty(t, item.Requested)
}

// TestListHistory_LegacyDecidedRowFallsBackToRequested: a row decided before
// the snapshot existed (simulated by clearing the column) carries no Diff and
// the history keeps serving the payload-derived requested summary.
func TestListHistory_LegacyDecidedRowFallsBackToRequested(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)

	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "pickup": "16:00"},
	))
	_, err := f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: false, Reason: "passt nicht", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	// Altzeile: pre-#2430 decided rows have no snapshot.
	_, err = f.db.NewUpdate().
		TableExpr("schedule.care_schedule_change_requests").
		Set("decision_snapshot = NULL").
		Where("id = ?", req.ID).
		Exec(t.Context())
	require.NoError(t, err)

	item := historyItemByID(t, f, req.ID)
	assert.Nil(t, item.Diff, "a row without a snapshot must not fabricate a diff")
	assert.NotEmpty(t, item.Requested, "the payload summary remains the fallback")
}

// TestDecide_PickupChangeFreezesDiff covers the pickup-change kind: the
// one-day exception's alt → neu is frozen on decision too.
func TestDecide_PickupChangeFreezesDiff(t *testing.T) {
	t.Parallel()

	f := newPickupChangeFixture(t)
	upsertMondayPickup(t, f, 15, 0)

	// Next Monday (always in the future), so the exception's weekday fallback
	// (the live Monday plan) supplies the old side.
	date := timezone.NewDate(2026, 8, 24).AddDays(1)
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
		Label:    date.Format("02.01.2006") + " · Abholzeit",
		Old:      "15:00",
		New:      "16:30",
		CareKind: schedule.DiffCareKindPickup,
	}}, item.Diff)
}
