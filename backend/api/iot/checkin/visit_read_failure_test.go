package checkin_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type unavailableCurrentVisitService struct {
	activeSvc.Service
}

type unavailablePresenceModeService struct {
	activeSvc.Service
}

func (s unavailablePresenceModeService) GetPresenceMode(context.Context) (string, error) {
	return "", errors.New("presence mode unavailable")
}

func (s *unavailableCurrentVisitService) GetStudentCurrentVisitWithRoom(context.Context, int64) (*activeSvc.VisitWithRoom, error) {
	return nil, errors.New("presence lookup unavailable")
}

func TestDeviceCheckin_VisitReadFailureDoesNotCreateVisit(t *testing.T) {
	t.Parallel()
	testDeviceCheckinPresenceReadFailure(t, false)
}

func TestDeviceCheckin_PresenceModeFailureDoesNotCreateVisit(t *testing.T) {
	t.Parallel()
	testDeviceCheckinPresenceReadFailure(t, true)
}

func testDeviceCheckinPresenceReadFailure(t *testing.T, modeFailure bool) {
	t.Helper()
	tc := setupCheckinRoute(t)
	ctx := testpkg.Ctx(t)
	device := testpkg.CreateTestDevice(t, tc.db, "visit-read-failure")
	staff := testpkg.CreateTestStaff(t, tc.db, "Read", "Failure")
	student := testpkg.CreateTestStudent(t, tc.db, "Read", "Failure", "1a")
	card := testpkg.CreateTestRFIDCard(t, tc.db, "VISITREADFAILURE")
	testpkg.LinkRFIDToStudent(t, tc.db, student.PersonID, card.ID)
	room := testpkg.CreateTestRoom(t, tc.db, "Read Failure Room")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Read Failure Activity")
	testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)

	active := tc.resource.ActiveService
	if modeFailure {
		tc.resource.ActiveService = unavailablePresenceModeService{Service: active}
	} else {
		tc.resource.Checkin = tc.newCheckinService(&unavailableCurrentVisitService{Service: active}, tc.resource.UsersService)
	}
	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/", tc.resource.Router())
	request := func() *http.Request {
		return testutil.NewAuthenticatedRequest(t, "POST", "/checkin", map[string]interface{}{
			"student_rfid": card.ID, "action": "checkin", "room_id": room.ID,
		}, testutil.WithDeviceContext(createTestDeviceContext(device)), testutil.WithStaffContext(staff))
	}

	rr := testutil.ExecuteRequest(router, request())
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"status":"error","error":"Internal server error"}`, rr.Body.String())
	visits, err := active.FindVisitsByStudentID(ctx, student.ID)
	require.NoError(t, err)
	assert.Empty(t, visits)
	attendance, err := active.GetStudentAttendanceStatus(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, attendance)
	assert.Equal(t, "not_checked_in", attendance.Status)

	// With the lookup restored, a genuinely missing visit permits the same scan.
	tc.resource.Checkin = tc.newCheckinService(active, tc.resource.UsersService)
	tc.resource.ActiveService = active
	rr = testutil.ExecuteRequest(router, request())
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	visits, err = active.FindVisitsByStudentID(ctx, student.ID)
	require.NoError(t, err)
	assert.Len(t, visits, 1)
}
