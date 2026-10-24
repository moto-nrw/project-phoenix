package schoolcalendar_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicCalendarContractPreservesCalendarDaysAndStoredReason(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSchoolCalendar(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	day := testpkg.Date(2026, 8, 24).String()
	row, err := module.CreateClosingDay(testpkg.Ctx(t), schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: day, EndDate: day, Reason: "  Fortbildung  "}})
	require.NoError(t, err)
	require.Positive(t, row.ID)
	require.Equal(t, day, row.StartDate)
	require.Equal(t, day, row.EndDate)
	require.Equal(t, "  Fortbildung  ", row.Reason)
	found, err := module.FindClosingDay(testpkg.Ctx(t), row.ID)
	require.NoError(t, err)
	require.Equal(t, row, found)
	require.NoError(t, module.DeleteClosingDay(testpkg.Ctx(t), row.ID))
	_, err = module.FindClosingDay(testpkg.Ctx(t), row.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrClosingDayNotFound)
}
