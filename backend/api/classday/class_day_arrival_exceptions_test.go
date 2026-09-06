// Router-level tests for the one write surface of moto schule (#2970): a
// Lehrkraft sets the class-wide arrival day exception of #2962 for an
// assigned class, gated by permission, the school's setting, the
// class_teachers assignment and the date.
//
// Dates are fixed far-future weekdays (the service refuses past dates), and
// the list is read with an explicit from/to window so nothing here depends
// on the wall clock.
package classday_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const (
	classDayReadPermission  = "class_day:read"
	classDayWritePermission = "class_day:arrival_exception_write"

	schoolPortalWriteScopeKey        = "operations.school_portal_write_scope"
	schoolPortalWriteScopeExceptions = "class_arrival_exceptions"

	// A Monday and the Saturday of the same week, far in the future.
	arrivalExceptionMonday   = "2099-03-02"
	arrivalExceptionTuesday  = "2099-03-03"
	arrivalExceptionSaturday = "2099-03-07"
	arrivalExceptionWindow   = "from=2099-03-01&to=2099-03-31"
)

type arrivalExceptionFixture struct {
	db        *testpkg.DB
	resource  *classday.Resource
	claims    jwt.AppClaims
	staffID   int64
	class     string
	openWrite func(t *testing.T)
}

// setupArrivalExceptionRoute wires the class-day resource with its write
// seam (#2970) the way api/base.go does.
func setupArrivalExceptionRoute(t *testing.T) (*testpkg.DB, *classday.Resource) {
	t.Helper()
	db, factory := testutil.SetupClassDayModule(t)
	return db, classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil,
		classday.WithArrivalExceptions(factory.ClassDayArrivalExceptions))
}

