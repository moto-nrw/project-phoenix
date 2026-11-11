package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/gitfixture"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	portalCompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The rig of the meal participation and conversation writes of #5862 (S4):
// the parent routes over a pool whose connections record every transaction
// step and fail the armed one, on a fixed school day. The meal plan and the
// portal share the clock, so ordering deadlines do not depend on when the
// test runs.

// mealWriteNow is the rig's clock: Monday 31 August 2026, 10:00 Berlin. The
// default deadline (09:00, no lead days) has passed for today's order and
// not for tomorrow's.
func mealWriteNow() time.Time { return childReadToday.BerlinMidnight().Add(10 * time.Hour) }

// --- the recording, faulting pool -----------------------------------------

// writeFaults records the steps of the request in flight and fails the armed
// ones, each once. Statement numbers count every statement the pool runs
// since the last reset, inside and outside transactions.
type writeFaults struct {
	mu sync.Mutex
	writeFaultState
	// after runs once statement afterAt returned, for example to cancel the
	// request; the pool consumes it. Only the arming callback sets it.
	after func()
}

// writeFaultState is what one request records and what is armed for it.
type writeFaultState struct {
	events []string // "BEGIN", "COMMIT", "ROLLBACK" or the statement
	stmts  int
	begins int

	beginKind string // fail the first BEGIN with this kind ("" none)

	stmtAt   int    // fail the stmtAt-th statement (0 none)
	stmtKind string // "plain" (no SQLSTATE), "bad_conn" or an SQLSTATE the server raises
	fired    bool   // the statement fault fired

	rollbackKind string // fail the first ROLLBACK after the statement fault ("" none)
	rollbackHit  bool

	afterAt int // run the armed callback once the afterAt-th statement returned (0 none)

	commitKind    string // fail the first COMMIT after the switch to the school ("" none)
	commitApplied bool   // commit for real before reporting the failure
	commitHit     bool
	switched      bool
}

// arm clears the record and arms the given faults.
func (f *writeFaults) arm(set func(*writeFaults)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeFaultState = writeFaultState{}
	if set != nil {
		set(f)
	}
}

func (f *writeFaults) snapshot() (events []string, begins int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...), f.begins
}

// injected is a failure of kind as the client sees it: "plain" has no
// SQLSTATE, "bad_conn" is driver.ErrBadConn, anything else a PostgreSQL error.
func injected(kind string) error {
	if kind == "plain" {
		return testpkg.InjectedFailure("")
	}
	return testpkg.InjectedFailure(kind)
}

// serverRaised reports whether kind is raised by the server on the
// statement's own connection, so the transaction is really aborted there.
func serverRaised(kind string) bool { return kind != "plain" && kind != "bad_conn" }

type writeFaultConnector struct {
	driver.Connector
	faults *writeFaults
}

func (c writeFaultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &writeFaultConn{Conn: conn, faults: c.faults}, nil
}

type writeFaultConn struct {
	driver.Conn
	faults *writeFaults
}

// statement records query and answers whether it is the armed one, and the
// callback to run once it returned.
func (c *writeFaultConn) statement(query string) (string, bool, func()) {
	f := c.faults
	f.mu.Lock()
	defer f.mu.Unlock()
	query = strings.TrimSpace(query)
	f.stmts++
	var after func()
	if f.afterAt == f.stmts && f.after != nil {
		after, f.after = f.after, nil
	}
	f.events = append(f.events, query)
	if strings.HasPrefix(query, "SELECT set_config('role'") {
		f.switched = true
	}
	if f.stmtAt == f.stmts && !f.fired {
		f.fired = true
		return f.stmtKind, true, after
	}
	return "", false, after
}

