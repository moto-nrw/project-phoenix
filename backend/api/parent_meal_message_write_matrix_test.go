package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The failure matrix of #6510 (Befundklasse 12): every route of S4 with a
// fault at each transaction step, observed over HTTP on a real database.
// The expectations are the behaviour of development before the handoff;
// PHX_S4_BASELINE=1 runs the same matrix there (two units, so the
// one-BEGIN assertion is skipped) as the counter-check.

// baselineRun reports whether the matrix runs on the code before the
// handoff.
func baselineRun() bool { return os.Getenv("PHX_S4_BASELINE") == "1" }

// writeScenario is one route in one starting state, with what its success
// must store, fixed here and not taken from a run.
type writeScenario struct {
	route, name string
	// settings are school settings beyond meal plan, registration and the
	// leadership option.
	settings map[string]any
	// prepare brings the family from its fresh state to the scenario's.
	prepare func(t *testing.T, r mealWriteRig, f mealWriteFamily)
	call    func(f mealWriteFamily) writeCall
	// frame marks the scenarios that also run the frame cells (BEGIN,
	// administrative part, switch, COMMIT).
	frame bool
	// rosterRetry marks a scenario whose order fixes several days at once:
	// its roster and participation reads are batched, and on an error the
	// meal plan reads the day alone again (order_registrations.go).
	rosterRetry bool
	// rows is the change of the row count per table a success makes;
	// broadcasts the read receipts it sends.
	rows       map[string]int
	broadcasts int
}

var (
	mealWriteWednesday = calendar.NewDate(2026, 9, 2)
	mealWriteThursday  = calendar.NewDate(2026, 9, 3)
	mealWriteLaterFrom = calendar.NewDate(2026, 9, 14)
)

// Short names of mealWriteTables for the expected row changes.
const (
	tSchedules = "schedule.meal_participation_schedules"
	tOverrides = "schedule.meal_participation_overrides"
	tLog       = "schedule.meal_registration_changes"
	tOrders    = "schedule.meal_orders"
	tDays      = "schedule.meal_order_days"
	tLines     = "schedule.meal_order_lines"
	tCounters  = "schedule.meal_order_change_counters"
	tChanges   = "schedule.meal_order_changes"
	tReads     = "users.parent_message_reads"
)

// pendingOrderChange fixes today's order with the child on it and then takes
// the child off today, so the next settlement records one order change.
func pendingOrderChange(t *testing.T, r mealWriteRig, f mealWriteFamily) {
	t.Helper()
	r.mustSend(t, mealSchedule(f.StudentID, 1, 2, 3, 4, 5), f)
	r.override(t, f, childReadToday, false)
}

// laterSchedule stores regular days that start after the next change takes
// effect, so a parent's new pattern deletes them.
func laterSchedule(t *testing.T, r mealWriteRig, f mealWriteFamily) {
	t.Helper()
	_, err := r.super.NewRaw(`INSERT INTO schedule.meal_participation_schedules (tenant_id, student_id, effective_from, monday, tuesday, wednesday, thursday, friday, changed_by_account_id) VALUES (?, ?, ?, true, false, true, false, true, ?)`,
		f.TenantID, f.StudentID, mealWriteLaterFrom, f.AccountID).Exec(context.Background())
	require.NoError(t, err)
}

