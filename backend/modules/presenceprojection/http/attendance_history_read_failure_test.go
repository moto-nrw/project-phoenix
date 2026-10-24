package presenceprojectionhttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedHistoryVisits struct {
	studentpresence.StudentHistory
	reads int
}

func (s *failedHistoryVisits) GetVisitsByStudentAndTimeRange(context.Context, int64, time.Time, time.Time) ([]*studentpresence.VisitHistoryEntry, error) {
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
	testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, calendar.Today().Add(8*time.Hour), nil)
	original := tc.resource.StudentHistoryService
	fault := &failedHistoryVisits{StudentHistory: original}
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

type failedHistorySettings struct {
	interfaceSettings
	key string
}

type interfaceSettings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveInt(context.Context, string) (int, error)
	ResolveString(context.Context, string) (string, error)
}

func (s failedHistorySettings) ResolveInt(ctx context.Context, key string) (int, error) {
	if key == s.key {
		return 0, errors.New("private visibility settings read failure")
	}
	return s.interfaceSettings.ResolveInt(ctx, key)
}

func (s failedHistorySettings) ResolveBool(ctx context.Context, key string) (bool, error) {
	if key == s.key {
		return false, errors.New("private visibility settings read failure")
	}
	return s.interfaceSettings.ResolveBool(ctx, key)
}

func TestAttendanceHistoryRejectsUnreadableVisibilitySettings(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, calendar.Berlin) })
	enableAttendanceLog(t, tc)
	require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), "gdpr.attendance_visible_days", 3, nil, nil))
	student := testpkg.CreateTestStudent(t, tc.db, "History", "Settings", "3a")
	account := testpkg.CreateTestAccount(t, tc.db, "history-settings-reader")
	for _, key := range []string{"gdpr.attendance_visible_days", "gdpr.room_detail_visible_days", "gdpr.attendance_log_enabled"} {
		routes := []string{"attendance-history"}
		if key != "gdpr.room_detail_visible_days" {
			routes = append(routes, "attendance-history/export?format=xlsx")
		}
		for _, route := range routes {
			t.Run(key+"/"+route, func(t *testing.T) {
				tc.resource.SettingsService = failedHistorySettings{tc.settingsService, key}
				response := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/%s", student.ID, route), nil), testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
				assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
				assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
				assert.NotContains(t, response.Body.String(), `"days"`)
				assert.NotContains(t, response.Body.String(), "private visibility settings read failure")
			})
		}
	}
}

func TestStudentResponsesHideSettingsReadFailures(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Private", "Settings", "3a")
	account := testpkg.CreateTestAccount(t, tc.db, "student-settings-reader")
	tc.resource.SettingsService = failedHistorySettings{tc.settingsService, "operations.student_photos_enabled"}
	for _, path := range []string{"/", fmt.Sprintf("/%d", student.ID)} {
		t.Run(path, func(t *testing.T) {
			response := authExec(t, tc, testutil.NewRequest(http.MethodGet, path, nil), testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), `"code":"general.server"`)
			assert.NotContains(t, response.Body.String(), "private visibility settings read failure")
			assert.NotContains(t, response.Body.String(), `"data"`)
		})
	}
}

func TestStudentWriteResponsesHideSettingsReadFailures(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Private", "Write", "3a")
	tc.resource.SettingsService = failedHistorySettings{tc.settingsService, "operations.student_photos_enabled"}
	for _, kind := range []string{"create", "update"} {
		t.Run(kind, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/students", nil).WithContext(testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t)))
			response := httptest.NewRecorder()
			person := &peopleModule.Person{ID: student.PersonID, FirstName: "Private", LastName: "Write"}
			state := &peopleModule.StudentState{ID: student.ID, PersonID: student.PersonID}
			if kind == "create" {
				tc.resource.RespondCreatedStudent(response, req, state, person)
			} else {
				tc.resource.RespondUpdatedStudent(response, req, student.ID, person, true, false, time.Now())
			}
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), "private visibility settings read failure")
		})
	}
}
