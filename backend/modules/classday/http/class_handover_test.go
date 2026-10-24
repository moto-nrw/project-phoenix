// Router-level tests for the class handover of moto schule (#4794): after
// lessons a Lehrkraft hands an assigned class over to the OGS and marks the
// children who are not there, gated by permission, the class assignment, the
// school's write scope, today and the school day; she may take it back the
// same day. The handover never changes the morning attendance.
//
// "Today" is the fixed far-future Monday of the morning attendance tests.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const classHandoverPermission = "class_day:handover_write"

// openHandover ticks the class handover in the school's write scope.
func (f *morningAttendanceFixture) openHandover(t *testing.T) {
	t.Helper()
	_, err := f.db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, '["class_handover"]'::jsonb)
		 ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
		f.tenantID, schoolPortalWriteScopeKey)
	require.NoError(t, err)
}

func (f *morningAttendanceFixture) handOver(t *testing.T, date, body string, perms ...string) int {
	t.Helper()
	rec := f.do(t, http.MethodPut, fmt.Sprintf("/handover/%s/%s", f.class, date), body, perms...)
	return rec.Code
}

func (f *morningAttendanceFixture) withdrawHandover(t *testing.T, date string, perms ...string) int {
	t.Helper()
	rec := f.do(t, http.MethodDelete, fmt.Sprintf("/handover/%s/%s", f.class, date), "", perms...)
	return rec.Code
}

type handoverReport struct {
	CanHandOver bool `json:"can_hand_over"`
	Handover    *struct {
		HandedOverAt time.Time `json:"handed_over_at"`
		HandedOverBy string    `json:"handed_over_by"`
		AbsentCount  int       `json:"absent_count"`
	} `json:"handover"`
	Rows []struct {
		StudentID      int64 `json:"student_id"`
		HandoverAbsent bool  `json:"handover_absent"`
	} `json:"rows"`
}

func (f *morningAttendanceFixture) handoverReport(t *testing.T, date string, perms ...string) handoverReport {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/?class="+f.class+"&date="+date, "", perms...)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data handoverReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func (r handoverReport) absent(studentID int64) bool {
	for _, row := range r.Rows {
		if row.StudentID == studentID {
			return row.HandoverAbsent
		}
	}
	return false
}

func TestClassHandoverRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.openHandover(t)

	report := f.handoverReport(t, morningMonday, classDayReadPermission, classHandoverPermission)
	assert.True(t, report.CanHandOver)
	assert.Nil(t, report.Handover, "not handed over yet")

	before := time.Now()
	code := f.handOver(t, morningMonday, fmt.Sprintf(`{"present_student_ids":[%d],"absent_student_ids":[%d]}`, f.sick, f.present), classHandoverPermission)
	require.Equal(t, http.StatusNoContent, code)
	var stored []struct {
		StudentID int64 `bun:"student_id"`
		Absent    bool  `bun:"absent"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT student_id, absent FROM active.class_handover_children WHERE tenant_id = ? ORDER BY absent`, f.tenantID).
		Scan(testpkg.Ctx(t), &stored))
	require.Len(t, stored, 2, "every handed-over child is stored, da and nicht da")
	assert.Equal(t, f.sick, stored[0].StudentID)
	assert.False(t, stored[0].Absent)
	assert.Equal(t, f.present, stored[1].StudentID)
	assert.True(t, stored[1].Absent)

	report = f.handoverReport(t, morningMonday, classDayReadPermission, classHandoverPermission)
	require.NotNil(t, report.Handover)
	assert.Equal(t, "Morgen", report.Handover.HandedOverBy[:len("Morgen")], "names the Lehrkraft")
	assert.Equal(t, 1, report.Handover.AbsentCount)
	assert.False(t, report.Handover.HandedOverAt.Before(before.Add(-time.Second)), "the moment is the write")
	assert.True(t, report.absent(f.present))
	assert.False(t, report.absent(f.sick))

	// A second handover the same day replaces the first.
	code = f.handOver(t, morningMonday, `{"absent_student_ids":[]}`, classHandoverPermission)
	require.Equal(t, http.StatusNoContent, code)
	report = f.handoverReport(t, morningMonday, classDayReadPermission)
	require.NotNil(t, report.Handover)
	assert.Zero(t, report.Handover.AbsentCount)
	assert.False(t, report.absent(f.present))
	var handovers int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM active.class_handovers WHERE tenant_id = ?`, f.tenantID).
		Scan(testpkg.Ctx(t), &handovers))
	assert.Equal(t, 1, handovers, "one handover per class and day")

	// The withdrawal deletes it; a second one finds nothing.
	require.Equal(t, http.StatusNoContent, f.withdrawHandover(t, morningMonday, classHandoverPermission))
	assert.Nil(t, f.handoverReport(t, morningMonday, classDayReadPermission).Handover)
	rec := f.do(t, http.MethodDelete, fmt.Sprintf("/handover/%s/%s", f.class, morningMonday), "", classHandoverPermission)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.class_handover_not_found")
}

func TestClassHandoverLeavesMorningAttendanceUntouched(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.openHandover(t)
	rec := f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	require.Equal(t, http.StatusNoContent,
		f.handOver(t, morningMonday, fmt.Sprintf(`{"absent_student_ids":[%d]}`, f.present), classHandoverPermission))

	_, statuses := f.report(t, morningMonday, classDayReadPermission)
	assert.Equal(t, "present/teacher", statuses[fmt.Sprintf("s%d", f.present)],
		"\"nicht da\" at the handover is a moment of its own and keeps the morning mark")
}

func TestClassHandoverGates(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	body := fmt.Sprintf(`{"absent_student_ids":[%d]}`, f.present)

	// The write scope does not tick the handover yet.
	rec := f.do(t, http.MethodPut, fmt.Sprintf("/handover/%s/%s", f.class, morningMonday), body, classHandoverPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.school_write_disabled")
	assert.False(t, f.handoverReport(t, morningMonday, classDayReadPermission, classHandoverPermission).CanHandOver)
	f.openHandover(t)

	// The permission gate.
	assert.Equal(t, http.StatusForbidden, f.handOver(t, morningMonday, body, classDayReadPermission))
	assert.False(t, f.handoverReport(t, morningMonday, classDayReadPermission).CanHandOver, "the verdict needs the permission")
	assert.Equal(t, http.StatusForbidden, f.withdrawHandover(t, morningMonday, classDayReadPermission))

	// Only today, for the handover and the withdrawal.
	rec = f.do(t, http.MethodPut, fmt.Sprintf("/handover/%s/%s", f.class, morningTuesday), body, classHandoverPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.class_handover_not_today")
	assert.Equal(t, http.StatusBadRequest, f.withdrawHandover(t, morningTuesday, classHandoverPermission))
	assert.False(t, f.handoverReport(t, morningTuesday, classDayReadPermission, classHandoverPermission).CanHandOver)

	// Only children with an OGS record on the class roster, da or nicht da.
	rec = f.do(t, http.MethodPut, fmt.Sprintf("/handover/%s/%s", f.class, morningMonday),
		fmt.Sprintf(`{"absent_student_ids":[%d]}`, f.other), classHandoverPermission)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.class_handover_unknown_child")
	assert.Equal(t, http.StatusConflict, f.handOver(t, morningMonday,
		fmt.Sprintf(`{"present_student_ids":[%d]}`, f.other), classHandoverPermission))
	assert.Equal(t, http.StatusBadRequest, f.handOver(t, morningMonday,
		fmt.Sprintf(`{"present_student_ids":[%d],"absent_student_ids":[%d]}`, f.present, f.present), classHandoverPermission),
		"a child is either da or nicht da")
	// List entries number apart from students; skip one whose ID a roster
	// child happens to share.
	entry := f.listEntry
	for entry == f.sick || entry == f.present {
		entry = testpkg.CreateTestClassListEntry(t, f.db, "Lena", "Liste", f.class).ID
	}
	assert.Equal(t, http.StatusConflict, f.handOver(t, morningMonday,
		fmt.Sprintf(`{"absent_student_ids":[%d]}`, entry), classHandoverPermission), "a class-list-only entry does not go to the OGS")

	// Only assigned classes, and well-formed IDs.
	rec = f.do(t, http.MethodPut, "/handover/"+f.class+"x/"+morningMonday, body, classHandoverPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusBadRequest, f.handOver(t, morningMonday, `{"absent_student_ids":[0]}`, classHandoverPermission))

	var handovers int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM active.class_handovers WHERE tenant_id = ?`, f.tenantID).
		Scan(testpkg.Ctx(t), &handovers))
	assert.Zero(t, handovers, "no refused write stored anything")
}

func TestClassHandoverRefusesADayWithoutSchool(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningSaturday)
	f.openHandover(t)

	rec := f.do(t, http.MethodPut, fmt.Sprintf("/handover/%s/%s", f.class, morningSaturday), `{"absent_student_ids":[]}`, classHandoverPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.class_handover_no_school_day")
	assert.False(t, f.handoverReport(t, morningSaturday, classDayReadPermission, classHandoverPermission).CanHandOver)
}
