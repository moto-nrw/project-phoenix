package contracttest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type conflictPlanBoundary struct {
	*pickupApprovalFixture
	carerequests.PickupApprovals
	owner careplan.Capability
}

func (p *conflictPlanBoundary) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	p.calls = append(p.calls, "student-day-lock")
	return p.owner.LockStudentAndExceptionDay(ctx, id, date)
}
func (p *conflictPlanBoundary) UpsertStudentPickupSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	_, err := p.owner.UpsertPickupSchedule(ctx, *row)
	return err
}

func TestNativeConflictStaffPickupWritesOnlyRequestedDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Conflict", "Pickup", "1a")
	staff, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Conflict", "Reviewer")
	guardian := testpkg.CreateTestAccount(t, db, "parent")
	today := func() calendar.Date { return "2026-09-21" }
	request, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{
		StudentID: student.ID, SubmittedBy: guardian.ID, RequestKind: "pickup_change", Status: "pending",
		Payload: json.RawMessage(`{"date":"2026-09-21","pickup_time":"14:00","reason":"Appointment"}`),
	})
	require.NoError(t, err)
	decisionPort := &decisionBoundary{owner: owner, student: compose.ReviewStudent{ID: student.ID}, staffID: staff.ID, guardianAccess: true}
	decisions, err := compose.NewRequestDecisions(compose.RequestDecisionDependencies{
		Records: owner, People: decisionPort, Plans: decisionPort, Effects: decisionPort, Today: today,
	})
	require.NoError(t, err)
	exceptions, err := compose.NewPickupApprovalExceptions(owner)
	require.NoError(t, err)
	probe := &pickupApprovalFixture{staffID: staff.ID}
	approvals, err := compose.NewPickupApprovals(compose.PickupApprovalDependencies{
		Fingerprint: securityruntime.Fingerprint,
		People:      probe, Presence: probe, Exceptions: exceptions, Excusal: probe, Locker: owner, Today: today,
	})
	require.NoError(t, err)
	plans := &conflictPlanBoundary{pickupApprovalFixture: probe, PickupApprovals: approvals, owner: owner}
	conflicts, err := compose.NewRequestConflicts(owner, plans, decisions, today)
	require.NoError(t, err)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		candidate, err := conflicts.ConflictCandidate(txCtx, request.ID)
		if err != nil {
			return err
		}
		require.Equal(t, student.ID, candidate.StudentID)
		if err := conflicts.LockConflictRequest(txCtx, request.ID); err != nil {
			return err
		}
		if err := conflicts.DecideConflictRequest(txCtx, carerequests.DecideInput{
			RequestID: request.ID, Reason: "Staff alternative", ReviewedBy: reviewer.ID,
		}); err != nil {
			return err
		}
		return conflicts.WriteStaffValue(txCtx, carerequests.StaffValueWrite{
			StudentID: student.ID, RequestIDs: []int64{request.ID}, Reason: "Staff alternative", PickupTime: "15:45",
		})
	})
	require.NoError(t, err)
	rows, err := owner.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, careplan.Date("2026-09-21"), rows[0].ExceptionDate)
	require.NotNil(t, rows[0].PickupTime)
	require.Equal(t, "15:45", rows[0].PickupTime.Format("15:04"))
	require.Equal(t, staff.ID, rows[0].CreatedBy)
	require.Equal(t, "staff", rows[0].Source)
	require.Equal(t, []string{"student-day-lock", "staff", "sync"}, probe.calls)
	_, err = conflicts.ConflictCandidate(ctx, request.ID)
	require.ErrorIs(t, err, careplan.ErrCareScheduleRequestNotPending)
}
