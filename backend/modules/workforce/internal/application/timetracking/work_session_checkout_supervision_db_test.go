package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingSupervisions refuses the supervision lookup with a database error
// inside the checkout's transaction. In PostgreSQL such an error aborts the
// whole transaction unless a savepoint contains it.
type failingSupervisions struct {
	timetracking.GroupSupervisorRepository
	calls int
}

func (f *failingSupervisions) FindActiveByStaffID(ctx context.Context, _ int64) ([]*timetracking.GroupSupervisor, error) {
	f.calls++
	return nil, testpkg.FailInCurrentTransaction(ctx)
}

// TestAutoCheckout_SupervisionEndFailureKeepsTheCheckout pins ZE-5 of
// #6288: ending the open supervisions on checkout is best effort, but its
// failure used to abort the checkout's transaction, so the checkout itself
// and its audit entry rolled back.
func TestAutoCheckout_SupervisionEndFailureKeepsTheCheckout(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
	staff := testpkg.CreateTestStaff(t, db, "Aufsicht", "Endet")
	session := &timetracking.WorkSession{
		StaffID: staff.ID, Date: autoCheckoutDay,
		Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
		CheckInTime: autoCheckoutDay.BerlinMidnight().Add(8 * time.Hour), CreatedBy: staff.ID,
	}
	session.SetTenantID(testpkg.Tenant(t))
	createSessionFixture(t, workTime, ctx, session)
	testpkg.CreateTestStaffShift(t, db, staff.ID, autoCheckoutDay, testpkg.StaffShiftOpts{})

	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	supervisions := &failingSupervisions{GroupSupervisorRepository: reposPresence.GroupSupervisor}
	service := newWorkSessionTestService(t, db, workTime, workTime, supervisions)
	var closed int
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		var runErr error
		closed, runErr = service.AutoCheckoutDueSessions(txCtx, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err, "a failed supervision end must not fail the checkout")
	require.Equal(t, 1, supervisions.calls)
	assert.Equal(t, 1, closed)
	stored, err := workTime.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID})
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.NotNil(t, stored[0].CheckOutTime, "the checkout commits")
}

// newWorkSessionTestService composes the work-session service over the test
// database with replaceable break records and supervisions.
func newWorkSessionTestService(t *testing.T, db *testpkg.DB, workTime workforceCompose.WorkTime, breaks timetracking.WorkSessionBreakRecords, supervisions timetracking.GroupSupervisorRepository) timetracking.WorkSessionService {
	t.Helper()
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposMembership := api.NewSchoolMembershipSuiteRepositories(testpkg.NewDatabase(t, db))
	reposAbsences := api.NewStaffAbsenceSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	return timetracking.NewWorkSessionService(
		workTime, breaks, owners.NewWorkSessionAudit(owners.NewTestAuditPersistence(testpkg.MustDatabase(db), nil).WorkSessionEdit), reposAbsences.StaffAbsence,
		supervisions, reposPresence.ActiveGroup, owners.WorkSessionStaff(reposPeople.Staff, api.MustNewStaffEmployment(testpkg.MustDatabase(db)), reposMembership.Membership), owners.NewWorkSessionSchedules(owners.NewStaffWorkScheduleRepository(workTime)), owners.NewWorkSessionTimeModels(owners.NewWorkTimeModelRepository(workTime)),
		snapshotSessionSettings{}, nil, db, owners.RenderTimeTrackingPDF,
		owners.RenderTimeTrackingWorkbook,
		timetracking.WithWorkSessionShifts(workforceCompose.NewTimeTrackingShifts(workTime)),
	)
}
