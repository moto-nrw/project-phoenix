package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock is the clock the simulation and the mock server share.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	moves []time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(_ context.Context, to time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = to
	c.moves = append(c.moves, to)
	return nil
}

func (c *fakeClock) Moves() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Time(nil), c.moves...)
}

// stampedCall is one request with the shared clock's time when it arrived and
// the login (the e-mail address in the mock token) or device that sent it.
type stampedCall struct {
	Method, Path, Clock, Caller string
	Body                        map[string]any
}

func berlinTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", value, simulationBerlin)
	require.NoError(t, err)
	return parsed
}

// clockedDayServer answers the full-day API on a clock: today's shifts, one
// staff member on sick leave, and a mock token that names the login.
func clockedDayServer(t *testing.T, clock *fakeClock) (*simulationHTTPTestServer, func() []stampedCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []stampedCall
	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		caller := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		mu.Lock()
		calls = append(calls, stampedCall{Method: r.Method, Path: r.URL.Path, Clock: clock.Now().In(simulationBerlin).Format("15:04"), Caller: caller, Body: body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		respond := func(data any) {
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
		}
		switch {
		case r.URL.Path == "/health":
			respond("OK")
		case r.URL.Path == "/auth/login":
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": fmt.Sprint(body["email"])})
		case r.URL.Path == "/api/staff-shifts":
			respond([]map[string]any{
				{"staff_id": 1, "start_time": "08:00:00"},
				{"staff_id": 10, "start_time": "08:00:00"},
				{"staff_id": 11, "start_time": "08:00:00"},
				{"staff_id": 12, "start_time": "08:00:00"},
				{"staff_id": 13, "start_time": "11:30:00"},
				{"staff_id": 14, "start_time": "08:00:00", "cancelled": true},
				{"staff_id": 99, "start_time": "08:00:00"},
			})
		case r.URL.Path == "/api/time-tracking/absences":
			if caller == "sandra@test.de" {
				respond([]map[string]any{{"id": 5, "absence_type": "sick"}})
				return
			}
			respond([]any{})
		case r.URL.Path == "/api/iot/checkin" && body["student_rfid"] == "DEMO-UNREGISTERED-TAG":
			w.WriteHeader(simulationHTTPStatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": codeRFIDTagNotFound})
		case r.URL.Path == "/api/timetable/periods/bootstrap":
			respond(map[string]any{"periods": []map[string]any{{"start_date": "2000-01-01", "end_date": "2100-12-31", "is_active": true}}})
		default:
			respond(map[string]any{"id": 77})
		}
	})
	t.Cleanup(srv.Close)
	return srv, func() []stampedCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]stampedCall(nil), calls...)
	}
}

func clockedDayState(baseURL string) *SeedState {
	students := make([]SeedStudent, 100)
	for i := range students {
		students[i] = SeedStudent{ID: int64(i + 1), FirstName: "Kind", LastName: fmt.Sprint(i), Class: fmt.Sprintf("Klasse %da", i%4+1)}
	}
	return &SeedState{
		BaseURL:   baseURL,
		DevicePIN: "1234",
		Bootstrap: SeedStateBootstrap{TenantSlug: "sonnenhof"},
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{{Email: "admin@test.de", Password: "p", StaffID: 1}},
			Betreuer: []AccountCredentials{
				{Email: "julia@test.de", Password: "p", StaffID: 10, Name: "Julia Klein"},
				{Email: "markus@test.de", Password: "p", StaffID: 11, Name: "Markus Wolf"},
				{Email: "sandra@test.de", Password: "p", StaffID: 12, Name: "Sandra Schröder"},
				{Email: "heike@test.de", Password: "p", StaffID: 13, Name: "Heike Hartmann"},
				{Email: "nicole@test.de", Password: "p", StaffID: 14, Name: "Nicole Schwarz"},
			},
		},
		Devices:    map[string]SeedDevice{"d1": {APIKey: "key-1", Name: "Scanner 1"}, "d2": {APIKey: "key-2", Name: "Scanner 2"}},
		Students:   students,
		Rooms:      map[string]int64{"Kreativraum": 6, "Leseecke": 9, "Sporthalle": 10},
		Activities: map[string]int64{"Basteln": 61, "Lesen": 60},
	}
}

func callsTo(calls []stampedCall, path string) []stampedCall {
	var matched []stampedCall
	for _, call := range calls {
		if call.Path == path {
			matched = append(matched, call)
		}
	}
	return matched
}

