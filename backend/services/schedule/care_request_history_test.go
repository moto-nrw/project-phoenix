package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

// TestListHistory_DecidedRequestsWithReviewerAndSummary proves the staff
// history: decided weekly-schedule requests come back newest-decision-first
// with the child's name, the reviewer's display name, the decision reason, and
// the payload-derived requested summary (no recomputed live diff) — while
// pending rows stay out and withdrawals carry no reviewer.
func TestListHistory_DecidedRequestsWithReviewerAndSummary(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)

	// Request 1: rejected by the staff account.
	first := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))
	staffCtx := f.staffCtx(f.staffAccount)
	_, err := f.svc.Decide(staffCtx, schedule.CareRequestDecideInput{
		RequestID: first.ID, Approve: false, Reason: "passt nicht", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	// Request 2: withdrawn by the guardian — no reviewer stamp.
	second := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 2, "arrival": "07:30"},
	))
	// Withdrawal is retired (#2267); the status stays valid for historic rows.
	err = f.repos.CareScheduleChangeRequest.Decide(
		f.staffCtx(f.chain.AccountID), second.ID, scheduleModels.CareRequestStatusWithdrawn, nil, nil, false,
	)
	require.NoError(t, err)

	// Request 3: still pending — must never surface in the history.
	third := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 3, "pickup": "15:00"},
	))

	items, next, err := f.svc.ListHistory(staffCtx, modelBase.RequestQueueFilters{Limit: 25})
	require.NoError(t, err)
	assert.Nil(t, next)

	byID := map[int64]*schedule.CareRequestHistoryItem{}
	for _, item := range items {
		byID[item.Request.ID] = item
		assert.NotEqual(t, third.ID, item.Request.ID, "pending rows must never appear in the history")
	}
	require.Contains(t, byID, first.ID)
	require.Contains(t, byID, second.ID)

	rejected := byID[first.ID]
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, rejected.Request.Status)
	assert.Equal(t, "Felix", rejected.FirstName)
	assert.Equal(t, "Schneider", rejected.LastName)
	assert.Equal(t, "Clara Confirm", rejected.ReviewerName)
	require.NotNil(t, rejected.Request.DecisionReason)
	assert.Equal(t, "passt nicht", *rejected.Request.DecisionReason)
	require.NotEmpty(t, rejected.Requested, "history rows carry the payload-derived requested summary")
	for _, entry := range rejected.Requested {
		assert.Empty(t, entry.Old, "the requested summary has no recomputed current side")
		assert.NotEmpty(t, entry.New)
	}

	withdrawn := byID[second.ID]
	assert.Equal(t, scheduleModels.CareRequestStatusWithdrawn, withdrawn.Request.Status)
	assert.Empty(t, withdrawn.ReviewerName, "withdrawals carry no reviewer")

	// Keyset pagination: page size 1 → disjoint pages.
	page1, next1, err := f.svc.ListHistory(staffCtx, modelBase.RequestQueueFilters{Limit: 1})
	require.NoError(t, err)
	require.Len(t, page1, 1)
	require.NotNil(t, next1)
	page2, next2, err := f.svc.ListHistory(staffCtx, modelBase.RequestQueueFilters{BeforeInstant: next1.UpdatedAt, BeforeID: next1.ID, Limit: 1})
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.NotEqual(t, page1[0].Request.ID, page2[0].Request.ID, "pages must not overlap")
	assert.Nil(t, next2, "two decided rows fit exactly two pages of one")
}

func TestListHistory_IncludesPickupChangeWithPayloadSummary(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	date := timezone.TodayDate().AddDays(1)
	pickupTime := time.Date(0, 1, 1, 15, 30, 0, 0, time.UTC)

	req, err := f.svc.CreatePickupChangeRequest(
		f.staffCtx(f.chain.AccountID), f.chain.StudentID, f.chain.AccountID,
		date, pickupTime, "Heute später",
	)
	require.NoError(t, err)
	_, err = f.svc.Decide(f.staffCtx(f.staffAccount), schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: false, Reason: "Nicht möglich", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	items, _, err := f.svc.ListHistory(f.staffCtx(f.staffAccount), modelBase.RequestQueueFilters{Limit: 25})
	require.NoError(t, err)
	var got *schedule.CareRequestHistoryItem
	for _, item := range items {
		if item.Request.ID == req.ID {
			got = item
			break
		}
	}
	require.NotNil(t, got)
	assert.Equal(t, scheduleModels.CareRequestKindPickupChange, got.Request.RequestKind)
	require.Equal(t, []schedule.RequestDiffEntry{{
		Label: date.Format("02.01.2006") + " · Abholzeit", New: "15:30", CareKind: schedule.DiffCareKindPickup,
	}}, got.Requested)
}
