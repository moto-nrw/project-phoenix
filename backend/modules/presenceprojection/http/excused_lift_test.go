package presenceprojectionhttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A response that cannot verify the effective status must fail the update and
// roll its status-day clear back. Otherwise the UI could claim a lift worked
// from a response which still only reflects the legacy live flag (#3854).
func TestUpdateStudent_LiftFailsWhenStatusDayResponseCannotBeVerified(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, 9, 9)
	tc := setupStudentsRoute(t, func() time.Time { return today.BerlinMidnight().Add(10 * time.Hour) })
	student := testpkg.CreateTestStudent(t, tc.db, "Lift", "Verification", "LV1")
	statusDay := testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, today, absencerecords.StudentStatusDayExcused)

	statusDays := tc.resource.StudentStatusDayService
	tc.resource.StudentStatusDayService = statusDayResponseFailure{
		StatusDayReads: statusDays,
		err:            errors.New("status day response lookup failed"),
	}

	lift := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
		map[string]any{"excused": false}), testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, lift.Code, lift.Body.String())

	tc.resource.StudentStatusDayService = statusDays
	rows, err := statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, today, today)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, statusDay.ID, rows[0].ID, "the failed response verification rolls back the lift")
}

type statusDayResponseFailure struct {
	studentpresence.StatusDayReads
	err error
}

func (s statusDayResponseFailure) FindActiveByStudentAndDateRange(
	_ context.Context,
	_ int64,
	_, _ calendar.Date,
) ([]*absencerecords.StudentStatusDay, error) {
	return nil, s.err
}