// With a clock setter the full day runs in waves: staff stamp in at their
// shift start, children arrive class by class after their lessons, and the
// clock never passes the 13:30 the screenshots show.
func TestRunFullDay_RunsTheDayInClockWaves(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: berlinTime(t, "2026-10-20 07:30")}
	srv, recorded := clockedDayServer(t, clock)
	statePath := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, WriteSeedState(clockedDayState(srv.URL), statePath))

	require.NoError(t, RunFullDay(context.Background(), FullDayOptions{
		Client: newTestClientFactory, StatePath: statePath, Now: clock.Now, SetClock: clock.Set,
	}))
	calls := recorded()

	moves := clock.Moves()
	require.NotEmpty(t, moves)
	for i := 1; i < len(moves); i++ {
		assert.True(t, moves[i].After(moves[i-1]), "the clock only moves forward")
	}
	assert.Equal(t, "12:45", moves[len(moves)-1].Format("15:04"), "the day ends before the screenshots' 13:30")

	stamps := map[string]string{}
	for _, call := range callsTo(calls, "/api/time-tracking/check-in") {
		stamps[call.Caller] = call.Clock
	}
	assert.Equal(t, map[string]string{
		"admin@test.de": "07:58", "julia@test.de": "07:58", "markus@test.de": "07:58", "heike@test.de": "11:31",
	}, stamps, "staff stamp in at their shift start; sick leave, a cancelled shift and staff without a login are skipped")

	starts := callsTo(calls, "/api/iot/session/start")
	require.Len(t, starts, 3)
	assert.Equal(t, "07:45", starts[0].Clock, "the early care opens the day")
	assert.Equal(t, []any{14.0}, starts[0].Body["supervisor_ids"], "the last caregiver at work opens it")
	assert.Equal(t, []string{"11:52", "11:53"}, []string{starts[1].Clock, starts[2].Clock}, "one midday session per minute")
	assert.Equal(t, []any{10.0}, starts[1].Body["supervisor_ids"])
	assert.Equal(t, []any{11.0}, starts[2].Body["supervisor_ids"], "a caregiver on sick leave supervises nothing")

	arrivals := map[string][]string{}
	for _, call := range callsTo(calls, "/api/iot/attendance/toggle") {
		if call.Body["action"] == "confirm" {
			arrivals[call.Clock] = append(arrivals[call.Clock], fmt.Sprint(call.Body["rfid"]))
		}
	}
	assert.Len(t, arrivals["07:45"], 1, "early care children arrive one per minute")
	assert.Len(t, arrivals["07:50"], 1)
	for minute, tags := range arrivals {
		assert.LessOrEqual(t, len(tags), arrivalsPerMinute, "at most %d children per minute at %s", arrivalsPerMinute, minute)
	}

	waveOf := map[string]string{"Klasse 1a": "12:00", "Klasse 2a": "12:00", "Klasse 3a": "12:15", "Klasse 4a": "12:35"}
	nextWave := map[string]string{"12:00": "12:15", "12:15": "12:35", "12:35": "12:40"}
	state := clockedDayState(srv.URL)
	for _, call := range callsTo(calls, "/api/iot/checkin") {
		if call.Body["action"] != "checkin" || call.Body["student_rfid"] == "DEMO-UNREGISTERED-TAG" || call.Clock < "11:00" {
			continue
		}
		var id int64
		_, err := fmt.Sscanf(fmt.Sprint(call.Body["student_rfid"]), "DE%06X", &id)
		require.NoError(t, err)
		wave := waveOf[state.Students[id-1].Class]
		assert.True(t, call.Clock >= wave && call.Clock < nextWave[wave], "child %d of %s checks in at %s", id, state.Students[id-1].Class, call.Clock)
	}

	var unterwegs, earlyCareLeaves []string
	for _, call := range callsTo(calls, "/api/iot/checkin") {
		if call.Body["action"] == "checkout" && call.Clock == "07:58" {
			earlyCareLeaves = append(earlyCareLeaves, fmt.Sprint(call.Body["student_rfid"]))
		}
		if call.Body["action"] == "checkout" && call.Clock > "07:58" {
			unterwegs = append(unterwegs, call.Clock+" "+fmt.Sprint(call.Body["student_rfid"]))
		}
	}
	assert.Len(t, earlyCareLeaves, earlyCareChildren, "the early care children leave for their lessons")
	loaded, err := LoadSeedStateProfile(statePath, "")
	require.NoError(t, err)
	assert.Equal(t, []string{
		fmt.Sprintf("12:40 DE%06X", loaded.Students[fullDayRoomStayLimit].ID),
		fmt.Sprintf("12:40 DE%06X", loaded.Students[fullDayRoomStayLimit+1].ID),
	}, unterwegs, "only two children are unterwegs")

	var pickups []string
	for _, call := range callsTo(calls, "/api/iot/attendance/toggle") {
		if call.Body["action"] == "confirm_daily_checkout" {
			pickups = append(pickups, call.Clock)
		}
	}
	assert.Equal(t, []string{"12:45", "12:45", "12:45", "12:45", "12:45", "12:45"}, pickups,
		"the children without a room place go home")
	assert.Len(t, callsTo(calls, "/api/iot/feedback"), 6, "each pickup leaves kiosk feedback")

	for _, call := range calls {
		if call.Method == "PUT" && strings.HasPrefix(call.Path, "/api/students/") {
			assert.Equal(t, "08:30", call.Clock, "parents report sick children in the morning")
		}
	}

	var block *stampedCall
	for i, call := range calls {
		if call.Path == "/api/timetable/instances" && call.Body["title"] == openRoomBlockTitle {
			block = &calls[i]
		}
	}
	require.NotNil(t, block)
	assert.Equal(t, []any{13.0}, block.Body["staff_ids"], "the block goes to a caregiver without a running session")

	adminLogins := 0
	for _, call := range callsTo(calls, "/auth/login") {
		if call.Body["email"] == "admin@test.de" {
			adminLogins++
		}
	}
	assert.Greater(t, adminLogins, 2, "the admin signs in again after the clock jumped past the access token")
}

