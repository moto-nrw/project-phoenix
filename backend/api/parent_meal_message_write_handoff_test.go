package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	settingsModule "github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The rows of the invariant table in #6510 one by one: each refusal answers
// with its status and code in its place (before any frame, between the child
// check and the school, or under the child's lock) and stores nothing; each
// success stores its write. Except TestParentMealMessageWritesAreOneTransaction,
// which fails there with two units, these hold on development as well.

// sent is one request with what it ran.
type sent struct {
	answer string
	detail string
	body   string
	stmts  []string
	begins int
}

func (r mealWriteRig) send(t *testing.T, w writeCall, token string) sent {
	t.Helper()
	r.faults.arm(nil)
	rec := r.call(t, context.Background(), w, token)
	events, begins := r.faults.snapshot()
	var stmts []string
	for _, e := range events {
		if e != "BEGIN" && e != "COMMIT" && e != "ROLLBACK" {
			stmts = append(stmts, e)
		}
	}
	var body struct{ Detail string }
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return sent{answer: answer(t, rec), detail: body.Detail, body: rec.Body.String(), stmts: stmts, begins: begins}
}

// reachedSchool reports whether a statement ran under the child's school.
func (s sent) reachedSchool() bool {
	for _, q := range s.stmts {
		if strings.HasPrefix(q, "SELECT set_config('role'") {
			return true
		}
	}
	return false
}

// readSettings reports whether a statement after the child's lock read a
// school setting.
func (s sent) readSettingsAfterLock() bool {
	locked := false
	for _, q := range s.stmts {
		if strings.HasPrefix(q, "WITH class_write_gate") {
			locked = true
			continue
		}
		if locked && strings.Contains(q, `"setting_value"`) {
			return true
		}
	}
	return false
}

func (r mealWriteRig) revokePermission(t *testing.T, f mealWriteFamily, permission string) {
	t.Helper()
	_, err := r.super.NewRaw(`UPDATE auth.guardian_student_access SET permissions = permissions - ? WHERE account_id = ?`, permission, f.AccountID).Exec(context.Background())
	require.NoError(t, err)
}

func (r mealWriteRig) endCare(t *testing.T, f mealWriteFamily) {
	t.Helper()
	from, until := calendar.NewDate(2026, 8, 1), calendar.NewDate(2026, 8, 15)
	testpkg.SetStudentLifecycle(t, r.db, f.StudentID, "active", &from, &until)
}

func (r mealWriteRig) registration(t *testing.T, enabled bool) {
	t.Helper()
	require.NoError(t, r.setSetting(testpkg.Ctx(t), settingsModule.KeyMealRegistrationEnabled, enabled))
}

func (r mealWriteRig) registrationChanges(t *testing.T, f mealWriteFamily) int {
	t.Helper()
	var n int
	require.NoError(t, r.super.NewRaw(`SELECT count(*) FROM schedule.meal_registration_changes WHERE student_id = ?`, f.StudentID).Scan(context.Background(), &n))
	return n
}

var (
	mealWriteSaturday     = calendar.NewDate(2026, 9, 5)
	mealWriteFarSaturday  = calendar.NewDate(2026, 10, 31)
	mealWriteFarFriday    = calendar.NewDate(2026, 10, 30)
	mealWriteLastFriday   = calendar.NewDate(2026, 8, 28)
	mealWriteLastWeekMon  = calendar.NewDate(2026, 8, 24)
	mealWriteTuesday      = calendar.NewDate(2026, 9, 1)
	mealWriteFriday       = calendar.NewDate(2026, 9, 4)
	mealWriteRoutes       = []string{"S", "D", "W", "C", "M"}
	mealWriteParticipates = []string{"S", "D", "W", "C"}
)

func mealWriteCallFor(route string, f mealWriteFamily) writeCall {
	switch route {
	case "S":
		return mealSchedule(f.StudentID, 1, 2, 3)
	case "D":
		return mealDay(f.StudentID, mealWriteWednesday, false)
	case "W":
		return mealWeek(f.StudentID, mealWriteWednesday, false)
	case "C":
		return mealClear(f.StudentID, mealWriteWednesday)
	default:
		return conversation(f.StudentID)
	}
}