func (r mealWriteRig) mustSend(t *testing.T, w writeCall, f mealWriteFamily) {
	t.Helper()
	r.faults.arm(nil)
	rec := r.call(t, context.Background(), w, f.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// override stores a day exception directly, as the school would.
func (r mealWriteRig) override(t *testing.T, f mealWriteFamily, date calendar.Date, participating bool) {
	t.Helper()
	_, err := r.super.NewRaw(`INSERT INTO schedule.meal_participation_overrides (tenant_id, student_id, date, participating, changed_by_account_id) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, student_id, date) DO UPDATE SET participating = EXCLUDED.participating`, f.TenantID, f.StudentID, date, participating, f.AccountID).Exec(context.Background())
	require.NoError(t, err)
}

// Weekly orders: the week's order is due Monday 09:00, while a later day
// stays open for parents until its own deadline.
var weeklyOrders = map[string]any{"operations.meal_order_rhythm": "weekly"}

// Split deadlines: today's order is due at 09:00 (the earlier deadline),
// parents may still cancel until 12:00.
var lateCancellation = map[string]any{"operations.meal_registration_model": "ongoing_split", "operations.meal_cancellation_cutoff_time": "12:00"}

func mealWriteScenarios() map[string][]writeScenario {
	day := func(date calendar.Date, participating bool) func(mealWriteFamily) writeCall {
		return func(f mealWriteFamily) writeCall { return mealDay(f.StudentID, date, participating) }
	}
	clearDay := func(date calendar.Date) func(mealWriteFamily) writeCall {
		return func(f mealWriteFamily) writeCall { return mealClear(f.StudentID, date) }
	}
	week := func(f mealWriteFamily) writeCall { return mealWeek(f.StudentID, childReadToday, false) }
	talk := func(f mealWriteFamily) writeCall { return conversation(f.StudentID) }
	exception := func(date calendar.Date, participating bool) func(*testing.T, mealWriteRig, mealWriteFamily) {
		return func(t *testing.T, r mealWriteRig, f mealWriteFamily) { r.override(t, f, date, participating) }
	}
	return map[string][]writeScenario{
		"S": {
			{route: "S", name: "order due, later pattern dropped", frame: true, prepare: laterSchedule,
				call: func(f mealWriteFamily) writeCall { return mealSchedule(f.StudentID, 1, 2, 3) },
				rows: map[string]int{tOrders: 1, tDays: 1, tLines: 1, tLog: 1}},
			{route: "S", name: "order change pending", prepare: pendingOrderChange,
				call: func(f mealWriteFamily) writeCall { return mealSchedule(f.StudentID, 1, 2) },
				rows: map[string]int{tCounters: 1, tChanges: 1, tLog: 1}},
		},
		"D": {
			{route: "D", name: "cancel a day", frame: true, call: day(mealWriteWednesday, false), rows: map[string]int{tOverrides: 1, tLog: 1}},
		},
		"W": {
			{route: "W", name: "order due, mixed week", frame: true, call: week, rows: map[string]int{tOrders: 1, tDays: 1, tLines: 1, tOverrides: 4, tLog: 4}},
			{route: "W", name: "order change pending", prepare: pendingOrderChange, call: week, rows: map[string]int{tCounters: 1, tChanges: 1, tOverrides: 4, tLog: 4}},
		},
		"C": {
			{route: "C", name: "clear an exception", frame: true, prepare: exception(mealWriteWednesday, false), call: clearDay(mealWriteWednesday), rows: map[string]int{tOverrides: -1, tLog: 1}},
			{route: "C", name: "nothing to clear", call: clearDay(mealWriteThursday), rows: map[string]int{}},
		},
		"M": {
			{route: "M", name: "staff message to mark", frame: true, prepare: func(t *testing.T, r mealWriteRig, f mealWriteFamily) {
				r.message(t, f, r.thread(t, f), true, "Hallo")
			}, call: talk, rows: map[string]int{tReads: 1}, broadcasts: 1},
			{route: "M", name: "own message only", prepare: func(t *testing.T, r mealWriteRig, f mealWriteFamily) {
				r.message(t, f, r.thread(t, f), false, "Frage")
			}, call: talk, rows: map[string]int{}},
			{route: "M", name: "answer to a group thread", prepare: func(t *testing.T, r mealWriteRig, f mealWriteFamily) {
				r.groupAnswer(t, f)
			}, call: talk, rows: map[string]int{}},
			{route: "M", name: "no conversation", call: talk, rows: map[string]int{}},
		},
		"weekly": {
			{route: "D", name: "weekly order due, change after the write", frame: true, settings: weeklyOrders, rosterRetry: true, call: day(mealWriteWednesday, false),
				rows: map[string]int{tOrders: 1, tDays: 5, tLines: 5, tOverrides: 1, tLog: 1, tCounters: 1, tChanges: 1}},
			{route: "D", name: "weekly order fixed, counter goes on", frame: true, settings: weeklyOrders,
				prepare: func(t *testing.T, r mealWriteRig, f mealWriteFamily) {
					r.mustSend(t, mealDay(f.StudentID, mealWriteThursday, false), f)
				},
				call: day(mealWriteWednesday, false), rows: map[string]int{tOverrides: 1, tLog: 1, tChanges: 1}},
			{route: "C", name: "weekly order due, exception cleared", frame: true, settings: weeklyOrders, rosterRetry: true, prepare: exception(mealWriteWednesday, false), call: clearDay(mealWriteWednesday),
				rows: map[string]int{tOrders: 1, tDays: 5, tLines: 4, tOverrides: -1, tLog: 1, tCounters: 1, tChanges: 1}},
			{route: "W", name: "weekly order due, changes after the write", frame: true, settings: weeklyOrders, rosterRetry: true, call: week,
				rows: map[string]int{tOrders: 1, tDays: 5, tLines: 5, tOverrides: 4, tLog: 4, tCounters: 1, tChanges: 4}},
		},
		"split": {
			{route: "D", name: "cancel today after the order", frame: true, settings: lateCancellation, call: day(childReadToday, false),
				rows: map[string]int{tOrders: 1, tDays: 1, tLines: 1, tOverrides: 1, tLog: 1, tCounters: 1, tChanges: 1}},
			{route: "C", name: "clear today's registration after the order", frame: true, settings: lateCancellation,
				prepare: func(t *testing.T, r mealWriteRig, f mealWriteFamily) {
					_, err := r.super.NewRaw(`UPDATE schedule.meal_participation_schedules SET monday = false WHERE student_id = ?`, f.StudentID).Exec(context.Background())
					require.NoError(t, err)
					r.override(t, f, childReadToday, true)
				}, call: clearDay(childReadToday),
				rows: map[string]int{tOrders: 1, tDays: 1, tLines: 1, tOverrides: -1, tLog: 1, tCounters: 1, tChanges: 1}},
		},
	}
}

// groupAnswer stores the guardian's answer to a class group thread in the
// team conversation: the only later statement is the group titles' read.
func (r mealWriteRig) groupAnswer(t *testing.T, f mealWriteFamily) {
	t.Helper()
	var groupID int64
	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), r.db, f.TenantID, func(ctx context.Context) error {
		var err error
		groupID, err = CreateParentGroupClassThreadForTest(ctx, testpkg.NewDatabase(t, r.db), f.staffAccountID, "1a", "Ausflug", "Bitte zusagen")
		return err
	}))
	threadID := r.thread(t, f)
	r.faults.arm(nil)
	_, err := r.super.NewRaw(`INSERT INTO users.parent_messages (tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, kind, group_thread_id) VALUES (?, ?, ?, ?, 'guardian', 'Rig', 'Zusage', 'message', ?)`,
		f.TenantID, threadID, f.StudentID, f.AccountID, groupID).Exec(context.Background())
	require.NoError(t, err)
}

