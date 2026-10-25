// Router-level tests for the Lehrkraft's day hint for the OGS (#4795): a
// short message for today, for an assigned class or one of its children,
// gated by permission, the school's setting operations.school_portal_write_scope,
// the class_teachers assignment and the day.
//
// The hint is for today only. The projection's clock is fixed to a Monday
// morning in Berlin, so nothing here depends on when the suite runs.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// dayHintToday is the fixed "today" of the projection: arrivalExceptionMonday.
var (
	dayHintToday = calendar.NewDate(2099, time.March, 2)
	dayHintNow   = time.Date(2099, time.March, 2, 10, 0, 0, 0, calendar.Berlin)
)

const (
	classDayDayHintPermission     = "class_day:day_hint_write"
	schoolPortalWriteScopeDayHint = "day_hints"
)

type dayHintFixture struct {
	*arrivalExceptionFixture
	tenantID  int64
	studentID int64
	siblingID int64
	otherID   int64
	today     string
}

// setupDayHintFixture builds one Lehrkraft with an assigned class of three
// children (the base fixture's Klara plus Ida and Bruno), and a child of
// another class.
func setupDayHintFixture(t *testing.T) *dayHintFixture {
	t.Helper()
	base := setupArrivalExceptionFixture(t, func() time.Time { return dayHintNow })
	ida := testpkg.CreateTestStudent(t, base.db, "Ida", "Hinweis", base.class)
	sibling := testpkg.CreateTestStudent(t, base.db, "Bruno", "Banknachbar", base.class)
	other := testpkg.CreateTestStudent(t, base.db, "Olga", "Andereklasse", fmt.Sprintf("x%d", time.Now().UnixNano()%100000))
	return &dayHintFixture{
		arrivalExceptionFixture: base,
		tenantID:                int64(base.claims.TenantID),
		studentID:               ida.ID, siblingID: sibling.ID, otherID: other.ID,
		today: dayHintToday.String(),
	}
}

func (f *dayHintFixture) setScope(t *testing.T, scopes ...string) {
	t.Helper()
	encoded, err := json.Marshal(scopes)
	require.NoError(t, err)
	_, err = f.db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, ?::jsonb)
		 ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
		f.tenantID, schoolPortalWriteScopeKey, string(encoded))
	require.NoError(t, err)
}

func (f *dayHintFixture) classPath(date string) string {
	return fmt.Sprintf("/day-hints/%s/%s", f.class, date)
}

func (f *dayHintFixture) childPath(date string, studentID int64) string {
	return fmt.Sprintf("/day-hints/%s/%s/students/%d", f.class, date, studentID)
}

// dayReport reads today's report of the class as the route serves it.
func (f *dayHintFixture) dayReport(t *testing.T, date string, perms ...string) map[string]any {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/?class="+f.class+"&date="+date, "", perms...)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data, rec.Body.String())
	return envelope.Data
}

func rowHints(t *testing.T, report map[string]any, studentID int64) []any {
	t.Helper()
	rows, _ := report["rows"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if id, _ := row["student_id"].(float64); int64(id) == studentID {
			hints, _ := row["day_hints"].([]any)
			return hints
		}
	}
	t.Fatalf("student %d is not in the report", studentID)
	return nil
}

func (f *dayHintFixture) liveSchoolHints(t *testing.T) int {
	t.Helper()
	var count int
	err := f.db.NewRaw(`SELECT COUNT(*) FROM users.student_notes
		WHERE tenant_id = ? AND origin = 'school' AND deleted_at IS NULL`, f.tenantID).Scan(testpkg.Ctx(t), &count)
	require.NoError(t, err)
	return count
}