func (c *writeFaultConn) fail(ctx context.Context, kind string) error {
	if !serverRaised(kind) {
		return injected(kind)
	}
	_, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'injected %s' USING ERRCODE = '%s'; END $$`, kind, kind), nil)
	if err == nil {
		return errors.New("raise returned no error")
	}
	return err
}

func (c *writeFaultConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	kind, armed, after := c.statement(query)
	if after != nil {
		defer after()
	}
	if armed {
		return nil, c.fail(ctx, kind)
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *writeFaultConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	kind, armed, after := c.statement(query)
	if armed {
		if after != nil {
			defer after()
		}
		return nil, c.fail(ctx, kind)
	}
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	if after != nil {
		if err != nil {
			after()
			return nil, err
		}
		return &afterRows{Rows: rows, after: after}, nil
	}
	return rows, err
}

// afterRows runs after once its rows are closed.
type afterRows struct {
	driver.Rows
	after func()
	once  sync.Once
}

func (r *afterRows) Close() error {
	err := r.Rows.Close()
	r.once.Do(r.after)
	return err
}

func (c *writeFaultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	f := c.faults
	f.mu.Lock()
	f.begins++
	f.events = append(f.events, "BEGIN")
	kind := ""
	if f.begins == 1 {
		kind = f.beginKind
	}
	f.mu.Unlock()
	if kind != "" {
		return nil, injected(kind)
	}
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return writeFaultTx{Tx: tx, faults: f}, nil
}

func (c *writeFaultConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *writeFaultConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *writeFaultConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type writeFaultTx struct {
	driver.Tx
	faults *writeFaults
}

func (t writeFaultTx) Commit() error {
	f := t.faults
	f.mu.Lock()
	f.events = append(f.events, "COMMIT")
	kind, applied := "", false
	if f.commitKind != "" && f.switched && !f.commitHit {
		f.commitHit = true
		kind, applied = f.commitKind, f.commitApplied
	}
	f.mu.Unlock()
	if kind == "" {
		return t.Tx.Commit()
	}
	var err error
	if applied {
		err = t.Tx.Commit()
	} else {
		err = t.Tx.Rollback()
	}
	if err != nil {
		return err
	}
	return injected(kind)
}

func (t writeFaultTx) Rollback() error {
	f := t.faults
	f.mu.Lock()
	f.events = append(f.events, "ROLLBACK")
	kind := ""
	if f.rollbackKind != "" && f.fired && !f.rollbackHit {
		f.rollbackHit = true
		kind = f.rollbackKind
	}
	f.mu.Unlock()
	err := t.Tx.Rollback()
	if kind != "" {
		return injected(kind)
	}
	return err
}

// --- the rig ----------------------------------------------------------------

// mealWriteRig serves the five routes over the recording pool. It reaches
// the owners' test doubles only through functions.
type mealWriteRig struct {
	childReadRig
	super  *bun.DB
	faults *writeFaults
	// broadcasts counts the read receipts sent to the school.
	broadcasts func() int
	// openThread opens the family's team conversation.
	openThread func(t *testing.T, f mealWriteFamily) int64
	// addMessage stores a message from the staff member or the guardian.
	addMessage func(t *testing.T, f mealWriteFamily, threadID int64, fromStaff bool, body string)
	// setSetting stores a school setting.
	setSetting func(ctx context.Context, key string, value any) error
}

func newMealWriteRig(t *testing.T) mealWriteRig {
	t.Helper()
	super := testpkg.SetupIsolatedTestDB(t)
	faults := &writeFaults{}
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(writeFaultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(testpkg.IsolatedTestDSN(t))), faults: faults,
	})))
	t.Cleanup(func() { _ = pool.Close() })
	database := testpkg.NewDatabase(t, pool)
	messages := NewParentMessagingSuiteRepositories(database)
	records, err := NewParentMessagingTestRepositories(database)
	require.NoError(t, err)
	broadcaster := testpkg.NewRecordingBroadcaster()
	identity, err := NewTestIdentityAccess(database, slog.Default())
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, pool))
	require.NoError(t, err)
	people := NewPeopleRepositorySuiteFactory(database)
	mealSettings, err := owners.BuildMealPlanSettings(settings.Settings)
	require.NoError(t, err)
	schoolCalendar, err := owners.BuildSchoolCalendarRecords(database, func(schoolCalendarCompose.Observation) {}, settings.Settings)
	require.NoError(t, err)
	meals, err := newMealPlanAt(database, mealSettings, owners.NewStudentRootReads(people.Student), owners.MustNewPeopleDirectory(database), schoolCalendar, mealWriteNow)
	require.NoError(t, err)
	rig := newChildReadRigOn(t, pool, func(deps *portalCompose.Dependencies) {
		deps.Now = mealWriteNow
		deps.MealPlan = meals
		deps.MessagingEnabled = owners.BindParentMessagingGate(settings.Settings, slog.Default())
		deps.MessageThreadRepo = owners.BindParentMessageThreadAccess(messages.ParentMessageThread)
		deps.MessageRepo = owners.BindParentMessageReads(records.Message)
		deps.MessageReadRepo = owners.BindParentMessageInboxReads(messages.ParentMessageRead)
		deps.Conversations = NewParentConversationCore(messages.ParentMessageThread, records.Message, messages.ParentMessageRead, broadcaster, slog.Default(),
			owners.BindParentLeadershipChoice(settings.Settings, owners.BindParentMessageLeadership(people.Staff, identity)))
		deps.GroupThreads = NewParentGroupThreadTestPort(database)
	})
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), settingsModule.KeyParentMessageLeadershipEnabled, true, nil, nil))
	return mealWriteRig{
		childReadRig: rig, super: super, faults: faults,
		broadcasts: func() int { return len(broadcaster.Calls()) },
		openThread: func(t *testing.T, f mealWriteFamily) int64 {
			t.Helper()
			thread, err := testpkg.TenantQuery4(t, testpkg.Ctx(t), pool, messages.ParentMessageThread.GetOrCreate, f.TenantID, f.StudentID, f.AccountID, "team")
			require.NoError(t, err)
			return thread.ID
		},
		addMessage: func(t *testing.T, f mealWriteFamily, threadID int64, fromStaff bool, body string) {
			t.Helper()
			sender, kind := f.AccountID, "guardian"
			if fromStaff {
				sender, kind = f.staffAccountID, "staff"
			}
			message := &communication.PortalParentMessage{ThreadID: threadID, StudentID: f.StudentID, SenderAccountID: sender, SenderKind: kind, SenderName: "Rig", Body: body, Kind: "message"}
			message.TenantID = f.TenantID
			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), pool, records.Message.Create, message))
		},
		setSetting: func(ctx context.Context, key string, value any) error {
			return settings.Settings.SetValue(ctx, key, value, nil, nil)
		},
	}
}

// newMealPlanAt composes the Meal Plan like owners.BuildServeMealPlan, on
// the clock now, so the ordering deadlines do not depend on when the test
// runs.
func newMealPlanAt(db transactionruntime.Database, settings *mealplanCompose.Settings, students owners.StudentRootReads, names peopledirectory.Capability, records schoolcalendar.Days, now func() time.Time) (mealplan.Capability, error) {
	return mealplanCompose.New(mealplanCompose.Dependencies{
		DB: db.Pool(), Settings: settings, Observe: func(mealplanCompose.Observation) {}, Now: now,
		Participants: func(ctx context.Context, date string) ([]mealplan.ParticipantCandidate, error) {
			rows, err := students.OverlappingOnDate(ctx, date, now())
			if err != nil {
				return nil, err
			}
			out := make([]mealplan.ParticipantCandidate, 0, len(rows))
			for _, row := range rows {
				out = append(out, mealplan.ParticipantCandidate{StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass})
			}
			return out, nil
		},
		ParticipantDays: func(ctx context.Context, dates []string) (map[string][]mealplan.ParticipantCandidate, error) {
			rows, err := peopledirectory.StudentRostersOnDates(ctx, names, dates, calendar.DateFromTime(now()).String())
			if err != nil {
				return nil, err
			}
			out := make(map[string][]mealplan.ParticipantCandidate, len(rows))
			for date, entries := range rows {
				candidates := make([]mealplan.ParticipantCandidate, 0, len(entries))
				for _, row := range entries {
					if row.Record != nil {
						candidates = append(candidates, mealplan.ParticipantCandidate{StudentID: row.Record.ID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.Record.SchoolClass})
					}
				}
				out[date] = candidates
			}
			return out, nil
		},
		StudentNames: func(ctx context.Context, ids []int64) ([]mealplan.ParticipantCandidate, error) {
			rows, err := names.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			out := make([]mealplan.ParticipantCandidate, 0, len(rows))
			for _, row := range rows {
				out = append(out, mealplan.ParticipantCandidate{StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName})
			}
			return out, nil
		},
		Calendar: func(ctx context.Context, from, to string) (map[string]bool, []mealplan.HolidayPeriod, error) {
			closed, err := records.NonWorkingDayDates(ctx, from, to)
			if err != nil {
				return nil, nil, err
			}
			periods, err := records.ListCalendarPeriods(ctx, schoolcalendar.HolidaysOverlapping(from, to))
			if err != nil {
				return nil, nil, err
			}
			holidays := make([]mealplan.HolidayPeriod, 0, len(periods))
			for _, period := range periods {
				holidays = append(holidays, mealplan.HolidayPeriod{Start: period.StartDate, End: period.EndDate})
			}
			return closed, holidays, nil
		},
	})
}

// mealWriteFamily is the rig's one family: a child in care since August with
// regular meals on every weekday, its guardian and a staff member who writes
// and reads the conversation.
type mealWriteFamily struct {
	childReadFamily
	staffAccountID int64
}

func (r mealWriteRig) mealFamily(t *testing.T) mealWriteFamily {
	t.Helper()
	f := r.family(t)
	from := calendar.NewDate(2026, 8, 1)
	testpkg.SetStudentLifecycle(t, r.db, f.StudentID, "active", &from, nil)
	_, err := r.super.NewRaw(`INSERT INTO schedule.meal_participation_schedules (tenant_id, student_id, effective_from, monday, tuesday, wednesday, thursday, friday, changed_by_account_id) VALUES (?, ?, ?, true, true, true, true, true, ?)`,
		f.TenantID, f.StudentID, from, f.AccountID).Exec(context.Background())
	require.NoError(t, err)
	_, staff := testpkg.CreateTestCalendarStaff(t, r.db, "Lea", "Leitung")
	testpkg.GrantTestPermission(t, r.db, f.TenantID, staff.ID, "parent_messages:leadership")
	return mealWriteFamily{childReadFamily: f, staffAccountID: staff.ID}
}

// thread opens the family's team conversation.
func (r mealWriteRig) thread(t *testing.T, f mealWriteFamily) int64 {
	t.Helper()
	return r.openThread(t, f)
}

// message adds a message to the thread, from the staff member or the
// guardian.
func (r mealWriteRig) message(t *testing.T, f mealWriteFamily, threadID int64, fromStaff bool, body string) {
	t.Helper()
	r.addMessage(t, f, threadID, fromStaff, body)
}

// mealWriteTables hold everything the five routes persist, all keyed by the
// school; their order is the order of restoring (parents first).
var mealWriteTables = []string{
	"schedule.meal_participation_schedules",
	"schedule.meal_participation_overrides",
	"schedule.meal_registration_changes",
	"schedule.meal_orders",
	"schedule.meal_order_days",
	"schedule.meal_order_lines",
	"schedule.meal_order_change_counters",
	"schedule.meal_order_changes",
	"users.parent_message_reads",
}

// volatileColumns are generated anew by every write (identities, clock
// defaults), so state comparisons leave them out.
const volatileColumns = `ARRAY['id','order_id','created_at','updated_at','changed_at','frozen_at','detected_at']`

// rows is the persisted state of the school: raw for restoring, or without
// volatile columns for comparing.
func (r mealWriteRig) rows(t *testing.T, tenantID int64, comparable bool) map[string]string {
	t.Helper()
	out := make(map[string]string, len(mealWriteTables))
	for _, table := range mealWriteTables {
		row := `to_jsonb(t)`
		if comparable {
			row = `to_jsonb(t) - ` + volatileColumns
		}
		var raw string
		require.NoError(t, r.super.NewRaw(fmt.Sprintf(`SELECT coalesce(jsonb_agg(%s ORDER BY (%s)::text), '[]'::jsonb)::text FROM %s t WHERE tenant_id = ?`, row, row, table), tenantID).Scan(context.Background(), &raw))
		out[table] = raw
	}
	return out
}

// restore puts the school's rows back as dumped, triggers and constraints
// aside: the registration log only grows in production.
func (r mealWriteRig) restore(t *testing.T, tenantID int64, dump map[string]string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, r.super.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
			return err
		}
		for i := len(mealWriteTables) - 1; i >= 0; i-- {
			if _, err := tx.NewRaw(fmt.Sprintf(`DELETE FROM %s WHERE tenant_id = ?`, mealWriteTables[i]), tenantID).Exec(ctx); err != nil {
				return err
			}
		}
		for _, table := range mealWriteTables {
			if _, err := tx.NewRaw(fmt.Sprintf(`INSERT INTO %s OVERRIDING SYSTEM VALUE SELECT * FROM jsonb_populate_recordset(NULL::%s, ?::jsonb)`, table, table), dump[table]).Exec(ctx); err != nil {
				return fmt.Errorf("restore %s: %w", table, err)
			}
		}
		return nil
	}))
}

// sameRows reports whether the comparable state equals want; with wait > 0
// it polls up to ten seconds: a connection the pool dropped may still hold
// its transaction for a moment.
func (r mealWriteRig) sameRows(tenantID int64, want map[string]string, wait int) bool {
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := r.rowsOf(tenantID)
		if err == nil && equalRows(got, want) {
			return true
		}
		if wait == 0 || time.Now().After(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (r mealWriteRig) rowsOf(tenantID int64) (map[string]string, error) {
	out := make(map[string]string, len(mealWriteTables))
	for _, table := range mealWriteTables {
		row := `to_jsonb(t) - ` + volatileColumns
		var raw string
		if err := r.super.NewRaw(fmt.Sprintf(`SELECT coalesce(jsonb_agg(%s ORDER BY (%s)::text), '[]'::jsonb)::text FROM %s t WHERE tenant_id = ?`, row, row, table), tenantID).Scan(context.Background(), &raw); err != nil {
			return nil, err
		}
		out[table] = raw
	}
	return out, nil
}

func equalRows(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// writeCall is one request of the five routes.
type writeCall struct {
	method, path string
	body         any
}

// call answers w within writeDeadline with the armed faults.
func (r mealWriteRig) call(t *testing.T, ctx context.Context, w writeCall, token string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, writeDeadline)
	defer cancel()
	req := testutil.NewAuthenticatedRequest(t, w.method, w.path, w.body, testutil.WithJWTBearer(token)).WithContext(ctx)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- testutil.ExecuteRequest(r.router, req) }()
	select {
	case rec := <-done:
		return rec
	case <-time.After(writeDeadline + 5*time.Second):
		t.Fatalf("%s %s did not answer in time", w.method, w.path)
		return nil
	}
}

func mealSchedule(studentID int64, weekdays ...int) writeCall {
	return writeCall{http.MethodPut, fmt.Sprintf("/parent/me/children/%d/meal-participation", studentID), map[string]any{"weekdays": weekdays}}
}

func mealDay(studentID int64, date calendar.Date, participating bool) writeCall {
	return writeCall{http.MethodPut, fmt.Sprintf("/parent/me/children/%d/meal-participation/%s", studentID, date), map[string]any{"participating": participating}}
}

func mealWeek(studentID int64, date calendar.Date, participating bool) writeCall {
	return writeCall{http.MethodPut, fmt.Sprintf("/parent/me/children/%d/meal-participation/weeks/%s", studentID, date), map[string]any{"participating": participating}}
}

func mealClear(studentID int64, date calendar.Date) writeCall {
	return writeCall{http.MethodDelete, fmt.Sprintf("/parent/me/children/%d/meal-participation/%s", studentID, date), nil}
}

func conversation(studentID int64) writeCall {
	return writeCall{http.MethodGet, fmt.Sprintf("/parent/me/messages/children/%d", studentID), nil}
}

// rowCounts counts the school's rows per table.
func (r mealWriteRig) rowCounts(t *testing.T, tenantID int64) map[string]int {
	t.Helper()
	out := make(map[string]int, len(mealWriteTables))
	for _, table := range mealWriteTables {
		var n int
		require.NoError(t, r.super.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s WHERE tenant_id = ?`, table), tenantID).Scan(context.Background(), &n))
		out[table] = n
	}
	return out
}

