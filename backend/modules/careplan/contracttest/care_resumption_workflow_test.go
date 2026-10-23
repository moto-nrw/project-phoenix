package contracttest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resumptionAuditFailure struct {
	err    error
	called bool
}

func (a *resumptionAuditFailure) RecordChangesForActor(context.Context, *peopledirectory.StudentState, *peopledirectory.StudentState, int64) error {
	a.called = true
	return a.err
}

func TestCareResumptionWorkflowRollsBackMembershipAndLedgerOnAuditFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	today := calendar.NewDate(2026, 10, 23)
	for _, ambient := range []bool{false, true} {
		t.Run(map[bool]string{false: "own unit", true: "ambient unit"}[ambient], func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			student := testpkg.CreateTestStudent(t, db, "Resume", "Rollback", "2a")
			actor := careActor(t, db)
			records := carePlanRecords(db)
			ended := today.AddDays(-1)
			_, err := db.NewUpdate().TableExpr("users.student_school_memberships").
				Set("enrolled_until = ?", ended).Set("status = ?", peopledirectory.StudentStatusInactive).
				Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(ctx)
			require.NoError(t, err)
			require.NoError(t, records.UpsertCareExit(ctx, careplan.CareExit{
				StudentID: student.ID, Reason: careplan.CareExitReasonMovedAway, RecordedBy: &actor,
			}))
			// A real ledger row must survive a failure after both ledger deletes.
			room := testpkg.CreateTestRoom(t, db, "Resume ledger")
			instance := testpkg.CreateTestActivityInstance(t, db, today, room.ID, testpkg.ActivityInstanceOpts{})
			status := "expected"
			require.NoError(t, records.RecordCareExitRemovals(ctx, []careplan.CareExitRemoval{{
				StudentID: student.ID, Kind: careplan.CareExitRemovalRoster, InstanceID: &instance.ID, Status: &status,
			}}))
			before := loadStudent(t, db, ctx, student.ID)
			ledger, err := records.ListCareExitRemovals(ctx, []int64{student.ID})
			require.NoError(t, err)
			require.NotEmpty(t, ledger)
			repos, err := api.NewCareLifecycleTestRepositories(testpkg.NewDatabase(t, db), nil)
			require.NoError(t, err)
			failure := &resumptionAuditFailure{err: errors.New("resumption audit unavailable")}
			svc := repos.NewCareResumption(api.CareLifecycleTestConfig{
				Audit: failure, Today: func() calendar.Date { return today },
				BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil },
			})
			input := careplan.CareResumeInput{StudentID: student.ID, NewStart: today, Checked: true, ActorAccountID: actor}
			if ambient {
				err = testpkg.TenantCommand1(t, ctx, db, svc, input)
			} else {
				err = svc(testpkg.WithTenantRuntime(t, ctx, db), input)
			}
			require.ErrorIs(t, err, failure.err)
			require.True(t, failure.called, "failure occurs after membership and both Care Plan deletes")
			after := loadStudent(t, db, ctx, student.ID)
			assert.Equal(t, before.EnrolledFrom, after.EnrolledFrom)
			assert.Equal(t, before.EnrolledUntil, after.EnrolledUntil)
			assert.Equal(t, before.Status, after.Status)
			exits, err := records.FindCareExits(ctx, []int64{student.ID})
			require.NoError(t, err)
			assert.Contains(t, exits, student.ID)
			afterLedger, err := records.ListCareExitRemovals(ctx, []int64{student.ID})
			require.NoError(t, err)
			assert.Equal(t, ledger, afterLedger)
		})
	}
}

func TestCareResumptionWorkflowHidesForeignStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	testpkg.OwnTenantRows(t, db, otherTenant)
	student := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Other", "School", "2a")
	today := calendar.NewDate(2026, 10, 23)
	resume := newCareResumptionAt(t, db, func() calendar.Date { return today })
	err := testpkg.TenantCommand1(t, ctx, db, resume, careplan.CareResumeInput{
		StudentID: student.ID, NewStart: today, Checked: true, ActorAccountID: careActor(t, db),
	})
	require.ErrorIs(t, err, careplan.ErrCareResumeUnknownStudent)
}