// writeOutcome is what a cell observes.
type writeOutcome struct {
	answer     string // "200" or "<status> <code>"
	rows       string // "none", "full" or "other"
	broadcasts int
}

func (o writeOutcome) String() string {
	return fmt.Sprintf("%s rows=%s broadcasts=%d", o.answer, o.rows, o.broadcasts)
}

// scenarioRun is a prepared scenario: its starting rows, its successful run
// and the statements of that run, numbered from 1.
type scenarioRun struct {
	writeScenario
	group     string
	family    mealWriteFamily
	start     map[string]string // raw, for restoring
	startCmp  map[string]string
	fullCmp   map[string]string
	success   writeOutcome
	stmts     []string
	switchAt  int // number of the switch to the school
	firstStmt int // number of the first statement after the first BEGIN
}

func (r mealWriteRig) prepareScenario(t *testing.T, group string, sc writeScenario) *scenarioRun {
	t.Helper()
	f := r.mealFamily(t)
	ctx := testpkg.Ctx(t)
	for key, value := range map[string]any{settingsModule.KeyMealPlanEnabled: true, settingsModule.KeyMealRegistrationEnabled: true, settingsModule.KeyParentMessageLeadershipEnabled: true} {
		require.NoError(t, r.setSetting(ctx, key, value))
	}
	for key, value := range sc.settings {
		require.NoError(t, r.setSetting(ctx, key, value))
	}
	if sc.prepare != nil {
		sc.prepare(t, r, f)
	}
	run := &scenarioRun{writeScenario: sc, group: group, family: f}
	run.start = r.rows(t, f.TenantID, false)
	run.startCmp = r.rows(t, f.TenantID, true)
	counts := r.rowCounts(t, f.TenantID)
	before := r.broadcasts()
	r.faults.arm(nil)
	rec := r.call(t, context.Background(), sc.call(f), f.token)
	require.Equal(t, http.StatusOK, rec.Code, "%s %s: %s", sc.route, sc.name, rec.Body.String())
	events, _ := r.faults.snapshot()
	// The oracle: the row changes and broadcasts fixed in the scenario, and
	// the success state recorded on development.
	after := r.rowCounts(t, f.TenantID)
	for _, table := range mealWriteTables {
		require.Equal(t, sc.rows[table], after[table]-counts[table], "%s %s: rows of %s", sc.route, sc.name, table)
	}
	require.Equal(t, sc.broadcasts, r.broadcasts()-before, "%s %s: broadcasts", sc.route, sc.name)
	r.matchSuccessState(t, group, sc.route+" "+sc.name, f.TenantID)
	run.fullCmp = r.rows(t, f.TenantID, true)
	run.success = writeOutcome{answer: "200", rows: "full", broadcasts: sc.broadcasts}
	n, begun := 0, false
	for _, e := range events {
		switch e {
		case "BEGIN":
			begun = true
			continue
		case "COMMIT", "ROLLBACK":
			continue
		}
		n++
		run.stmts = append(run.stmts, e)
		if begun && run.firstStmt == 0 {
			run.firstStmt = n
		}
		if strings.HasPrefix(e, "SELECT set_config('role'") && run.switchAt == 0 {
			run.switchAt = n
		}
	}
	require.Positive(t, run.switchAt, "%s %s: %q", sc.route, sc.name, run.stmts)
	var writes []string
	for _, q := range run.stmts {
		if stmtClass(q) == "write" {
			writes = append(writes, strings.Join(strings.Fields(q)[:3], " "))
		}
	}
	t.Logf("%s %s: %d statements, writes %q", sc.route, sc.name, len(run.stmts), writes)
	if os.Getenv("PHX_S4_DEBUG") != "" {
		for i, q := range run.stmts {
			t.Logf("  %02d %.110s", i+1, strings.Join(strings.Fields(q), " "))
		}
	}
	r.restore(t, f.TenantID, run.start)
	return run
}