// identityColumns name the school, child, accounts, rows and instants,
// which differ between two databases; what is left is the success state
// development and this branch must share.
const identityColumns = `ARRAY['id','order_id','created_at','updated_at','changed_at','frozen_at','detected_at','tenant_id','student_id','account_id','changed_by_account_id','thread_id','last_read_message_id','last_read_at']`

// successStatesDir holds the success states recorded on development, one
// file per scenario group, each naming the development commit it came from.
//
// Recording is bound to that commit: PHX_S4_GOLDEN=write together with
// PHX_S4_GOLDEN_SHA=<sha> writes only when <sha> is on origin/development and
// nothing under backend/ but tests and test data differs from <sha>, go.mod
// and go.sum included, with no untracked file besides those. A branch that
// changes any production source cannot record its own oracle; a comparing
// run never writes.
const successStatesDir = "testdata/parent_meal_message_write_success"

// successStateBinding is the part of the repository the recorded states are
// bound to: the whole backend except tests and test data.
var successStateBinding = []string{":(top)backend", ":(top,exclude,glob)backend/**/*_test.go", ":(top,exclude,glob)backend/**/testdata/**"}

// recordedStates is one reference file.
type recordedStates struct {
	Source string            `json:"development_source"`
	States map[string]string `json:"states"`
}

