package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type decisionBoundary struct {
	owner           careplan.Capability
	student         compose.ReviewStudent
	staffID         int64
	ledgerErr       error
	notificationErr error
	guardianAccess  bool
	calls           []string
}

func (d *decisionBoundary) LockStudent(context.Context, int64) (compose.ReviewStudent, error) {
	d.calls = append(d.calls, "lock_student")
	return d.student, nil
}
func (d *decisionBoundary) CanReview(context.Context, compose.ReviewStudent) (bool, error) {
	d.calls = append(d.calls, "authorize")
	return true, nil
}
func (d *decisionBoundary) GuardianHasAccess(context.Context, int64, int64) (bool, error) {
	d.calls = append(d.calls, "guardian")
	return d.guardianAccess, nil
}
func (d *decisionBoundary) Snapshot(context.Context, *carerequests.Request) *carerequests.DecisionSnapshot {
	d.calls = append(d.calls, "snapshot")
	return carerequests.FreezeDecision([]carerequests.DiffEntry{{Label: "Montag · Abholzeit", Old: "15:00", New: "14:00", Weekday: 1, CareKind: carerequests.KindPickup}})
}
func (d *decisionBoundary) ApplyWeekly(ctx context.Context, request *carerequests.Request, _ int64) (bool, error) {
	d.calls = append(d.calls, "apply")
	_, err := d.owner.CreatePickupSchedule(ctx, careplan.PickupSchedule{StudentID: request.StudentID, Weekday: 1,
		PickupTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), CreatedBy: d.staffID})
	return false, err
}
func (*decisionBoundary) ApplyPickup(context.Context, *carerequests.Request, *string, bool) (int64, error) {
	return 0, errors.New("unexpected pickup application")
}
func (d *decisionBoundary) RegisterDecision(context.Context, *carerequests.Request, carerequests.DecideInput, string, bool) error {
	d.calls = append(d.calls, "effects")
	return nil
}
func (d *decisionBoundary) ReloadDecision(ctx context.Context, id int64) (*carerequests.ReviewItem, error) {
	d.calls = append(d.calls, "reload")
	row, err := d.owner.FindCareScheduleRequest(ctx, id, false)
	return &carerequests.ReviewItem{Request: &row}, err
}
func (d *decisionBoundary) RecordDecision(context.Context, *carerequests.Request, int64, map[string]any) error {
	d.calls = append(d.calls, "ledger")
	return d.ledgerErr
}

func TestNativeRequestDecisionRollsBackAppliedPlanAndSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Decision", "Transaction", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	staff, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Decision", "Reviewer")
	row, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: student.ID, SubmittedBy: account.ID,
		RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"14:00"}]}`)})
	require.NoError(t, err)
	boundary := &decisionBoundary{owner: owner, student: compose.ReviewStudent{ID: student.ID}, staffID: staff.ID,
		guardianAccess: true, ledgerErr: errors.New("decision ledger unavailable")}
	service, err := compose.NewRequestDecisions(compose.RequestDecisionDependencies{Records: owner, People: boundary, Plans: boundary,
		Effects: boundary, Today: func() calendar.Date { return "2026-09-20" }})
	require.NoError(t, err)
	input := carerequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: reviewer.ID, ExpectedVersion: careplan.ParentRequestVersion(row.UpdatedAt)}
	_, err = service.Decide(ctx, input)
	require.ErrorIs(t, err, boundary.ledgerErr)
	require.Equal(t, []string{"lock_student", "authorize", "snapshot", "guardian", "apply", "effects", "reload", "ledger"}, boundary.calls)
	unchanged, err := owner.FindCareScheduleRequest(ctx, row.ID, false)
	require.NoError(t, err)
	require.Equal(t, "pending", unchanged.Status)
	var absent *carerequests.DecisionSnapshot
	require.NoError(t, json.Unmarshal(unchanged.DecisionSnapshot, &absent))
	require.Nil(t, absent)
	rows, err := owner.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Empty(t, rows)
	boundary.ledgerErr, boundary.calls = nil, nil
	result, err := service.Decide(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "approved", result.Request.Status)
	var snapshot carerequests.DecisionSnapshot
	require.NoError(t, json.Unmarshal(result.Request.DecisionSnapshot, &snapshot))
	require.Equal(t, "15:00", snapshot.Entries()[0].Old)
	rows, err = owner.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