func TestSchoolDayHintsClosedByDefault(t *testing.T) {
	t.Parallel()
	f := setupDayHintFixture(t)

	report := f.dayReport(t, f.today, classDayReadPermission, classDayDayHintPermission)
	assert.Equal(t, false, report["can_write_day_hints"], "the empty default keeps moto schule read-only")

	rec := f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"Heute Sportfest"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.school_write_disabled")

	rec = f.do(t, http.MethodDelete, f.childPath(f.today, f.studentID), "", classDayDayHintPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.school_write_disabled")

	// Ticking only the arrival exceptions does not open the day hints: each
	// value of the multi-select opens its own write.
	f.setScope(t, schoolPortalWriteScopeExceptions)
	rec = f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"Heute Sportfest"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.school_write_disabled")
	assert.Zero(t, f.liveSchoolHints(t))
}

func TestSchoolDayHintsRequirePermissionAndAssignment(t *testing.T) {
	t.Parallel()
	f := setupDayHintFixture(t)
	f.setScope(t, schoolPortalWriteScopeDayHint)

	rec := f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"Heute Sportfest"}`, classDayReadPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "classday.school_write_disabled", "the permission gate answers before the setting")

	report := f.dayReport(t, f.today, classDayReadPermission)
	assert.Equal(t, false, report["can_write_day_hints"], "the verdict needs the permission too")

	rec = f.do(t, http.MethodPut, fmt.Sprintf("/day-hints/%s/%s", "fremd", f.today), `{"body":"Heute Sportfest"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Zero(t, f.liveSchoolHints(t))
}

func TestSchoolDayHintsRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupDayHintFixture(t)
	f.setScope(t, schoolPortalWriteScopeExceptions, schoolPortalWriteScopeDayHint)
	perms := []string{classDayReadPermission, classDayDayHintPermission}

	report := f.dayReport(t, f.today, perms...)
	assert.Equal(t, true, report["can_write_day_hints"])

	rec := f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"  Heute Sportfest, bitte Turnbeutel mitgeben  "}`, classDayDayHintPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = f.do(t, http.MethodPut, f.childPath(f.today, f.studentID), `{"body":"Hat Bauchweh, bitte im Blick behalten"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// The class hint is one row per child of the class, the child hint one row.
	assert.Equal(t, 4, f.liveSchoolHints(t))

	report = f.dayReport(t, f.today, perms...)
	classHints, _ := report["class_day_hints"].([]any)
	require.Len(t, classHints, 1, "the class hint shows once, not once per child")
	classHint, _ := classHints[0].(map[string]any)
	assert.Equal(t, "Heute Sportfest, bitte Turnbeutel mitgeben", classHint["body"])
	assert.Equal(t, true, classHint["own"])
	childHints := rowHints(t, report, f.studentID)
	require.Len(t, childHints, 1)
	assert.Equal(t, "Hat Bauchweh, bitte im Blick behalten", childHints[0].(map[string]any)["body"])
	assert.Empty(t, rowHints(t, report, f.siblingID), "the child hint stays with its child")

	// A second write of the same author and target rewords the hint.
	rec = f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"Sportfest fällt aus"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, 4, f.liveSchoolHints(t))
	report = f.dayReport(t, f.today, perms...)
	classHints, _ = report["class_day_hints"].([]any)
	require.Len(t, classHints, 1)
	assert.Equal(t, "Sportfest fällt aus", classHints[0].(map[string]any)["body"])

	rec = f.do(t, http.MethodDelete, f.childPath(f.today, f.studentID), "", classDayDayHintPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = f.do(t, http.MethodDelete, f.childPath(f.today, f.studentID), "", classDayDayHintPermission)
	require.Equal(t, http.StatusNotFound, rec.Code, "a removed hint is gone")
	rec = f.do(t, http.MethodDelete, f.classPath(f.today), "", classDayDayHintPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Zero(t, f.liveSchoolHints(t))

	// Every removal is recorded in the deletion ledger.
	var logged int
	err := f.db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions WHERE tenant_id = ?`, f.tenantID).Scan(testpkg.Ctx(t), &logged)
	require.NoError(t, err)
	assert.Equal(t, 4, logged)
}

func TestSchoolDayHintsRefuseOtherDaysChildrenAndLengths(t *testing.T) {
	t.Parallel()
	f := setupDayHintFixture(t)
	f.setScope(t, schoolPortalWriteScopeDayHint)
	yesterday := dayHintToday.AddDays(-1).String()
	tomorrow := dayHintToday.AddDays(1).String()

	for _, date := range []string{yesterday, tomorrow} {
		rec := f.do(t, http.MethodPut, f.classPath(date), `{"body":"Hinweis"}`, classDayDayHintPermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "classday.day_hint_not_today")
		rec = f.do(t, http.MethodDelete, f.childPath(date, f.studentID), "", classDayDayHintPermission)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "classday.day_hint_not_today")
	}
	report := f.dayReport(t, tomorrow, classDayReadPermission, classDayDayHintPermission)
	assert.Equal(t, false, report["can_write_day_hints"], "another day offers no hint")

	rec := f.do(t, http.MethodPut, f.childPath(f.today, f.otherID), `{"body":"Hinweis"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.day_hint_student_not_in_class")

	rec = f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"   "}`, classDayDayHintPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.day_hint_invalid")

	rec = f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"`+strings.Repeat("ä", 501)+`"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.day_hint_invalid")

	rec = f.do(t, http.MethodPut, f.classPath(f.today), `{"body":"`+strings.Repeat("ä", 500)+`"}`, classDayDayHintPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, "500 characters are allowed: %s", rec.Body.String())
	assert.Zero(t, func() int {
		var other int
		err := f.db.NewRaw(`SELECT COUNT(*) FROM users.student_notes WHERE tenant_id = ? AND student_id = ?`, f.tenantID, f.otherID).Scan(testpkg.Ctx(t), &other)
		require.NoError(t, err)
		return other
	}(), "a class hint never reaches a child of another class")
}