// TestParentMealMessageWritesRefuseBeforeTheSchool: refusals of the child
// check and of the Go checks between it and the school answer as before and
// run no statement of the school.
func TestParentMealMessageWritesRefuseBeforeTheSchool(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)
	other := rig.mealFamily(t)
	g := rig.mealFamily(t)
	h := rig.mealFamily(t)
	start := rig.rows(t, f.TenantID, true)

	for _, route := range mealWriteRoutes {
		got := rig.send(t, mealWriteCallFor(route, f), other.token)
		require.Equal(t, "403 care.child_not_linked", got.answer, route)
		require.False(t, got.reachedSchool(), "%s: %q", route, got.stmts)
	}
	// The child check's own refusal comes back as it is, without the
	// conversation's context.
	require.Equal(t, "parent: child not linked to account", rig.send(t, conversation(f.StudentID), other.token).detail)

	cases := []struct {
		name string
		call writeCall
		want string
	}{
		{"D past day", mealDay(f.StudentID, mealWriteLastFriday, true), "400 meals.participation_out_of_range"},
		{"D after the last day", mealDay(f.StudentID, mealWriteFarFriday, true), "400 meals.participation_out_of_range"},
		{"D Saturday after the last day", mealDay(f.StudentID, mealWriteFarSaturday, true), "400 meals.participation_out_of_range"},
		{"C after the last day", mealClear(f.StudentID, mealWriteFarFriday), "400 meals.participation_out_of_range"},
		{"W last week", mealWeek(f.StudentID, mealWriteLastWeekMon, false), "400 meals.participation_out_of_range"},
		{"W after the last week", mealWeek(f.StudentID, mealWriteFarFriday.AddDays(3), false), "400 meals.participation_out_of_range"},
		{"M unknown audience", writeCall{http.MethodGet, fmt.Sprintf("/parent/me/messages/children/%d?audience=elsewhere", f.StudentID), nil}, "400 general.input"},
	}
	for _, c := range cases {
		got := rig.send(t, c.call, f.token)
		require.Equal(t, c.want, got.answer, c.name)
		require.False(t, got.reachedSchool(), "%s: %q", c.name, got.stmts)
	}
	require.Zero(t, rig.send(t, writeCall{http.MethodGet, fmt.Sprintf("/parent/me/messages/children/%d?audience=elsewhere", f.StudentID), nil}, f.token).begins, "the audience is checked before any unit")

	rig.revokePermission(t, f, "parent_portal.meal_participation.manage")
	for _, route := range mealWriteParticipates {
		got := rig.send(t, mealWriteCallFor(route, f), f.token)
		require.Equal(t, "403 meals.participation_manage_denied", got.answer, route)
		require.False(t, got.reachedSchool(), "%s: %q", route, got.stmts)
	}
	// Priority: the missing right comes before the weekend.
	require.Equal(t, "403 meals.participation_manage_denied", rig.send(t, mealDay(f.StudentID, mealWriteSaturday, true), f.token).answer)

	rig.endCare(t, g)
	for _, route := range mealWriteParticipates {
		got := rig.send(t, mealWriteCallFor(route, g), g.token)
		require.Equal(t, "403 care.child_care_ended", got.answer, route)
		require.False(t, got.reachedSchool(), "%s: %q", route, got.stmts)
	}
	// Priority: the ended care comes before invalid weekdays.
	require.Equal(t, "403 care.child_care_ended", rig.send(t, mealSchedule(g.StudentID, 6), g.token).answer)
	// Reading the conversation stays open after the care ended.
	require.Equal(t, "200", rig.send(t, conversation(g.StudentID), g.token).answer)

	// Without parent_portal.access the child lookup itself finds no child.
	rig.revokePermission(t, h, "parent_portal.access")
	for _, route := range mealWriteRoutes {
		got := rig.send(t, mealWriteCallFor(route, h), h.token)
		require.Equal(t, "403 care.child_not_linked", got.answer, route)
		require.False(t, got.reachedSchool(), "%s: %q", route, got.stmts)
	}
	require.True(t, rig.sameRows(f.TenantID, start, 1), "nothing stored")
}