// setupArrivalExceptionFixture builds one Lehrkraft with an assigned class
// that has an active child, on a tenant of its own so the setting toggles
// of parallel tests cannot interfere.
func setupArrivalExceptionFixture(t *testing.T) *arrivalExceptionFixture {
	t.Helper()
	db, resource := setupArrivalExceptionRoute(t)
	tenantID := testpkg.Tenant(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Lehr", fmt.Sprintf("Kraft-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("ae%d", time.Now().UnixNano()%100000)
	_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	_ = testpkg.CreateTestStudent(t, db, "Klara", "Klassentag", className)

	return &arrivalExceptionFixture{
		db:       db,
		resource: resource,
		staffID:  staff.ID,
		class:    className,
		claims: jwt.AppClaims{
			ID: int(account.ID), Sub: account.Email,
			Roles: []string{"lehrkraft"}, TenantID: tenantID,
			Scope: tenant.ScopeSchool,
		},
		// The tenant override the OGS admin would set on the settings page;
		// written as the row itself, since the settings service is not part
		// of this route's surface.
		openWrite: func(t *testing.T) {
			t.Helper()
			_, err := db.ExecContext(testpkg.Ctx(t),
				`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, ?::jsonb)
				 ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
				tenantID, schoolPortalWriteScopeKey, `"`+schoolPortalWriteScopeExceptions+`"`)
			require.NoError(t, err)
		},
	}
}

func (f *arrivalExceptionFixture) do(t *testing.T, method, path, body string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return testutil.ExecuteWithAuthPermissions(t, f.resource.SchoolRouter(), req, f.claims, perms)
}

func (f *arrivalExceptionFixture) listPath() string {
	return "/arrival-exceptions?class=" + f.class + "&" + arrivalExceptionWindow
}

func (f *arrivalExceptionFixture) writePath(date string) string {
	return fmt.Sprintf("/arrival-exceptions/%s/%s", f.class, date)
}

func TestSchoolArrivalExceptionsClosedByDefault(t *testing.T) {
	t.Parallel()
	f := setupArrivalExceptionFixture(t)
	path := f.writePath(arrivalExceptionMonday)
	body := `{"arrival_time":"12:45","reason":"Unterricht fällt aus"}`

	// The classes list says the action is not available.
	rec := f.do(t, http.MethodGet, "/classes", "", classDayReadPermission, classDayWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_write_arrival_exception":false`)

	// Setting none: every write answers 403 with the stable code, even with
	// the permission in the token.
	rec = f.do(t, http.MethodPut, path, body, classDayWritePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "school_write_disabled")

	rec = f.do(t, http.MethodDelete, path, "", classDayWritePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "school_write_disabled")

	rec = f.do(t, http.MethodGet, "/arrival-exceptions/block-start?class="+f.class+"&date="+arrivalExceptionMonday, "", classDayWritePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "school_write_disabled")

	// Reading stays open: what the OGS entered is visible regardless.
	rec = f.do(t, http.MethodGet, f.listPath(), "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_edit":false`)
	assert.Contains(t, rec.Body.String(), `"exceptions":[]`)
}

func TestSchoolArrivalExceptionsRequirePermissionBeforeSetting(t *testing.T) {
	t.Parallel()
	f := setupArrivalExceptionFixture(t)
	f.openWrite(t)
	path := f.writePath(arrivalExceptionMonday)

	// class_day:read alone opens the list but no write.
	rec := f.do(t, http.MethodPut, path, `{"arrival_time":"12:45"}`, classDayReadPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "school_write_disabled", "the permission gate answers before the setting")

	rec = f.do(t, http.MethodDelete, path, "", classDayReadPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// The classes flag needs the permission too.
	rec = f.do(t, http.MethodGet, "/classes", "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_write_arrival_exception":false`)

	rec = f.do(t, http.MethodGet, "/classes", "", classDayReadPermission, classDayWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_write_arrival_exception":true`)
}

func TestSchoolArrivalExceptionsRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupArrivalExceptionFixture(t)
	f.openWrite(t)
	path := f.writePath(arrivalExceptionMonday)

	rec := f.do(t, http.MethodPut, path, `{"arrival_time":"12:45","reason":"Unterricht fällt aus"}`, classDayWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"arrival_time":"12:45"`)
	assert.Contains(t, rec.Body.String(), `"origin":"school"`)
	assert.Contains(t, rec.Body.String(), "Unterricht fällt aus")

	// Attributed to the Lehrkraft's staff row, marked as entered by the
	// school — the shared service wrote it, nothing per child.
	var origin string
	var createdBy int64
	require.NoError(t, f.db.NewSelect().
		TableExpr("education.class_arrival_exceptions").
		ColumnExpr("origin, created_by").
		Where("school_class = ? AND date = ?", f.class, arrivalExceptionMonday).
		Scan(testpkg.Ctx(t), &origin, &createdBy))
	assert.Equal(t, "school", origin)
	assert.Equal(t, f.staffID, createdBy)

	// The list carries it with the edit verdict.
	rec = f.do(t, http.MethodGet, f.listPath(), "", classDayReadPermission, classDayWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_edit":true`)
	assert.Contains(t, rec.Body.String(), `"date":"`+arrivalExceptionMonday+`"`)

	// The class day view shows the class-wide line for that date.
	rec = f.do(t, http.MethodGet, "/?class="+f.class+"&date="+arrivalExceptionMonday, "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"class_arrival_exception":{"arrival_time":"12:45","reason":"Unterricht fällt aus","origin":"school"}`)

	rec = f.do(t, http.MethodDelete, path, "", classDayWritePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	rec = f.do(t, http.MethodGet, f.listPath(), "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"exceptions":[]`)

	rec = f.do(t, http.MethodGet, "/?class="+f.class+"&date="+arrivalExceptionMonday, "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "class_arrival_exception")
}

func TestSchoolArrivalExceptionsRefuseForeignClassAndBadDates(t *testing.T) {
	t.Parallel()
	f := setupArrivalExceptionFixture(t)
	f.openWrite(t)
	body := `{"arrival_time":"12:45"}`

	t.Run("class not assigned", func(t *testing.T) {
		other := fmt.Sprintf("fremd%d", time.Now().UnixNano()%100000)
		_ = testpkg.CreateTestStudent(t, f.db, "Fremd", "Kind", other)
		path := fmt.Sprintf("/arrival-exceptions/%s/%s", other, arrivalExceptionMonday)
		rec := f.do(t, http.MethodPut, path, body, classDayWritePermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "nicht zugewiesen")

		rec = f.do(t, http.MethodGet, "/arrival-exceptions?class="+other, "", classDayReadPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("past date", func(t *testing.T) {
		path := f.writePath("2000-01-03")
		rec := f.do(t, http.MethodPut, path, body, classDayWritePermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "class_arrival_exception_past_date")

		rec = f.do(t, http.MethodDelete, path, "", classDayWritePermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("weekend", func(t *testing.T) {
		rec := f.do(t, http.MethodPut, f.writePath(arrivalExceptionSaturday), body, classDayWritePermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "class_arrival_exception_weekend")
	})

	t.Run("bad date and bad time", func(t *testing.T) {
		rec := f.do(t, http.MethodPut, f.writePath("heute"), body, classDayWritePermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

		rec = f.do(t, http.MethodPut, f.writePath(arrivalExceptionMonday), `{"arrival_time":"12h45"}`, classDayWritePermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

		rec = f.do(t, http.MethodGet, "/arrival-exceptions?class="+f.class+"&from=gestern", "", classDayReadPermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("nothing to delete", func(t *testing.T) {
		rec := f.do(t, http.MethodDelete, f.writePath(arrivalExceptionTuesday), "", classDayWritePermission)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})
}

func TestSchoolArrivalExceptionBlockStart(t *testing.T) {
	t.Parallel()
	f := setupArrivalExceptionFixture(t)
	f.openWrite(t)
	ctx := testpkg.Ctx(t)
	// The instance fixture takes a calendar date; the read side only needs a
	// weekday the class has a block on, so any coming weekday works.
	date := testpkg.TodayDate().AddDays(14)
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDays(1)
	}

	room := testpkg.CreateTestRoom(t, f.db, fmt.Sprintf("Raum-%d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, f.db, fmt.Sprintf("Randstunde-%d", time.Now().UnixNano()))
	_, err := f.db.NewUpdate().TableExpr("activities.groups").
		Set("target_group_type = ?", "klasse").
		Set("target_school_class = ?", strings.ToUpper(f.class)).
		Where("id = ?", group.ID).
		Exec(ctx)
	require.NoError(t, err)
	_ = testpkg.CreateTestActivityInstance(t, f.db, date, room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID, StartHHMM: "12:45", EndHHMM: "14:00"})
	_ = testpkg.CreateTestActivityInstance(t, f.db, date, room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID, StartHHMM: "11:15", EndHHMM: "12:00", Status: "cancelled"})

	rec := f.do(t, http.MethodGet, "/arrival-exceptions/block-start?class="+f.class+"&date="+date.String(), "", classDayWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"start":"12:45"`, "case-insensitive class match, cancelled block skipped")

	// A day without a block answers an empty start, not an error.
	rec = f.do(t, http.MethodGet, "/arrival-exceptions/block-start?class="+f.class+"&date="+arrivalExceptionMonday, "", classDayWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"start":""`)

	rec = f.do(t, http.MethodGet, "/arrival-exceptions/block-start?class="+f.class+"&date=morgen", "", classDayWritePermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
