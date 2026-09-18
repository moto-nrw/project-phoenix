package careschedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPartialAbsenceCreate_RefusesPendingFullDayRequest(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := careschedule.NewPartialAbsenceService(
		repos.StudentPickupException,
		repos.StudentStatusDay,
		repos.ExcusedAbsenceRequest,
		repos.InstanceStudent,
		nil,
		db,
	)

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

	_, err := svc.Create(ctx, careschedule.PartialAbsenceInput{
		StudentID: chain.StudentID,
		Date:      day,
		FromTime:  timezone.NormalizeWallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC)),
		Reason:    "Arzttermin",
		StaffID:   staff.ID,
	})
	require.ErrorIs(t, err, careschedule.ErrPartialAbsencePendingRequestConflict)

	rows, listErr := repos.StudentPickupException.FindByStudentIDAndDateRange(ctx, chain.StudentID, scheduleModels.Date(day), scheduleModels.Date(day))
	require.NoError(t, listErr)
	assert.Empty(t, rows, "pending full-day request must block partial-absence creation")
}