var recordedSuccessStates sync.Map // group → *sync.Map of scenario → state

func (r mealWriteRig) successState(t *testing.T, tenantID int64) string {
	t.Helper()
	var parts []string
	for _, table := range mealWriteTables {
		row := `to_jsonb(t) - ` + identityColumns
		var raw string
		require.NoError(t, r.super.NewRaw(fmt.Sprintf(`SELECT coalesce(jsonb_agg(%s ORDER BY (%s)::text), '[]'::jsonb)::text FROM %s t WHERE tenant_id = ?`, row, row, table), tenantID).Scan(context.Background(), &raw))
		parts = append(parts, table+"="+raw)
	}
	return strings.Join(parts, "\n")
}

// recording reports whether this run records the success states. It fails
// the test unless the production sources are exactly those of the named
// development commit.
func recording(t *testing.T) (string, bool) {
	t.Helper()
	if os.Getenv("PHX_S4_GOLDEN") != "write" {
		return "", false
	}
	sha := os.Getenv("PHX_S4_GOLDEN_SHA")
	if sha == "" {
		t.Fatal("PHX_S4_GOLDEN=write needs PHX_S4_GOLDEN_SHA, the development commit the states come from")
	}
	// gitfixture.Run fails the test on any Git error, so an unknown commit
	// or one off origin/development refuses the recording.
	git := func(args ...string) string { return strings.TrimSpace(gitfixture.Run(t, "..", args...)) }
	sha = git("rev-parse", "--verify", sha+"^{commit}")
	git("merge-base", "--is-ancestor", sha, "origin/development")
	if changed := git(append([]string{"diff", "--name-only", sha, "--"}, successStateBinding...)...); changed != "" {
		t.Fatalf("recording refused: the backend differs from development %s outside tests, so this tree cannot record the oracle:\n%s", sha, changed)
	}
	// Ignored files count too: Go builds an ignored generated.go all the
	// same. Only files the build or an embed can read refuse; other build
	// artifacts do not block recording.
	var untracked []string
	for _, path := range strings.Split(git(append([]string{"ls-files", "--others", "--"}, successStateBinding...)...), "\n") {
		if path != "" && buildInput(path) {
			untracked = append(untracked, path)
		}
	}
	if len(untracked) > 0 {
		t.Fatalf("recording refused: untracked or ignored backend build inputs outside tests:\n%s", strings.Join(untracked, "\n"))
	}
	return sha, true
}

