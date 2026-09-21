package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func (d *decisionBoundary) RecordMarkedDone(context.Context, *carerequests.Request, int64, string) error {
	d.calls = append(d.calls, "record_done")
	return d.ledgerErr
}
func (d *decisionBoundary) NotifyMarkedDone(context.Context, *carerequests.Request, int64, string) error {
	d.calls = append(d.calls, "notify_done")
	return d.notificationErr
}

func TestNativeMarkDoneOnlyClosesPastPickupRequests(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Request", "Completion", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	boundary := &decisionBoundary{owner: owner, student: compose.ReviewStudent{ID: student.ID}}
	service, err := compose.NewRequestDecisions(compose.RequestDecisionDependencies{Records: owner, People: boundary, Plans: boundary,
		Effects: boundary, Today: func() calendar.Date { return "2026-09-20" }})
	require.NoError(t, err)
	create := func(kind, payload string) carerequests.Request {
		row, createErr := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: student.ID, SubmittedBy: account.ID,
			RequestKind: kind, Status: "pending", Payload: json.RawMessage(payload)})
		require.NoError(t, createErr)
		return row
	}
	closeRequest := func(row carerequests.Request) error {
		return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			return service.MarkDone(txCtx, row.ID, careplan.ParentRequestVersion(row.UpdatedAt), "  Erledigt  ", account.ID)
		})
	}
	for _, payload := range []string{`{"date":"2026-09-20"}`, `{"date":"2026-09-21"}`, `{"date":"invalid"}`} {
		require.ErrorIs(t, closeRequest(create("pickup_change", payload)), careplan.ErrParentRequestNotPast)
	}
	require.ErrorIs(t, closeRequest(create("weekly_schedule", `{"weekdays":[]}`)), careplan.ErrParentRequestNotPast)
	row := create("pickup_change", `{"date":"2026-09-19"}`)
	boundary.ledgerErr = errors.New("ledger unavailable")
	require.ErrorIs(t, closeRequest(row), boundary.ledgerErr)
	boundary.ledgerErr, boundary.notificationErr = nil, errors.New("notice unavailable")
	require.ErrorIs(t, closeRequest(row), boundary.notificationErr)
	unchanged, err := owner.FindCareScheduleRequest(ctx, row.ID, false)
	require.NoError(t, err)
	require.Equal(t, "pending", unchanged.Status)
	boundary.notificationErr, boundary.calls = nil, nil
	require.NoError(t, closeRequest(row))
	require.Equal(t, []string{"lock_student", "authorize", "record_done", "notify_done"}, boundary.calls)
	closed, err := owner.FindCareScheduleRequest(ctx, row.ID, false)
	require.NoError(t, err)
	require.Equal(t, "done", closed.Status)
	require.NotNil(t, closed.DecisionReason)
	require.Equal(t, "Erledigt", *closed.DecisionReason)
	require.Nil(t, closed.AppliedAt)
	rows, err := owner.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Empty(t, rows)
}
