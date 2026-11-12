package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The tablet list (GET /api/iot/students, also the bracelet assignment) shows
// a holiday guest of the hosting school only from the first day of its week
// (#5630, ADR 0120 Nachtrag zu Punkt 2). The school's own child with a later
// start stays. Fixed dates: the waiting guest's week lies in 2099, the arrived
// guest's week runs from 2020 to 2099, so both sides hold on any test day.
func TestTabletRosterShowsAHolidayGuestOnlyFromTheFirstDayOfItsWeek(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)
	device := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-holiday-guest")

	weekStarts, weekEnds := calendar.NewDate(2099, 7, 6), calendar.NewDate(2099, 7, 10)
	waiting := testpkg.CreateTestStudent(t, ctx.db, "Wartet", "Feriengast", "Feriengast")
	testpkg.SetStudentLifecycle(t, ctx.db, waiting.ID, testpkg.StudentStatusPending, &weekStarts, &weekEnds)
	arrivedFrom := calendar.NewDate(2020, 7, 6)
	arrived := testpkg.CreateTestStudent(t, ctx.db, "Da", "Feriengast", "Feriengast")
	testpkg.SetStudentLifecycle(t, ctx.db, arrived.ID, testpkg.StudentStatusActive, &arrivedFrom, &weekEnds)
	ownLater := testpkg.CreateTestStudent(t, ctx.db, "Kommt", "Spaeter", "1a")
	testpkg.SetStudentLifecycle(t, ctx.db, ownLater.ID, testpkg.StudentStatusPending, &weekStarts, nil)

	router := dataTenantRoot(ctx.db, ctx.resource.Router())
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students", nil, testutil.WithDeviceContext(device))
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	data, ok := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	listed := map[int64]bool{}
	for _, item := range data {
		student, _ := item.(map[string]interface{})
		listed[int64(student["student_id"].(float64))] = true
	}
	assert.False(t, listed[waiting.ID], "before its week the holiday guest is not on the tablet")
	assert.True(t, listed[arrived.ID], "during its week the holiday guest is on the tablet")
	assert.True(t, listed[ownLater.ID], "an own child with a later start stays on the tablet as before")
}
