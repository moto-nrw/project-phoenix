package contracttest_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartialAbsenceCreate_RefusesPendingFullDayRequest(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	svc, err := compose.NewPartialAbsences(db, repos.CarePlan(), api.NewStudentPresenceForTests(testpkg.NewDatabase(t, db)), nil)
	require.NoError(t, err)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Partial", "Pending")

	day := calendar.NewDate(2026, 8, 24).AddDays(5)
	ctx := testpkg.TenantContext(chain.TenantID)

	req := &absencerecords.ExcusedAbsenceRequest{
		StudentID:   chain.StudentID,
		SubmittedBy: chain.AccountID,
		Dates:       []calendar.Date{day},
		Note:        "Familienfeier",
		Status:      absencerecords.ExcusedRequestStatusPending,
	}
	req.TenantID = chain.TenantID
	require.NoError(t, repos.ExcusedAbsenceRequest.Create(ctx, req))

	_, err = svc.CreatePartialAbsence(ctx, careplan.PartialAbsenceInput{
		StudentID: chain.StudentID,
		Date:      day,
		FromTime:  calendar.NormalizeWallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC)),
		Reason:    "Arzttermin",
		StaffID:   staff.ID,
	})
	require.ErrorIs(t, err, careplan.ErrPartialAbsencePendingRequestConflict)

	rows, listErr := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupException.FindByStudentIDAndDateRange(ctx, chain.StudentID, careplan.Date(day), careplan.Date(day))
	require.NoError(t, listErr)
	assert.Empty(t, rows, "pending full-day request must block partial-absence creation")
}
