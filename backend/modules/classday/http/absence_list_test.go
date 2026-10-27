// Router-level tests for the absence list of moto schule (#4793): the
// Lehrkraft reads the absence days of her class in a period from the morning
// marks and the status days, the read and every export land in the access
// log, another class stays closed, and the statement count does not grow
// with the children.
//
// "Today" is the pinned far-future Monday of the morning attendance tests.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	classdayhttp "github.com/moto-nrw/project-phoenix/modules/classday/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type absenceListCounts struct {
	AbsentDays      int `json:"absent_days"`
	ExcusedDays     int `json:"excused_days"`
	UnexcusedDays   int `json:"unexcused_days"`
	NotRecordedDays int `json:"not_recorded_days"`
}

type absenceListBody struct {
	SchoolClass  string `json:"school_class"`
	From         string `json:"from"`
	To           string `json:"to"`
	CountedUntil string `json:"counted_until"`
	SchoolDays   int    `json:"school_days"`
	Children     []struct {
		StudentID int64 `json:"student_id"`
		absenceListCounts
	} `json:"children"`
	Totals absenceListCounts `json:"totals"`
}

// insertMorningMark stores a past morning mark the way the Lehrkraft's
// write stores it; the write itself only accepts today.
func insertMorningMark(t *testing.T, db *testpkg.DB, tenantID, studentID int64, date, status string) {
	t.Helper()
	_, err := db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO active.morning_attendance (tenant_id, date, student_id, status) VALUES (?, ?, ?, ?)`,
		tenantID, date, studentID, status)
	require.NoError(t, err)
}

func TestAbsenceListAPI(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	day := func(raw string) calendar.Date {
		date, err := calendar.ParseDate(raw)
		require.NoError(t, err)
		return date
	}

	// Today the child without a report is absent and the child reported
	// sick is at school; the week before holds an absent morning excused by
	// a status day and a class trip.
	rec := f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"absent"},{"student_id":%d,"status":"present"}]}`,
		f.present, f.sick), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-27", "absent")
	_ = testpkg.CreateTestStudentStatusDay(t, f.db, f.present, day("2099-02-27"), "excused")
	_ = testpkg.CreateTestStudentStatusDay(t, f.db, f.sick, day("2099-02-26"), "class_trip")
	insertMorningMark(t, f.db, f.tenantID, f.other, "2099-02-27", "absent")

	path := "/absence-list?class=" + f.class + "&from=2099-02-23&to=2099-03-06"
	rec = f.do(t, http.MethodGet, path, "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data absenceListBody `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	list := envelope.Data
	assert.Equal(t, "2099-03-02", list.CountedUntil, "nothing after today counts")
	assert.Equal(t, 6, list.SchoolDays)
	counts := map[int64]absenceListCounts{}
	for _, child := range list.Children {
		counts[child.StudentID] = child.absenceListCounts
	}
	require.Len(t, counts, 2, "the class-list entry and the other class stay out")
	assert.Equal(t, absenceListCounts{AbsentDays: 2, ExcusedDays: 1, UnexcusedDays: 1, NotRecordedDays: 4}, counts[f.present])
	assert.Equal(t, absenceListCounts{NotRecordedDays: 4}, counts[f.sick], "present while sick and the class trip are no absence days")

	// Every read and every export is logged, the export with its format.
	rec = f.do(t, http.MethodGet, "/absence-list/export?class="+f.class+"&from=2099-02-23&to=2099-03-06&format=xlsx", "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "fehlzeiten-ma")
	assert.NotEmpty(t, rec.Body.Bytes())
	rec = f.do(t, http.MethodGet, path, "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var logged []struct {
		ResourceType string `bun:"resource_type"`
		Format       string `bun:"format"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT resource_type, COALESCE(metadata->>'format', '') AS format FROM audit.data_access_log
		WHERE actor_account_id = ? AND resource_type LIKE 'class_absence_list%' ORDER BY id`, f.claims.ID).Scan(testpkg.Ctx(t), &logged))
	require.Len(t, logged, 3)
	assert.Equal(t, "class_absence_list", logged[0].ResourceType)
	assert.Equal(t, "class_absence_list_export", logged[1].ResourceType)
	assert.Equal(t, "xlsx", logged[1].Format)
	assert.Equal(t, "class_absence_list", logged[2].ResourceType, "the second read of the same period is logged again")

	// Another class, a broken period and an unknown format are refused.
	rec = f.do(t, http.MethodGet, "/absence-list?class="+f.class+"x", "", classDayReadPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	for _, period := range []string{"from=2099-03-06&to=2099-02-23", "from=2099-02-30&to=2099-03-06", "from=2099-02-23"} {
		rec = f.do(t, http.MethodGet, "/absence-list?class="+f.class+"&"+period, "", classDayReadPermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, period+": "+rec.Body.String())
		assert.Contains(t, rec.Body.String(), "classday.absence_list_invalid_period", period)
	}
	rec = f.do(t, http.MethodGet, "/absence-list/export?class="+f.class+"&format=docx", "", classDayReadPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = f.do(t, http.MethodGet, path, "")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestAbsenceListQueryBudget reads the list of every class for the
// Schulverwaltung in a constant number of statements, whatever the number of
// children.
func TestAbsenceListQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	today, err := calendar.ParseDate(morningMonday)
	require.NoError(t, err)
	now := today.BerlinMidnight().Add(9 * time.Hour)
	module, err := rootAPI.NewClassDayTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), func() time.Time { return now })
	require.NoError(t, err)
	router := classdayhttp.NewResource(module.ClassDay, nil).SchoolRouter(rootAPI.ProtectedSchoolRoutes)

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Schul", fmt.Sprintf("Verwaltung-%d", time.Now().UnixNano()))
	claims := sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"schulverwaltung"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}
	className := fmt.Sprintf("fz%d", time.Now().UnixNano()%100000)
	addChild := func(i int) {
		child := testpkg.CreateTestStudent(t, db, fmt.Sprintf("Kind%d", i), "Fehlzeit", className)
		insertMorningMark(t, db, testpkg.Tenant(t), child.ID, "2099-02-27", "absent")
		_ = testpkg.CreateTestStudentStatusDay(t, db, child.ID, today, "sick")
	}
	addChild(1)
	addChild(2)

	counter := testpkg.CaptureQueries(t, db)
	// Each measurement reads a period of its own; both write one access-log
	// row.
	read := func(from string, children int) int {
		counter.Reset()
		rec := testutil.ExecuteWithAuthPermissions(t, router,
			httptest.NewRequest(http.MethodGet, "/absence-list?class="+className+"&from="+from+"&to="+morningMonday, nil),
			claims, []string{classDayReadPermission, classDayAllClassesPermission})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"absent_days":%d,`, 2*children))
		return counter.Total()
	}
	small := read("2099-02-23", 2)
	addChild(3)
	addChild(4)
	addChild(5)
	large := read("2099-02-24", 5)
	assert.Equal(t, small, large, "the list must not grow with the number of children")
	testpkg.AssertQueryBudget(t, "modules.classday.absence_list", counter.Queries())
}