// buildInput reports whether an untracked file can reach the backend build:
// Go, assembly, cgo and syso sources, the module files, the file types the
// backend embeds, and anything in a directory embedded whole.
func buildInput(path string) bool {
	switch filepath.Base(path) {
	case "go.mod", "go.sum", "go.work", "go.work.sum":
		return true
	}
	if strings.HasPrefix(path, "backend/test/query-budgets/") || strings.HasPrefix(path, "backend/api/testdata/routes/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".s", ".c", ".h", ".cc", ".cpp", ".cxx", ".hh", ".hpp", ".m", ".f", ".syso", ".swig", ".swigcxx",
		".json", ".ttf", ".png", ".html", ".tmpl", ".sql", ".yaml", ".yml", ".txt", ".xml", ".xsd":
		return true
	}
	return false
}

// matchSuccessState compares the state a success left with the state
// development left in the same scenario; while recording it only collects.
func (r mealWriteRig) matchSuccessState(t *testing.T, group, scenario string, tenantID int64) {
	t.Helper()
	state := r.successState(t, tenantID)
	if _, record := recording(t); record {
		states, _ := recordedSuccessStates.LoadOrStore(group, &sync.Map{})
		states.(*sync.Map).Store(scenario, state)
		return
	}
	raw, err := os.ReadFile(filepath.Join(successStatesDir, group+".json"))
	require.NoError(t, err, "success states of %s", group)
	var want recordedStates
	require.NoError(t, json.Unmarshal(raw, &want))
	require.Regexp(t, `^[0-9a-f]{40}$`, want.Source, "%s: the states name their development commit", group)
	require.Contains(t, want.States, scenario, "no success state recorded on development")
	require.Equal(t, want.States[scenario], state, "%s: success state differs from development %s", scenario, want.Source)
}

// writeSuccessStates stores the recorded success states of group, only
// while recording.
func (r mealWriteRig) writeSuccessStates(t *testing.T, group string) {
	t.Helper()
	sha, record := recording(t)
	if !record {
		return
	}
	states, ok := recordedSuccessStates.Load(group)
	require.True(t, ok, group)
	out := recordedStates{Source: sha, States: map[string]string{}}
	states.(*sync.Map).Range(func(key, value any) bool {
		out.States[key.(string)] = value.(string)
		return true
	})
	raw, err := json.MarshalIndent(out, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(successStatesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(successStatesDir, group+".json"), append(raw, '\n'), 0o644))
}
