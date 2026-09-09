package staffoffboarding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplecompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipcompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablecompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforcecompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

type fixture struct {
	deps       staffoffboarding.Dependencies
	staffID    int64
	personID   int64
	shiftID    int64
	membership *schoolmembership.Module
	workforce  *workforce.Module
	people     *peopledirectory.Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Lifecycle", "Subject")
	actor := testpkg.CreateTestStaff(t, db, "Lifecycle", "Actor")
	account := testpkg.CreateTestAccount(t, db, "offboarding-actor@example.org")
	memberDeps := membershipcompose.Dependencies{DB: db, Observe: func(membershipcompose.Observation) {}}
	membership, err := membershipcompose.New(memberDeps)
	require.NoError(t, err)
	retirement, err := membershipcompose.NewOffboarding(memberDeps)
	require.NoError(t, err)
	people, err := peoplecompose.New(peoplecompose.Dependencies{DB: db, Observe: func(peoplecompose.Observation) {}})
	require.NoError(t, err)
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: db, Observe: func(presencecompose.Observation) {}})
	require.NoError(t, err)
	runtime := testpkg.ConfigRuntime(db)
	workforceDeps := workforcecompose.Dependencies{LockStaffAssignment: func(ctx context.Context, id int64) error {
		_, err := membership.FindStaffForMutation(ctx, id)
		return err
	}, DB: db, AssignedStaffIDs: runtime.AssignedStaffIDs, RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor, Observe: func(workforcecompose.Observation) {}}
	work, err := workforcecompose.New(workforceDeps)
	require.NoError(t, err)
	workOffboarding, err := workforcecompose.NewOffboarding(workforceDeps, func(context.Context, workforce.StaffAbsence, int64) error {
		return errors.New("fixture has no absences")
	})
	require.NoError(t, err)
	timetableOffboarding, err := timetablecompose.NewOffboarding(timetablecompose.OffboardingDependencies{DB: db, Observe: func(timetablecompose.Observation) {}})
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, db, "Offboarding workflow")
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2027, 10, 4), room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	shift, err := work.CreateStaffShift(ctx, workforce.StaffShift{StaffID: staff.ID, Date: "2027-10-04", StartTime: "08:00:00", EndTime: "12:00:00", CreatedBy: actor.ID})
	require.NoError(t, err)
	_, err = membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "4a"})
	require.NoError(t, err)
	deps := staffoffboarding.Dependencies{
		UnitOfWork: tenant.NewTransactionRunner().RunInTx,
		Authorize: func(context.Context) (staffoffboarding.Actor, error) {
			return staffoffboarding.Actor{TenantID: testpkg.Tenant(t), AccountID: account.ID, StaffID: actor.ID, Username: "actor"}, nil
		},
		Today:      func() string { return "2027-10-02" },
		FindStaff:  membership.FindStaff,
		Membership: retirement, Workforce: workOffboarding, Timetable: timetableOffboarding, People: people,
		PreviewAccess: func(context.Context, int64) (staffoffboarding.AccessPreview, error) {
			return staffoffboarding.AccessPreview{}, errors.New("fixture subject has no account")
		},
		ExecuteAccess: func(context.Context, int64, string) (staffoffboarding.AccessResult, error) {
			return staffoffboarding.AccessResult{}, errors.New("fixture subject has no account")
		},
		LockSupervision:    presence.LockStaffSupervision,
		AppendAudit:        func(context.Context, staffoffboarding.Actor, staffoffboarding.Result) error { return nil },
		Cleanup:            func(context.Context, int64) error { return nil },
		GroupAccessChanged: func(context.Context) {},
		Observe:            func(staffoffboarding.Observation) {},
	}
	return fixture{deps: deps, staffID: staff.ID, personID: staff.PersonID, shiftID: shift.ID, membership: membership, workforce: work, people: people}
}

func (f fixture) assertOperational(t *testing.T) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	_, err := f.membership.FindStaff(ctx, f.staffID)
	require.NoError(t, err)
	_, err = f.workforce.FindStaffShift(ctx, f.shiftID)
	require.NoError(t, err)
	assignments, err := f.membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{f.staffID}})
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	preview, err := f.deps.Timetable.Preview(ctx, f.staffID, f.deps.Today())
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Counts.InstanceAssignments, "timetable deletion must roll back with the other owners")
}