// observe runs the scenario's call with the faults set armed and reads what
// it left.
func (r mealWriteRig) observe(t *testing.T, run *scenarioRun, set func(*writeFaults)) (writeOutcome, int, []string) {
	t.Helper()
	before := r.broadcasts()
	r.faults.arm(set)
	rec := r.call(t, context.Background(), run.call(run.family), run.family.token)
	events, begins := r.faults.snapshot()
	out := writeOutcome{answer: answer(t, rec), broadcasts: r.broadcasts() - before}
	switch {
	case r.sameRows(run.family.TenantID, run.startCmp, 0):
		out.rows = "none"
	case r.sameRows(run.family.TenantID, run.fullCmp, 0):
		out.rows = "full"
	case r.sameRows(run.family.TenantID, run.startCmp, 1):
		// A dropped connection may still hold its transaction a moment.
		out.rows = "none"
	default:
		out.rows = "other"
	}
	if out.rows != "none" {
		r.restore(t, run.family.TenantID, run.start)
	}
	return out, begins, events
}

// stmtClass sorts a statement of the school's part by what it does.
func stmtClass(q string) string {
	upper := strings.ToUpper(q)
	switch {
	case strings.HasPrefix(upper, "WITH CLASS_WRITE_GATE"), strings.Contains(upper, "PG_ADVISORY_XACT_LOCK("):
		return "lock"
	case strings.HasPrefix(upper, "INSERT"), strings.HasPrefix(upper, "UPDATE"), strings.HasPrefix(upper, "DELETE"):
		return "write"
	default:
		return "read"
	}
}

// matrixCells lists the statement cells of a scenario: number, label and
// whether the cell gets the full cross.
type stmtCell struct {
	at    int
	label string
	full  bool
}