func TestAdvanceClock_NeverMovesBack(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: berlinTime(t, "2026-10-20 12:20")}
	srv, _ := clockedDayServer(t, clock)
	rt := newRuntime(clockedDayState(srv.URL), newClient(srv.URL, false), FullDayOptions{
		Client: newTestClientFactory, Now: clock.Now, SetClock: clock.Set,
	})
	rt.adminLoginAt = clock.Now()

	require.NoError(t, rt.advanceClock(context.Background(), at(12, 0)))
	assert.Empty(t, clock.Moves(), "a wave whose time has passed runs at once")

	require.NoError(t, rt.advanceClock(context.Background(), at(12, 25)))
	assert.Equal(t, []time.Time{berlinTime(t, "2026-10-20 12:25")}, clock.Moves())
}

func TestAdvanceClock_FailsWhenTheClockDoesNotMove(t *testing.T) {
	t.Parallel()

	start := berlinTime(t, "2026-10-20 07:30")
	rt := newRuntime(&SeedState{}, nil, FullDayOptions{
		Now:      func() time.Time { return start },
		SetClock: func(context.Context, time.Time) error { return nil },
	})

	err := rt.advanceClock(context.Background(), at(8, 0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the clock still reads 07:30")
}

func TestAdvanceClock_WithoutSetterRunsAtOnce(t *testing.T) {
	t.Parallel()

	rt := newRuntime(&SeedState{}, nil, FullDayOptions{})
	require.NoError(t, rt.advanceClock(context.Background(), at(23, 59)))
}

func TestCommandClock_PassesTheTargetAsUnixSeconds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := filepath.Join(dir, "set-clock")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s %s' \"$1\" \"$2\" > \""+dir+"/args\"\n"), 0o700))

	setter := CommandClock(script + " --berlin")
	require.NotNil(t, setter)
	require.NoError(t, setter(context.Background(), time.Unix(1792491600, 0)))

	args, err := os.ReadFile(filepath.Join(dir, "args"))
	require.NoError(t, err)
	assert.Equal(t, "--berlin 1792491600", string(args))

	assert.Nil(t, CommandClock("  "), "no command moves no clock")
	assert.Error(t, CommandClock(filepath.Join(dir, "missing"))(context.Background(), time.Unix(0, 0)))
}

func TestArrivalOrder_GroupsChildrenByTheEndOfTheirLessons(t *testing.T) {
	t.Parallel()

	students := []SeedStudent{
		{Class: "Klasse 3b"}, {Class: "Klasse 1a"}, {Class: "Klasse 4a"}, {Class: "Klasse 2b"}, {Class: "Gast"},
	}
	assert.Equal(t, [][]int{{1, 3}, {0}, {2, 4}}, arrivalOrder(students),
		"a class without a grade arrives with the last wave")
	assert.Equal(t, 3, classGrade("Klasse 3b"))
	assert.Zero(t, classGrade("Vorschule"))
}

func TestShiftDue_StampsInAtMostFiveMinutesEarly(t *testing.T) {
	t.Parallel()

	now := berlinTime(t, "2026-10-20 07:58")
	assert.True(t, shiftDue(plannedShift{StartTime: "08:00:00"}, now))
	assert.True(t, shiftDue(plannedShift{StartTime: "07:30"}, now))
	assert.False(t, shiftDue(plannedShift{StartTime: "08:04"}, now))
	assert.False(t, shiftDue(plannedShift{StartTime: "11:30:00"}, now))
	assert.False(t, shiftDue(plannedShift{StartTime: ""}, now))
}