// TestParentMealMessageWritesRefuseUnderTheLock: the meal plan's own checks
// run in the school's part in their order: the child's lock, the input, the
// registration, the route's deadline and calendar.
func TestParentMealMessageWritesRefuseUnderTheLock(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)
	_, err := rig.super.NewRaw(`INSERT INTO schedule.closing_days (tenant_id, start_date, end_date, reason) VALUES (?, ?, ?, 'Studientag')`, f.TenantID, mealWriteThursday, mealWriteThursday).Exec(context.Background())
	require.NoError(t, err)
	start := rig.rows(t, f.TenantID, true)

	cases := []struct {
		name     string
		call     writeCall
		want     string
		settings bool // a school setting is read after the lock
	}{
		{"S invalid weekday", mealSchedule(f.StudentID, 6), "400 meals.invalid_participation", false},
		{"D Saturday", mealDay(f.StudentID, mealWriteSaturday, true), "400 meals.invalid_participation", false},
		{"C Saturday", mealClear(f.StudentID, mealWriteSaturday), "400 meals.invalid_participation", false},
		{"D today after the deadline", mealDay(f.StudentID, childReadToday, false), "409 meals.participation_cutoff_passed", true},
		{"D closed day", mealDay(f.StudentID, mealWriteThursday, false), "409 meals.participation_cutoff_passed", true},
		{"C closed day", mealClear(f.StudentID, mealWriteThursday), "409 meals.participation_cutoff_passed", true},
	}
	for _, c := range cases {
		got := rig.send(t, c.call, f.token)
		require.Equal(t, c.want, got.answer, c.name)
		require.True(t, got.reachedSchool(), c.name)
		require.Equal(t, c.settings, got.readSettingsAfterLock(), "%s: %q", c.name, got.stmts)
	}
	require.True(t, rig.sameRows(f.TenantID, start, 1), "nothing stored")

	// Only the locked day would change: the deadline has passed.
	for _, day := range []calendar.Date{mealWriteTuesday, mealWriteWednesday, mealWriteThursday, mealWriteFriday} {
		rig.override(t, f, day, false)
	}
	start = rig.rows(t, f.TenantID, true)
	require.Equal(t, "409 meals.participation_cutoff_passed", rig.send(t, mealWeek(f.StudentID, childReadToday, false), f.token).answer)
	require.True(t, rig.sameRows(f.TenantID, start, 1), "nothing stored")

	// Registration off: the input is still checked first, without a
	// setting.
	rig.registration(t, false)
	for _, c := range []struct {
		name string
		call writeCall
		want string
	}{
		{"S invalid weekday", mealSchedule(f.StudentID, 6), "400 meals.invalid_participation"},
		{"D Saturday", mealDay(f.StudentID, mealWriteSaturday, true), "400 meals.invalid_participation"},
		{"C Saturday", mealClear(f.StudentID, mealWriteSaturday), "400 meals.invalid_participation"},
	} {
		got := rig.send(t, c.call, f.token)
		require.Equal(t, c.want, got.answer, c.name)
		require.False(t, got.readSettingsAfterLock(), "%s: %q", c.name, got.stmts)
	}
	for _, route := range mealWriteParticipates {
		require.Equal(t, "403 meals.registration_disabled", rig.send(t, mealWriteCallFor(route, f), f.token).answer, route)
	}
	rig.registration(t, true)
	require.True(t, rig.sameRows(f.TenantID, start, 1), "nothing stored")
}