func (run *scenarioRun) cells() []stmtCell {
	var out []stmtCell
	add := func(at int, label string, full bool) {
		for i := range out {
			if out[i].at == at {
				out[i].full = out[i].full || full
				out[i].label += "+" + label
				return
			}
		}
		out = append(out, stmtCell{at: at, label: label, full: full})
	}
	if run.frame {
		// A: the administrative part, from the admin role up to the switch.
		add(run.firstStmt, "A first", true)
		add(run.switchAt-1, "A last", true)
		for at := run.firstStmt + 1; at < run.switchAt-1; at++ {
			add(at, "A", false)
		}
		add(run.switchAt, "T switch", true)
	}
	first, last := map[string]int{}, map[string]int{}
	for at := run.switchAt + 1; at <= len(run.stmts); at++ {
		class := stmtClass(run.stmts[at-1])
		if first[class] == 0 {
			first[class] = at
		}
		last[class] = at
	}
	for _, class := range []string{"lock", "read", "write"} {
		if first[class] != 0 {
			add(first[class], "B-"+class+" first", true)
			add(last[class], "B-"+class+" last", true)
		}
	}
	for at := run.switchAt + 1; at <= len(run.stmts); at++ {
		q := run.stmts[at-1]
		// The positions that carry undone intermediate states get the full
		// cross too: after the later schedules' delete, after the counter.
		full := strings.HasPrefix(q, "INSERT INTO schedule.meal_participation_schedules") || strings.HasPrefix(q, "INSERT INTO schedule.meal_order_changes")
		add(at, "B-"+stmtClass(q), full)
	}
	return out
}

var (
	mainKinds    = []string{"plain", "23514", "57014", "40P01", "40001"}
	minimumKinds = []string{"40P01", "57014", "plain"}
	cleanupKinds = []string{"", "plain", "23514", "57014", "40P01", "40001", "bad_conn"}
)

// receiptAt reports whether statement at belongs to the read-receipt lookup
// of the conversation, whose failure the route logs and passes over: the
// statements after the message list up to the read cursor.
func (run *scenarioRun) receiptAt(at int) bool {
	list, cursor := 0, 0
	for i, q := range run.stmts {
		if strings.HasPrefix(q, `SELECT "parent_message"."client_message_id"`) && list == 0 {
			list = i + 1
		}
		if strings.HasPrefix(q, "SELECT r.last_read_at AS last_read_at") {
			cursor = i + 1
		}
	}
	return list > 0 && cursor > 0 && at > list && at <= cursor
}

// rosterRetryAt reports whether statement at belongs to the batched reads
// of an order that fixes several days: the roster and the participation of
// all due days (order_registrations.go), up to the order's insert. On an
// error the meal plan reads the day alone again.
func (run *scenarioRun) rosterRetryAt(at int) bool {
	if !run.rosterRetry {
		return false
	}
	first := 0
	for i, q := range run.stmts {
		switch {
		case i+1 <= run.switchAt:
		case first == 0 && strings.HasPrefix(q, `SELECT DISTINCT "student".id`):
			first = i + 1
		case first > 0 && strings.HasPrefix(q, "INSERT INTO schedule.meal_orders"):
			return at >= first && at <= i
		}
	}
	return false
}

// laterSQL reports whether a statement follows the read cursor before the
// commit.
func (run *scenarioRun) laterSQL() bool {
	for i, q := range run.stmts {
		if strings.HasPrefix(q, "SELECT r.last_read_at AS last_read_at") {
			return i+1 < len(run.stmts)
		}
	}
	return false
}

// expectStatement is development's answer to a fault of kind at statement
// at. Every fault answers 500 with nothing stored, except at the read
// receipt: there the route goes on, and what follows decides.
func (run *scenarioRun) expectStatement(at int, kind string) writeOutcome {
	failed := writeOutcome{answer: "500 general.server", rows: "none"}
	if run.rosterRetryAt(at) {
		// A failure the client saw leaves the transaction healthy, and the
		// single-day read answers; one the server raised aborts it, and the
		// retry fails with it.
		if !serverRaised(kind) {
			return run.success
		}
		return failed
	}
	if !run.receiptAt(at) {
		return failed
	}
	switch {
	case !serverRaised(kind):
		// The client saw the failure; the transaction is healthy.
		return run.success
	case kind == "57014":
		// pgdriver drops the connection: the next statement or the COMMIT
		// fails on it.
		return failed
	case run.laterSQL():
		// The aborted transaction refuses the next statement (25P02).
		return failed
	default:
		// COMMIT of the aborted transaction ends with ROLLBACK and no
		// error: the answer stands, nothing of it is stored.
		return writeOutcome{answer: "200", rows: run.success.rows, broadcasts: run.success.broadcasts}
	}
}

