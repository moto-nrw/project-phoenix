// Router-level tests for the Schulverwaltung's absence reports (#4791): a
// child the Lehrkraft marked absent without a report shows up as
// "fehlt ohne Meldung"; the holder of class_day:absence_report_write enters
// sick or excused for today through the OGS's status-day write, which goes
// into the child's change history, and takes only such a report back.
//
// "Today" is the pinned far-future Monday of the morning attendance tests.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const absenceReportPermission = "class_day:absence_report_write"

type absenceReportRow struct {
	StudentID      int64  `json:"student_id"`
	Status         string `json:"status"`
	Unreported     bool   `json:"unreported"`
	StatusBySchool bool   `json:"status_by_school"`
}

// absenceRows reads the class view and returns its rows by child and the
// report verdict.
func (f *morningAttendanceFixture) absenceRows(t *testing.T, perms ...string) (bool, map[int64]absenceReportRow) {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/?class="+f.class+"&date="+morningMonday, "", perms...)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data struct {
			CanReport bool               `json:"can_report_absences"`
			Rows      []absenceReportRow `json:"rows"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	rows := map[int64]absenceReportRow{}
	for _, row := range envelope.Data.Rows {
		rows[row.StudentID] = row
	}
	return envelope.Data.CanReport, rows
}

func (f *morningAttendanceFixture) report4791(t *testing.T, method, date string, studentID int64, body string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, method, fmt.Sprintf("/absence-reports/%s/%s/students/%d", f.class, date, studentID), body, perms...)
}

// markAbsent marks the child without a report and the sick child "fehlt".
func (f *morningAttendanceFixture) markAbsent(t *testing.T) {
	t.Helper()
	rec := f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"absent"},{"student_id":%d,"status":"absent"},{"student_id":%d,"status":"present"}]}`,
		f.present, f.sick, f.other), morningAttendancePermission)
	if rec.Code == http.StatusConflict {
		// The child of the other class is not on this roster.
		rec = f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"absent"},{"student_id":%d,"status":"absent"}]}`,
			f.present, f.sick), morningAttendancePermission)
	}
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

func TestAbsenceReportRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.markAbsent(t)

	// The child marked absent without a report misses without one; the
	// child the OGS reported sick does not.
	canReport, rows := f.absenceRows(t, classDayReadPermission, absenceReportPermission)
	assert.True(t, canReport)
	assert.True(t, rows[f.present].Unreported)
	assert.False(t, rows[f.sick].Unreported, "a sick report explains the morning")

	rec := f.report4791(t, http.MethodPut, morningMonday, f.present, `{"status":"excused"}`, absenceReportPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// The OGS reads an ordinary excused day; the school's source tells who
	// may take it back.
	_, rows = f.absenceRows(t, classDayReadPermission, absenceReportPermission)
	assert.Equal(t, "excused", rows[f.present].Status)
	assert.True(t, rows[f.present].StatusBySchool)
	assert.False(t, rows[f.present].Unreported)
	var source string
	require.NoError(t, f.db.NewRaw(`SELECT source FROM active.student_status_days
		WHERE student_id = ? AND date = ? AND cleared_at IS NULL`, f.present, morningMonday).Scan(testpkg.Ctx(t), &source))
	assert.Equal(t, "school", source)

	// A second report finds nobody missing.
	rec = f.report4791(t, http.MethodPut, morningMonday, f.present, `{"status":"sick"}`, absenceReportPermission)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.absence_report_not_missing")

	// The withdrawal clears the school's day; the child misses again.
	rec = f.report4791(t, http.MethodDelete, morningMonday, f.present, "", absenceReportPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	_, rows = f.absenceRows(t, classDayReadPermission, absenceReportPermission)
	assert.Empty(t, rows[f.present].Status)
	assert.True(t, rows[f.present].Unreported)
	rec = f.report4791(t, http.MethodDelete, morningMonday, f.present, "", absenceReportPermission)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.absence_report_not_found")

	// Both steps are in the child's change history, with the actor.
	var logged []struct {
		OldValue *string `bun:"old_value"`
		NewValue *string `bun:"new_value"`
		Actor    int64   `bun:"edited_by"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT old_value, new_value, edited_by FROM audit.student_field_edits
		WHERE student_id = ? AND field_name = 'school_absence_report' ORDER BY id`, f.present).Scan(testpkg.Ctx(t), &logged))
	require.Len(t, logged, 2)
	assert.Equal(t, int64(f.claims.ID), logged[0].Actor)
	require.NotNil(t, logged[0].NewValue)
	assert.Contains(t, *logged[0].NewValue, "Entschuldigt am 02.03.2099")
	require.NotNil(t, logged[1].OldValue)
	assert.Contains(t, *logged[1].OldValue, "Entschuldigt am 02.03.2099")
	require.NotNil(t, logged[1].NewValue)
	assert.Contains(t, *logged[1].NewValue, "Fehlt ohne Meldung")
}

func TestAbsenceReportKeepsTheReportsOfOthers(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.markAbsent(t)

	// The sick day the OGS entered is no report of the school: neither
	// reported over nor taken back.
	rec := f.report4791(t, http.MethodPut, morningMonday, f.sick, `{"status":"excused"}`, absenceReportPermission)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	rec = f.report4791(t, http.MethodDelete, morningMonday, f.sick, "", absenceReportPermission)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	_, rows := f.absenceRows(t, classDayReadPermission)
	assert.Equal(t, "sick", rows[f.sick].Status)
	assert.False(t, rows[f.sick].StatusBySchool)
}

func TestAbsenceReportGates(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.markAbsent(t)

	// A Lehrkraft without the permission neither reports nor withdraws, and
	// her view offers no report.
	rec := f.report4791(t, http.MethodPut, morningMonday, f.present, `{"status":"sick"}`, classDayReadPermission, morningAttendancePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = f.report4791(t, http.MethodDelete, morningMonday, f.present, "", classDayReadPermission, morningAttendancePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	canReport, rows := f.absenceRows(t, classDayReadPermission, morningAttendancePermission)
	assert.False(t, canReport)
	assert.True(t, rows[f.present].Unreported, "the Lehrkraft still sees who misses in her class")

	// Only today, only sick or excused, only a child of the class, only a
	// child marked absent.
	rec = f.report4791(t, http.MethodPut, morningTuesday, f.present, `{"status":"sick"}`, absenceReportPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.absence_report_not_today")
	rec = f.report4791(t, http.MethodPut, morningMonday, f.present, `{"status":"class_trip"}`, absenceReportPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = f.report4791(t, http.MethodPut, morningMonday, f.other, `{"status":"sick"}`, absenceReportPermission)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.absence_report_unknown_child")
	rec = f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = f.report4791(t, http.MethodPut, morningMonday, f.present, `{"status":"sick"}`, absenceReportPermission)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.absence_report_not_missing")
}

func TestMorningAttendanceTellsTheSchoolWhoMissesWithoutReport(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)

	// A mark without a missing child tells nobody.
	rec := f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"},{"student_id":%d,"status":"absent"}]}`,
		f.present, f.sick), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Empty(t, f.module.Unreported.Reports, "the sick child has a report")

	f.markAbsent(t)
	require.NotEmpty(t, f.module.Unreported.Reports)
	report := f.module.Unreported.Reports[len(f.module.Unreported.Reports)-1]
	assert.Equal(t, f.tenantID, report.TenantID)
	assert.Equal(t, morningMonday, report.Date)
	assert.Contains(t, report.StudentIDs, f.present)
	assert.NotContains(t, report.StudentIDs, f.sick)
	assert.Equal(t, int64(f.claims.ID), report.ActorAccountID, "the Lehrkraft who marked is spared")
}
