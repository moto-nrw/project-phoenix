// Router-level tests for the absence list of moto schule (#4793): the
// Lehrkraft reads the absence days of her class in a period from the morning
// marks and the status days, the read and every export land in the access
// log, another class stays closed, and the statement count does not grow
// with the children.
//
// "Today" is the pinned far-future Monday of the morning attendance tests.
package classdayhttp_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	classdayhttp "github.com/moto-ogs/project-phoenix/modules/classday/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

type absenceListCounts struct {
	AbsentDays      int `json:"absent_days"`
	ExcusedDays     int `json:"excused_days"`
	UnexcusedDays   int `json:"unexcused_days"`
	NotRecordedDays int `json:"not_recorded_days"`
}

type absenceListBody struct {
	SchoolClass   string `json:"school_class"`
	AvailableFrom string `json:"available_from"`
	Incomplete    bool   `json:"incomplete"`
	From          string `json:"from"`
	To            string `json:"to"`
	CountedUntil  string `json:"counted_until"`
	SchoolDays    int    `json:"school_days"`
	Children      []struct {
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
	// A child scheduled for a later start who came once before it (#6165):
	// reading its recorded care must stay within the budget.
	early := testpkg.CreateTestScheduledStudent(t, db, "Frueh", "Fehlzeit", className, "2099-04-19")
	attended := calendar.Date("2099-02-27")
	checkIn := attended.BerlinMidnight().Add(12 * time.Hour)
	checkOut := checkIn.Add(4 * time.Hour)
	device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("absence-budget-%d", early.ID))
	staff := testpkg.CreateTestStaff(t, db, "Ganztag", fmt.Sprintf("Budget-%d", early.ID))
	testpkg.CreateTestAttendanceForDate(t, db, early.ID, staff.ID, device.ID, attended, checkIn, &checkOut)

	counter := testpkg.CaptureQueries(t, db)
	// Each measurement reads a period of its own; both write one access-log
	// row.
	read := func(from string, children int) int {
		counter.Reset()
		rec := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/school/class-day", router),
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
	querybudgets.AssertQueryBudget(t, "modules.classday.absence_list", counter.Measurement(counter.Queries()))
}

func TestAbsenceListRetentionWindows(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"gdpr.morning_attendance_retention_days", "gdpr.status_day_retention_days"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f := setupMorningAttendanceFixture(t, morningMonday)
			_, err := f.db.ExecContext(testpkg.Ctx(t), `INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, '31'::jsonb)`, f.tenantID, key)
			require.NoError(t, err)
			insertMorningMark(t, f.db, f.tenantID, f.present, "2099-01-29", "absent")
			insertMorningMark(t, f.db, f.tenantID, f.present, "2099-01-30", "absent")
			rec := f.do(t, http.MethodGet, "/absence-list?class="+f.class+"&from=2099-01-26&to="+morningMonday, "", classDayReadPermission)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var envelope struct {
				Data absenceListBody `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
			assert.True(t, envelope.Data.Incomplete)
			assert.Equal(t, "2099-01-30", envelope.Data.AvailableFrom)
			for _, child := range envelope.Data.Children {
				if child.StudentID == f.present {
					assert.Equal(t, 1, child.UnexcusedDays, "only the retained boundary day counts")
				}
			}
		})
	}
}

func TestAbsenceListIncludesEndedCare(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	end := calendar.Date("2099-02-26")
	testpkg.SetStudentLifecycle(t, f.db, f.present, "active", nil, &end)
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-25", "absent")
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-27", "absent")
	rec := f.do(t, http.MethodGet, "/absence-list?class="+f.class+"&from=2099-02-23&to="+morningMonday, "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data absenceListBody `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	counts := map[int64]absenceListCounts{}
	for _, child := range envelope.Data.Children {
		counts[child.StudentID] = child.absenceListCounts
	}
	require.Contains(t, counts, f.present)
	assert.Equal(t, absenceListCounts{AbsentDays: 1, UnexcusedDays: 1, NotRecordedDays: 3}, counts[f.present])
}

// A child whose care ended on Friday gets a return scheduled on Monday for a
// later start (#6041 review). The resumption stores the child as pending with
// the new start and no end, which says nothing about the care before; the
// days the child was cared for must keep their rows and counts.
func TestAbsenceListPreservesCareBeforeAScheduledReturn(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	end := calendar.Date("2099-02-27")
	testpkg.SetStudentLifecycle(t, f.db, f.present, "active", nil, &end)
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-25", "absent")
	_ = testpkg.CreateTestStudentStatusDay(t, f.db, f.present, end, "excused")
	path := "/absence-list?class=" + f.class + "&from=2099-02-23&to=" + morningMonday
	read := func() absenceListBody {
		rec := f.do(t, http.MethodGet, path, "", classDayReadPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data absenceListBody `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		return envelope.Data
	}
	before := read()
	counts := map[int64]absenceListCounts{}
	for _, child := range before.Children {
		counts[child.StudentID] = child.absenceListCounts
	}
	require.Contains(t, counts, f.present)
	require.Equal(t, absenceListCounts{AbsentDays: 2, ExcusedDays: 1, UnexcusedDays: 1, NotRecordedDays: 3}, counts[f.present])

	// What the resumption writes (ResumeStudentCare): the new start, no end,
	// pending until the start day.
	restart := calendar.Date("2099-04-19")
	testpkg.SetStudentLifecycle(t, f.db, f.present, "pending", &restart, nil)
	after := read()
	assert.Equal(t, before.Children, after.Children, "scheduling the return must keep the rows and counts of the earlier care")
	assert.Equal(t, before.Totals, after.Totals)
}

// A new child approved in winter for a care start after today (#6041 review
// 2): the absence list of the running period, read today, must not gain the
// child with "nicht erfasst" days before its start. Rows, totals and the
// export stay what they were before the approval.
func TestAbsenceListLeavesOutAKnownNewChildBeforeItsStart(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-25", "absent")
	path := "/absence-list?class=" + f.class + "&from=2099-02-02&to=" + morningMonday
	read := func() absenceListBody {
		rec := f.do(t, http.MethodGet, path, "", classDayReadPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data absenceListBody `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		return envelope.Data
	}
	export := func() string {
		rec := f.do(t, http.MethodGet, "/absence-list/export?class="+f.class+"&from=2099-02-02&to="+morningMonday+"&format=xlsx", "", classDayReadPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		workbook, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		require.NoError(t, err)
		var content bytes.Buffer
		for _, file := range workbook.File {
			reader, err := file.Open()
			require.NoError(t, err)
			_, err = io.Copy(&content, reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
		}
		return content.String()
	}
	before := read()
	require.NotEmpty(t, before.Children)
	require.NotContains(t, export(), "Februar, Neu")

	child := testpkg.CreateTestScheduledStudent(t, f.db, "Neu", "Februar", f.class, "2099-04-19")
	after := read()
	for _, row := range after.Children {
		assert.NotEqual(t, child.ID, row.StudentID, "a child that starts after today was never cared for in this period")
	}
	assert.Equal(t, before.Children, after.Children)
	assert.Equal(t, before.Totals, after.Totals)
	assert.NotContains(t, export(), "Februar, Neu", "the export leaves the child out too")
}

// A new child approved for a later start comes to the OGS on one day before
// it (#6041 review 3): the completed attendance and the morning mark of that
// day are care that really happened. Read later, the period keeps the child's
// row for that day; a day without such a record still does not count.
func TestAbsenceListKeepsRecordedCareOfAKnownNewChildBeforeItsStart(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	child := testpkg.CreateTestScheduledStudent(t, f.db, "Frueh", "Gekommen", f.class, "2099-04-19")
	attended := calendar.Date("2099-02-27")
	checkIn := attended.BerlinMidnight().Add(12 * time.Hour)
	checkOut := checkIn.Add(4 * time.Hour)
	device := testpkg.CreateTestDevice(t, f.db, fmt.Sprintf("absence-early-%d", child.ID))
	staff := testpkg.CreateTestStaff(t, f.db, "Ganztag", fmt.Sprintf("Frueh-%d", child.ID))
	testpkg.CreateTestAttendanceForDate(t, f.db, child.ID, staff.ID, device.ID, attended, checkIn, &checkOut)
	insertMorningMark(t, f.db, f.tenantID, child.ID, attended.String(), "present")
	insertMorningMark(t, f.db, f.tenantID, child.ID, "2099-02-25", "absent")

	rec := f.do(t, http.MethodGet, "/absence-list?class="+f.class+"&from=2099-02-23&to="+morningMonday, "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data absenceListBody `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	counts := map[int64]absenceListCounts{}
	for _, row := range envelope.Data.Children {
		counts[row.StudentID] = row.absenceListCounts
	}
	require.Contains(t, counts, child.ID, "the day the child was cared for keeps its row")
	assert.Equal(t, absenceListCounts{}, counts[child.ID],
		"only the attended day counts: present there, the absent mark of a day without care stays out")
}

// A new child approved for 3 February is activated by the hourly run on that
// day (#6041 review 3). Read on 4 February, the January list and the list of
// the running half-year stay what they were before the activation: the
// activation is no evidence of care before the start.
func TestAbsenceListKeepsAKnownNewChildOutBeforeItsStartAfterActivation(t *testing.T) {
	t.Parallel()
	const today, start = "2099-02-04", "2099-02-03"
	f := setupMorningAttendanceFixture(t, today)
	child := testpkg.CreateTestScheduledStudent(t, f.db, "Neu", "Februar", f.class, start)
	read := func(query string) absenceListBody {
		t.Helper()
		rec := f.do(t, http.MethodGet, "/absence-list?class="+f.class+query, "", classDayReadPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data absenceListBody `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		return envelope.Data
	}
	const january = "&from=2099-01-01&to=2099-01-31"
	januaryBefore, halfYearBefore := read(january), read("")
	for _, row := range januaryBefore.Children {
		require.NotEqual(t, child.ID, row.StudentID, "the child was not cared for in January")
	}

	startDay, err := calendar.ParseDate(start)
	require.NoError(t, err)
	activationRun := startDay.BerlinMidnight().Add(time.Hour)
	membership, err := rootAPI.NewSchoolMembershipTestModule(testpkg.NewDatabase(t, f.db), func() time.Time { return activationRun })
	require.NoError(t, err)
	moved, err := membership.TransitionStatus(testpkg.Ctx(t), child.ID, "pending", "active")
	require.NoError(t, err)
	require.True(t, moved)

	januaryAfter, halfYearAfter := read(january), read("")
	assert.Equal(t, januaryBefore.Children, januaryAfter.Children, "the activation must not pull the child into January")
	assert.Equal(t, januaryBefore.Totals, januaryAfter.Totals)
	assert.Equal(t, halfYearBefore.Children, halfYearAfter.Children, "the half-year gains no days before the start")
	assert.Equal(t, halfYearBefore.Totals, halfYearAfter.Totals)
}

// A child's care ended on 31 October; on 7 November the school resumes it
// for 1 February (#6073). Read afterwards, the days between the end and the
// resumption stay out of the list, also with an absence entered after the
// end, while a day with recorded attendance in the gap still counts.
func TestAbsenceListLeavesOutTheCareGapOfAResumedChild(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, "2099-11-09")
	child := testpkg.CreateTestStudent(t, f.db, "Wieder", "Aufgenommen", f.class)
	clock := calendar.Date("2099-10-20").BerlinMidnight()
	membership, err := rootAPI.NewSchoolMembershipTestModule(testpkg.NewDatabase(t, f.db), func() time.Time { return clock })
	require.NoError(t, err)
	_, err = membership.EndCare(testpkg.Ctx(t), []int64{child.ID}, "2099-10-31")
	require.NoError(t, err)
	clock = calendar.Date("2099-11-07").BerlinMidnight().Add(10 * time.Hour)
	resumed, err := membership.ResumeCare(testpkg.Ctx(t), child.ID, "2100-02-01", "pending", "2099-11-07")
	require.NoError(t, err)
	require.True(t, resumed)
	insertMorningMark(t, f.db, f.tenantID, child.ID, "2099-11-03", "absent")
	read := func() map[int64]absenceListCounts {
		t.Helper()
		rec := f.do(t, http.MethodGet, "/absence-list?class="+f.class+"&from=2099-11-01&to=2099-11-06", "", classDayReadPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data absenceListBody `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		counts := map[int64]absenceListCounts{}
		for _, row := range envelope.Data.Children {
			counts[row.StudentID] = row.absenceListCounts
		}
		return counts
	}
	require.NotContains(t, read(), child.ID, "the gap between the end and the resumption is no care")

	attended := calendar.Date("2099-11-04")
	checkIn := attended.BerlinMidnight().Add(12 * time.Hour)
	checkOut := checkIn.Add(4 * time.Hour)
	device := testpkg.CreateTestDevice(t, f.db, fmt.Sprintf("absence-gap-%d", child.ID))
	staff := testpkg.CreateTestStaff(t, f.db, "Ganztag", fmt.Sprintf("Luecke-%d", child.ID))
	testpkg.CreateTestAttendanceForDate(t, f.db, child.ID, staff.ID, device.ID, attended, checkIn, &checkOut)
	insertMorningMark(t, f.db, f.tenantID, child.ID, attended.String(), "present")
	counts := read()
	require.Contains(t, counts, child.ID, "a day the child was cared for in the gap keeps its row")
	assert.Equal(t, absenceListCounts{}, counts[child.ID], "only the attended day counts; the absence of a day without care stays out")
}

func TestAbsenceListPreservesDatedAlumni(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	end := calendar.Date("2099-02-26")
	testpkg.SetStudentLifecycle(t, f.db, f.present, "active", nil, &end)
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-25", "absent")
	insertMorningMark(t, f.db, f.tenantID, f.present, "2099-02-27", "absent")
	path := "/absence-list?class=" + f.class + "&from=2099-02-23&to=" + morningMonday
	read := func() absenceListBody {
		rec := f.do(t, http.MethodGet, path, "", classDayReadPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data absenceListBody `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		return envelope.Data
	}
	before := read()
	testpkg.SetStudentLifecycle(t, f.db, f.present, "alumnus", nil, &end)
	after := read()
	require.Equal(t, before.Children, after.Children, "graduation must preserve rows and retained counts before departure")
	assert.Equal(t, before.Totals, after.Totals)
	rec := f.do(t, http.MethodGet, "/absence-list/export?class="+f.class+"&from=2099-02-23&to="+morningMonday+"&format=xlsx", "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	workbook, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)
	var content bytes.Buffer
	for _, file := range workbook.File {
		reader, err := file.Open()
		require.NoError(t, err)
		_, err = io.Copy(&content, reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
	}
	assert.Contains(t, content.String(), "Da, Paul", "the export must retain the graduate's row too")
}