func (r mealWriteRig) assertCell(t *testing.T, run *scenarioRun, name string, want writeOutcome, set func(*writeFaults)) {
	t.Helper()
	got, begins, events := r.observe(t, run, set)
	if want.rows == "full" && run.fullCmp != nil && equalRows(run.fullCmp, run.startCmp) {
		want.rows = "none"
	}
	if !baselineRun() && begins > 0 {
		require.Equal(t, 1, begins, "%s %s %s: one BEGIN: %q", run.route, run.name, name, events)
	}
	require.Equal(t, want.String(), got.String(), "%s %s %s", run.route, run.name, name)
}

// runMatrix walks the cells of one group of scenarios, each scenario in a
// school of its own.
func runMatrix(t *testing.T, group string) {
	rig := newMealWriteRig(t)
	cells := 0
	for _, sc := range mealWriteScenarios()[group] {
		t.Run(sc.route+" "+sc.name, func(t *testing.T) {
			// The school of the scenario lives in the rig's own database.
			testpkg.EnsureTestTenant(t, rig.super, testpkg.OwnTenant(t))
			cells += rig.runScenario(t, group, sc)
		})
	}
	t.Logf("%s: %d cells", group, cells)
	rig.writeSuccessStates(t, group)
}

// runScenario walks the cells of one scenario and counts them.
func (rig mealWriteRig) runScenario(t *testing.T, group string, sc writeScenario) int {
	cells := 0
	{
		run := rig.prepareScenario(t, group, sc)
		if run.frame {
			// R: the first BEGIN fails once.
			for _, kind := range []string{"bad_conn", "plain", "57014", "40P01", "40001"} {
				rig.assertCell(t, run, "BEGIN "+kind, writeOutcome{answer: "500 general.server", rows: "none"}, func(f *writeFaults) { f.beginKind = kind })
				cells++
			}
			// K: the COMMIT of the school's part.
			for _, kind := range []string{"40P01", "40001"} {
				rig.assertCell(t, run, "COMMIT not applied "+kind, writeOutcome{answer: "500 general.server", rows: "none"}, func(f *writeFaults) { f.commitKind = kind })
				cells++
			}
			for _, kind := range []string{"plain", "57014", "bad_conn"} {
				rig.assertCell(t, run, "COMMIT unknown, applied "+kind, writeOutcome{answer: "500 general.server", rows: "full"}, func(f *writeFaults) { f.commitKind, f.commitApplied = kind, true })
				rig.assertCell(t, run, "COMMIT unknown, not applied "+kind, writeOutcome{answer: "500 general.server", rows: "none"}, func(f *writeFaults) { f.commitKind = kind })
				cells += 2
			}
		}
		// Success, confirmed COMMIT: the after-commit work runs.
		rig.assertCell(t, run, "success", run.success, nil)
		cells++
		for _, cell := range run.cells() {
			kinds, cleanups := minimumKinds, []string{""}
			if cell.full {
				kinds, cleanups = mainKinds, cleanupKinds
			}
			for _, kind := range kinds {
				for _, cleanup := range cleanups {
					name := fmt.Sprintf("stmt %d (%s: %.40s) %s / rollback %s", cell.at, cell.label, run.stmts[cell.at-1], kind, cleanup)
					at, kind, cleanup := cell.at, kind, cleanup
					rig.assertCell(t, run, name, run.expectStatement(at, kind), func(f *writeFaults) {
						f.stmtAt, f.stmtKind, f.rollbackKind = at, kind, cleanup
					})
					cells++
				}
			}
		}
	}
	return cells
}

func TestParentMealMessageWriteMatrixMessages(t *testing.T) {
	t.Parallel()
	runMatrix(t, "M")
}

func TestParentMealMessageWriteMatrixSchedule(t *testing.T) {
	t.Parallel()
	runMatrix(t, "S")
}

func TestParentMealMessageWriteMatrixDay(t *testing.T) {
	t.Parallel()
	runMatrix(t, "D")
}

func TestParentMealMessageWriteMatrixWeek(t *testing.T) {
	t.Parallel()
	runMatrix(t, "W")
}

func TestParentMealMessageWriteMatrixClear(t *testing.T) {
	t.Parallel()
	runMatrix(t, "C")
}

func TestParentMealMessageWriteMatrixWeeklyOrders(t *testing.T) {
	t.Parallel()
	runMatrix(t, "weekly")
}

func TestParentMealMessageWriteMatrixSplitDeadlines(t *testing.T) {
	t.Parallel()
	runMatrix(t, "split")
}