// TestParentMealMessageWritesStoreTheirWrite: each success stores exactly
// its write; the no-ops store no registration entry.
func TestParentMealMessageWritesStoreTheirWrite(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)

	got := rig.send(t, mealSchedule(f.StudentID, 1, 2, 3), f.token)
	require.Equal(t, "200", got.answer)
	require.Contains(t, got.body, `"effective_from":"2026-09-01"`)
	require.Equal(t, 1, rig.registrationChanges(t, f))

	got = rig.send(t, mealWeek(f.StudentID, childReadToday, false), f.token)
	require.Equal(t, "200", got.answer)
	require.Contains(t, got.body, `"changed":["2026-09-01","2026-09-02"],"locked":["2026-08-31"]`)
	require.Equal(t, 3, rig.registrationChanges(t, f))

	// Nothing left to change but the locked day, which already does not eat.
	rig.override(t, f, childReadToday, false)
	got = rig.send(t, mealWeek(f.StudentID, childReadToday, false), f.token)
	require.Equal(t, "200", got.answer)
	require.Contains(t, got.body, `"changed":[],"locked":[]`)
	require.Equal(t, 3, rig.registrationChanges(t, f))

	require.Equal(t, "200", rig.send(t, mealDay(f.StudentID, mealWriteFriday, true), f.token).answer)
	require.Equal(t, 4, rig.registrationChanges(t, f))
	require.Equal(t, "200", rig.send(t, mealClear(f.StudentID, mealWriteFriday), f.token).answer)
	require.Equal(t, 5, rig.registrationChanges(t, f))
	// Clearing a day without an exception stores no entry.
	require.Equal(t, "200", rig.send(t, mealClear(f.StudentID, mealWriteFriday), f.token).answer)
	require.Equal(t, 5, rig.registrationChanges(t, f))

	// A missing conversation is an empty view; a fault from the school's
	// part on carries the conversation's context.
	got = rig.send(t, conversation(f.StudentID), f.token)
	require.Equal(t, "200", got.answer)
	require.Contains(t, got.body, `"thread_id":""`)
	threadAt := 0
	for i, q := range got.stmts {
		if strings.HasPrefix(q, `SELECT "parent_message_thread"`) {
			threadAt = i + 1
		}
	}
	require.Positive(t, threadAt, "%q", got.stmts)
	rig.faults.arm(func(w *writeFaults) { w.stmtAt, w.stmtKind = threadAt, "plain" })
	rec := rig.call(t, context.Background(), conversation(f.StudentID), f.token)
	var body struct{ Detail string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.True(t, strings.HasPrefix(body.Detail, "parent: get child conversation: "), body.Detail)
}

// TestParentMealMessageWritesPersistNothingWhenCancelled cancels the
// request right after the child's lock (the conversation: after its thread
// lookup): the write answers 500 and stores nothing.
func TestParentMealMessageWritesPersistNothingWhenCancelled(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)
	rig.message(t, f, rig.thread(t, f), true, "Hallo")
	for _, route := range mealWriteRoutes {
		probe := rig.send(t, mealWriteCallFor(route, f), f.token)
		require.Equal(t, "200", probe.answer, route)
		at := 0
		for i, q := range probe.stmts {
			if strings.HasPrefix(q, "WITH class_write_gate") || (route == "M" && strings.HasPrefix(q, `SELECT "parent_message_thread"`)) {
				at = i + 1
				break
			}
		}
		require.Positive(t, at, route)
		// The conversation marks read again only without a cursor.
		_, err := rig.super.NewRaw(`DELETE FROM users.parent_message_reads WHERE tenant_id = ?`, f.TenantID).Exec(context.Background())
		require.NoError(t, err)
		start := rig.rows(t, f.TenantID, false)
		startCmp := rig.rows(t, f.TenantID, true)
		ctx, cancel := context.WithCancel(context.Background())
		rig.faults.arm(func(w *writeFaults) { w.afterAt, w.after = at, cancel })
		rec := rig.call(t, ctx, mealWriteCallFor(route, f), f.token)
		cancel()
		require.Equal(t, http.StatusInternalServerError, rec.Code, "%s: %s", route, rec.Body.String())
		require.True(t, rig.sameRows(f.TenantID, startCmp, 1), "%s: nothing stored", route)
		rig.restore(t, f.TenantID, start)
	}
}

// TestParentMealMessageWritesStatementCounts logs the statements of each
// route; it runs unchanged on development for the measurement in #6510.
func TestParentMealMessageWritesStatementCounts(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)
	rig.message(t, f, rig.thread(t, f), true, "Hallo")
	for _, route := range mealWriteRoutes {
		rig.faults.arm(nil)
		rec := rig.call(t, context.Background(), mealWriteCallFor(route, f), f.token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", route, rec.Body.String())
		events, _ := rig.faults.snapshot()
		t.Logf("statements %s: %d", route, len(events))
	}
}

