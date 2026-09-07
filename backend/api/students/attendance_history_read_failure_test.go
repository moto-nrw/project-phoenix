package students_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedHistoryVisits struct {
	activeService.StudentHistoryService
	reads int
}

func (s *failedHistoryVisits) GetVisitsByStudentAndTimeRange(context.Context, int64, time.Time, time.Time) ([]*activeService.VisitHistoryEntry, error) {
	s.reads++
	return nil, errors.New("private visit history read failure")
}

func TestAttendanceHistoryRejectsFailedVisitReadAndCanRetry(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	enableAttendanceLog(t, tc)
	student := testpkg.CreateTestStudent(t, tc.db, "History", "Failure", "3a")
	staff := testpkg.CreateTestStaff(t, tc.db, "History", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "history-failure")
	account := testpkg.CreateTestAccount(t, tc.db, "history-failure-reader")
	testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, timezone.Today().Add(8*time.Hour), nil)
	original := tc.resource.StudentHistoryService
	fault := &failedHistoryVisits{StudentHistoryService: original}
	tc.resource.StudentHistoryService = fault
	request := func() *http.Request {
		return testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/attendance-history", student.ID), nil)
	}
	response := authExec(t, tc, request(), testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, 1, fault.reads)
	assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.NotContains(t, response.Body.String(), "private visit history read failure")
	assert.NotContains(t, response.Body.String(), "Attendance history retrieved successfully")
	tc.resource.StudentHistoryService = original
	response = authExec(t, tc, request(), testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"room_detail_available":true`)
}
