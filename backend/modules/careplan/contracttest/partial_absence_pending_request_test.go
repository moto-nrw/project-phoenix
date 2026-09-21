package contracttest_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartialAbsenceCreate_RefusesPendingFullDayRequest(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	timetableDeps := repositories.NewUnobservedTimetableDependencies(db)
	repos := repositories.NewFactory(db, timetableDeps)
	svc, err := compose.NewPartialAbsences(db, repos.CarePlan(), timetableDeps.Capability, nil)
	require.NoError(t, err)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Partial", "Pending")

	day := timezone.NewDate(2026, 8, 24).AddDays(5)
	ctx := testpkg.TenantContext(chain.TenantID)

	req := &activeModels.ExcusedAbsenceRequest{
		StudentID:   chain.StudentID,
		SubmittedBy: chain.AccountID,
		Dates:       []timezone.Date{day},
		Note:        "Familienfeier",
		Status:      activeModels.ExcusedRequestStatusPending,
	}
	req.SetTenantID(chain.TenantID)
	require.NoError(t, repos.ExcusedAbsenceRequest.Create(ctx, req))

	_, err = svc.CreatePartialAbsence(ctx, careplan.PartialAbsenceInput{
		StudentID: chain.StudentID,
		Date:      day,
		FromTime:  timezone.NormalizeWallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC)),
		Reason:    "Arzttermin",
		StaffID:   staff.ID,
	})
	require.ErrorIs(t, err, careplan.ErrPartialAbsencePendingRequestConflict)

	rows, listErr := repos.StudentPickupException.FindByStudentIDAndDateRange(ctx, chain.StudentID, scheduleModels.Date(day), scheduleModels.Date(day))
	require.NoError(t, listErr)
	assert.Empty(t, rows, "pending full-day request must block partial-absence creation")
}