func TestWorkflowRollsBackAfterEachOwnerCommand(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"workforce", "timetable", "membership", "audit", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newFixture(t)
			failure := errors.New("injected " + phase + " failure")
			switch phase {
			case "workforce":
				owner := f.deps.Workforce
				f.deps.Workforce = workforce.NewOffboarding(owner.Lock, owner.Preview, func(ctx context.Context, staffID, actorID int64, date, revision string) (workforce.OffboardingCounts, error) {
					result, err := owner.Execute(ctx, staffID, actorID, date, revision)
					if err != nil {
						return result, err
					}
					return result, failure
				})
			case "timetable":
				owner := f.deps.Timetable
				f.deps.Timetable = timetable.NewOffboarding(owner.Preview, func(ctx context.Context, staffID int64, date, revision string) (timetable.OffboardingCounts, error) {
					result, err := owner.Execute(ctx, staffID, date, revision)
					if err != nil {
						return result, err
					}
					return result, failure
				})
			case "membership":
				owner := f.deps.Membership
				f.deps.Membership = schoolmembership.NewOffboarding(owner.Preview, func(ctx context.Context, staffID int64, revision string) (schoolmembership.Retirement, error) {
					result, err := owner.Execute(ctx, staffID, revision)
					if err != nil {
						return result, err
					}
					return result, failure
				})
			case "audit":
				f.deps.AppendAudit = func(context.Context, staffoffboarding.Actor, staffoffboarding.Result) error { return failure }
			case "cleanup":
				f.deps.Cleanup = func(context.Context, int64) error { return failure }
			}
			workflow, err := staffoffboarding.New(f.deps)
			require.NoError(t, err)
			result, err := workflow.Offboard(testpkg.Ctx(t), f.staffID)
			require.ErrorIs(t, err, failure)
			require.Equal(t, staffoffboarding.Result{}, result)
			f.assertOperational(t)
		})
	}
}

func TestWorkflowDetectsDriftThenRetiresIdempotently(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := testpkg.Ctx(t)
	workflow, err := staffoffboarding.New(f.deps)
	require.NoError(t, err)
	preview, err := workflow.Preview(ctx, f.staffID)
	require.NoError(t, err)
	_, err = f.membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: f.staffID, SchoolClass: "4b"})
	require.NoError(t, err)
	_, err = workflow.Execute(ctx, f.staffID, preview.Revision)
	require.ErrorIs(t, err, staffoffboarding.ErrConflict)
	_, err = f.workforce.FindStaffShift(ctx, f.shiftID)
	require.NoError(t, err, "drift must be rejected before deleting another owner's rows")
	preview, err = workflow.Preview(ctx, f.staffID)
	require.NoError(t, err)
	result, err := workflow.Execute(ctx, f.staffID, preview.Revision)
	require.NoError(t, err)
	require.EqualValues(t, 2, result.Membership.ClassAssignments)
	require.EqualValues(t, 1, result.Workforce.Shifts)
	require.EqualValues(t, 1, result.Timetable.InstanceAssignments)
	_, err = f.membership.FindStaff(ctx, f.staffID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	person, err := f.people.FindPerson(ctx, f.personID)
	require.NoError(t, err, "person remains available to retained history")
	require.Equal(t, "Subject", person.LastName)
	result, err = workflow.Execute(ctx, f.staffID, preview.Revision)
	require.NoError(t, err)
	require.Equal(t, staffoffboarding.Result{}, result)
}

func TestWorkflowRequiresAuthorizationBeforeOwnerReads(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.deps.Authorize = func(context.Context) (staffoffboarding.Actor, error) { return staffoffboarding.Actor{}, nil }
	f.deps.FindStaff = func(context.Context, int64) (schoolmembership.Staff, error) {
		t.Fatal("unauthorized request reached owner query")
		return schoolmembership.Staff{}, nil
	}
	workflow, err := staffoffboarding.New(f.deps)
	require.NoError(t, err)
	_, err = workflow.Offboard(testpkg.Ctx(t), f.staffID)
	require.ErrorIs(t, err, staffoffboarding.ErrUnauthorized)
	f.assertOperational(t)
}
