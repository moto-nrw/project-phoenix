package students_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Class-wide arrival day exceptions over HTTP (#2962): admins write, plain
// staff only once operations.class_arrival_exception_editors opens it up,
// and everybody reads the list together with a can_edit verdict.

func classArrivalExceptionWorkday(offset int) timezone.Date {
	date := timezone.TodayDate().AddDays(offset)
	for date.Weekday() == 0 || date.Weekday() == 6 {
		date = date.AddDays(1)
	}
	return date
}

func TestClassArrivalExceptionsAdminRoundTrip(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Klassen", "Kind", "CAE1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Koordination", "Admin")
	claims := testutil.AdminTestClaims(int(account.ID))
	date := classArrivalExceptionWorkday(3).String()
	path := fmt.Sprintf("/class-arrival-exceptions/CAE1/%s", date)

	body := map[string]any{"arrival_time": "12:45", "reason": "Unterricht fällt aus"}
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, body), claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"arrival_time":"12:45"`)
	assert.Contains(t, rr.Body.String(), `"school_class":"CAE1"`)
	assert.Contains(t, rr.Body.String(), "Unterricht fällt aus")

	rr = authExec(t, tc, testutil.NewRequest("GET", "/class-arrival-exceptions/CAE1", nil), claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"can_edit":true`)
	assert.Contains(t, rr.Body.String(), `"exceptions":[{`)
	assert.Contains(t, rr.Body.String(), `"arrival_time":"12:45"`)

	rr = authExec(t, tc, testutil.NewRequest("DELETE", path, nil), claims, []string{"admin:*"})
	assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

	rr = authExec(t, tc, testutil.NewRequest("GET", "/class-arrival-exceptions/CAE1", nil), claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"exceptions":[]`)
}

func TestClassArrivalExceptionsAdminWithoutStaffCreatesUnattributedRow(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Klassen", "Kind", "CAEAdmin")
	account := testpkg.CreateTestAccount(t, tc.db, "arrival-exception-admin@example.com")
	claims := testutil.AdminTestClaims(int(account.ID))
	date := classArrivalExceptionWorkday(1)
	path := fmt.Sprintf("/class-arrival-exceptions/CAEAdmin/%s", date)

	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{"arrival_time": "12:45"}), claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	row := &scheduleModel.ClassArrivalException{}
	require.NoError(t, tc.db.NewSelect().Model(row).
		ModelTableExpr(`education.class_arrival_exceptions AS "class_arrival_exception"`).
		Where(`"class_arrival_exception".school_class = ? AND "class_arrival_exception".date = ?`, "CAEAdmin", date).
		Scan(testpkg.Ctx(t)))
	assert.Nil(t, row.CreatedBy)
}

func TestClassArrivalExceptionsRejectBadInput(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Klassen", "Kind", "CAE2")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Koordination", "Admin")
	claims := testutil.AdminTestClaims(int(account.ID))

	t.Run("past date", func(t *testing.T) {
		path := fmt.Sprintf("/class-arrival-exceptions/CAE2/%s", timezone.TodayDate().AddDays(-1).String())
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{"arrival_time": "12:45"}), claims, []string{"admin:*"})
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "class_arrival_exception_past_date")
	})

	t.Run("unknown class", func(t *testing.T) {
		path := fmt.Sprintf("/class-arrival-exceptions/NOPE9/%s", timezone.TodayDate().String())
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{"arrival_time": "12:45"}), claims, []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("bad time", func(t *testing.T) {
		path := fmt.Sprintf("/class-arrival-exceptions/CAE2/%s", timezone.TodayDate().String())
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{"arrival_time": "12h45"}), claims, []string{"admin:*"})
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("bad date", func(t *testing.T) {
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", "/class-arrival-exceptions/CAE2/heute", map[string]any{"arrival_time": "12:45"}), claims, []string{"admin:*"})
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("weekend", func(t *testing.T) {
		date := classArrivalExceptionWorkday(0)
		for date.Weekday() != 6 {
			date = date.AddDays(1)
		}
		path := fmt.Sprintf("/class-arrival-exceptions/CAE2/%s", date)
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{"arrival_time": "12:45"}), claims, []string{"admin:*"})
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "class_arrival_exception_weekend")
	})

	t.Run("delete missing", func(t *testing.T) {
		path := fmt.Sprintf("/class-arrival-exceptions/CAE2/%s", timezone.TodayDate().AddDays(10).String())
		rr := authExec(t, tc, testutil.NewRequest("DELETE", path, nil), claims, []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
	})
}

func TestClassArrivalExceptionsEditorsSettingGatesStaff(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Klassen", "Kind", "CAE3")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Betreuung", "Kraft")
	claims := testutil.TeacherTestClaims(int(account.ID))
	staffPerms := []string{"users:read", "users:update"}
	date := classArrivalExceptionWorkday(2).String()
	path := fmt.Sprintf("/class-arrival-exceptions/CAE3/%s", date)
	body := map[string]any{"arrival_time": "12:45"}

	// Default: Koordination only. Staff see the list, but may not write.
	rr := authExec(t, tc, testutil.NewRequest("GET", "/class-arrival-exceptions/CAE3", nil), claims, staffPerms)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"can_edit":false`)

	rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, body), claims, staffPerms)
	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "class_arrival_exception_editor_required")

	rr = authExec(t, tc, testutil.NewRequest("DELETE", path, nil), claims, staffPerms)
	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())

	// The school opens it up.
	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.resource.SettingsService.SetValue(
		ctx, configModel.KeyClassArrivalExceptionEditors, configModel.ClassArrivalExceptionEditorsAllStaff, nil, nil,
	))

	rr = authExec(t, tc, testutil.NewRequest("GET", "/class-arrival-exceptions/CAE3", nil), claims, staffPerms)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"can_edit":true`)

	rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, body), claims, staffPerms)
	assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	// users:update stays the floor even with the setting open.
	rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path, body), claims, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
}