// TestParentMealMessageWritesAreOneTransaction: each route checks the child
// and writes in one transaction. The administrative part locks nothing; the
// first lock follows the switch to the school.
func TestParentMealMessageWritesAreOneTransaction(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)
	rig.message(t, f, rig.thread(t, f), true, "Hallo")
	for _, route := range mealWriteRoutes {
		rig.faults.arm(nil)
		rec := rig.call(t, context.Background(), mealWriteCallFor(route, f), f.token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", route, rec.Body.String())
		events, begins := rig.faults.snapshot()
		require.Equal(t, 1, begins, "%s: %q", route, events)
		require.Equal(t, "BEGIN", events[0], route)
		require.Equal(t, "SET LOCAL ROLE phoenix_admin", events[1], route)
		require.Equal(t, "COMMIT", events[len(events)-1], route)
		require.Equal(t, 1, childStatementCount(events, func(q string) bool { return q == "COMMIT" }), "%s: %q", route, events)
		switchAt := childStatementIndex(events, isChildTenantSwitch)
		require.Positive(t, switchAt, route)
		for _, q := range events[:switchAt] {
			require.False(t, isRowLock(q) || strings.HasPrefix(q, "WITH class_write_gate"), "%s: the administrative part locks nothing: %q", route, q)
		}
		if route != "M" {
			require.Greater(t, childStatementIndex(events, func(q string) bool { return strings.HasPrefix(q, "WITH class_write_gate") }), switchAt, route)
		}
	}
	// A refusal between the parts rolls the one transaction back before
	// any statement of the school.
	rig.endCare(t, f)
	rig.faults.arm(nil)
	rec := rig.call(t, context.Background(), mealWriteCallFor("D", f), f.token)
	require.Equal(t, http.StatusForbidden, rec.Code)
	events, begins := rig.faults.snapshot()
	require.Equal(t, 1, begins)
	require.Equal(t, "ROLLBACK", events[len(events)-1], "%q", events)
	require.Equal(t, -1, childStatementIndex(events, isChildTenantSwitch), "%q", events)
}

// TestParentMealScheduleDropsLaterPatterns: a parent's new pattern deletes
// the regular days set for a later start; a failure after that delete keeps
// them, the success removes them for good.
func TestParentMealScheduleDropsLaterPatterns(t *testing.T) {
	t.Parallel()
	rig := newMealWriteRig(t)
	f := rig.mealFamily(t)
	laterSchedule(t, rig, f)
	starts := func() []string {
		var out []string
		require.NoError(t, rig.super.NewRaw(`SELECT effective_from::text FROM schedule.meal_participation_schedules WHERE student_id = ? ORDER BY effective_from`, f.StudentID).Scan(context.Background(), &out))
		return out
	}
	require.Equal(t, []string{"2026-08-01", "2026-09-14"}, starts())
	start := rig.rows(t, f.TenantID, false)

	probe := rig.send(t, mealSchedule(f.StudentID, 1, 2, 3), f.token)
	require.Equal(t, "200", probe.answer)
	require.Equal(t, []string{"2026-08-01", "2026-09-01"}, starts(), "the later pattern is gone")
	deleteAt, upsertAt := 0, 0
	for i, q := range probe.stmts {
		switch {
		case strings.HasPrefix(q, "DELETE FROM schedule.meal_participation_schedules"):
			deleteAt = i + 1
		case strings.HasPrefix(q, "INSERT INTO schedule.meal_participation_schedules"):
			upsertAt = i + 1
		}
	}
	require.Positive(t, deleteAt)
	require.Equal(t, deleteAt+1, upsertAt, "the new pattern follows the delete")

	// Back to the start; a failure right after the delete keeps the later
	// pattern.
	rig.restore(t, f.TenantID, start)
	for _, kind := range []string{"23514", "40P01", "plain"} {
		rig.faults.arm(func(w *writeFaults) { w.stmtAt, w.stmtKind = upsertAt, kind })
		rec := rig.call(t, context.Background(), mealSchedule(f.StudentID, 1, 2, 3), f.token)
		require.Equal(t, http.StatusInternalServerError, rec.Code, kind)
		require.Eventually(t, func() bool {
			got := starts()
			return len(got) == 2 && got[1] == "2026-09-14"
		}, 10*time.Second, 25*time.Millisecond, "%s: the deleted later pattern is back", kind)
	}
}
