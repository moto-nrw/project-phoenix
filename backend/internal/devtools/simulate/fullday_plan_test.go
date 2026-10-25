package simulate

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runClockedDay runs the mocked Tuesday from 07:20 with the clock setter and
// returns the calls and the clock's moves.
func runClockedDay(t *testing.T) ([]stampedCall, []time.Time) {
	calls, moves, _ := runClockedDayWithSick(t)
	return calls, moves
}

// runClockedDayWithSick also returns the children reported sick at 08:30:
// the seed state lists its children in key order, not by ID.
func runClockedDayWithSick(t *testing.T) ([]stampedCall, []time.Time, map[int64]bool) {
	t.Helper()
	clock := &fakeClock{now: berlinTime(t, "2026-10-20 07:20")}
	srv, recorded := clockedDayServer(t, clock)
	statePath := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, WriteSeedState(clockedDayState(srv.URL), statePath))
	require.NoError(t, RunFullDay(context.Background(), FullDayOptions{
		Client: newTestClientFactory, StatePath: statePath, Now: clock.Now, SetClock: clock.Set,
	}))
	loaded, err := LoadSeedStateProfile(statePath, "")
	require.NoError(t, err)
	sick := map[int64]bool{}
	for _, student := range loaded.Students[fullDaySickFrom:fullDaySickTo] {
		sick[student.ID] = true
	}
	return recorded(), clock.Moves(), sick
}

func tagOf(id int64) string { return fmt.Sprintf("DE%06X", id) }

// scansOf lists clock and room of the child's kiosk scans with the action.
func scansOf(calls []stampedCall, id int64, action string) []string {
	var scans []string
	for _, call := range calls {
		if call.Body["action"] != action || (call.Body["student_rfid"] != tagOf(id) && call.Body["rfid"] != tagOf(id)) {
			continue
		}
		scans = append(scans, fmt.Sprintf("%s %v", call.Clock, call.Body["room_id"]))
	}
	return scans
}

// checkInsOf lists clock and room of the child's room check-ins: a kiosk
// scan into a room or a check-in on a running planned block.
func checkInsOf(calls []stampedCall, id int64) []string {
	blockRoom := map[int]int{}
	for _, block := range mockBlocks() {
		blockRoom[block["id"].(int)] = block["room_id"].(int)
	}
	var checkIns []string
	for _, call := range calls {
		var block int
		var student int64
		if _, err := fmt.Sscanf(call.Path, "/api/timetable/operations/instances/%d/students/%d/check-in", &block, &student); err == nil {
			if _, planned := blockRoom[block]; planned && student == id {
				checkIns = append(checkIns, fmt.Sprintf("%s %d", call.Clock, blockRoom[block]))
			}
			continue
		}
		if call.Path == "/api/iot/checkin" && call.Body["action"] == "checkin" && call.Body["student_rfid"] == tagOf(id) {
			checkIns = append(checkIns, fmt.Sprintf("%s %v", call.Clock, call.Body["room_id"]))
		}
	}
	return checkIns
}

// blockCallsOf lists clock and caller of the block's check-ins or
// check-outs ("check-in", "check-out") of the child.
func blockCallsOf(calls []stampedCall, block int, id int64, action string) []string {
	var booked []string
	for _, call := range callsTo(calls, fmt.Sprintf("/api/timetable/operations/instances/%d/students/%d/%s", block, id, action)) {
		booked = append(booked, call.Clock+" "+call.Caller)
	}
	return booked
}

func TestRunFullDay_RunsTheDayInClockWaves(t *testing.T) {
	t.Parallel()

	calls, moves, sick := runClockedDayWithSick(t)

	require.NotEmpty(t, moves)
	for i := 1; i < len(moves); i++ {
		assert.True(t, moves[i].After(moves[i-1]), "the clock only moves forward")
	}
	assert.Equal(t, "12:50", moves[len(moves)-1].Format("15:04"), "the day ends before the screenshots' 13:30")

	stamps := map[string]string{}
	for _, call := range callsTo(calls, "/api/time-tracking/check-in") {
		stamps[call.Caller] = call.Clock
	}
	assert.Equal(t, map[string]string{
		"claudia@test.de": "07:25", "admin@test.de": "07:58", "katarzyna@test.de": "08:25",
		"julia@test.de": "07:58", "markus@test.de": "07:58", "heike@test.de": "11:31",
	}, stamps, "staff stamp in at their shift start; sick leave, a cancelled shift and staff without a login are skipped")

	waveOf := map[int]string{1: "12:00", 2: "12:00", 3: "12:15", 4: "12:35"}
	nextWave := map[string]string{"12:00": "12:15", "12:15": "12:35", "12:35": "12:40"}
	state := clockedDayState("")
	for _, student := range state.Students {
		id := student.ID
		if id == mockSickChild || id == mockAbsentChild || sick[id] {
			assert.Empty(t, scansOf(calls, id, "confirm"), "child %d does not come today", id)
			assert.Empty(t, checkInsOf(calls, id), "child %d does not come today", id)
			continue
		}
		var afternoon []string
		for _, scan := range checkInsOf(calls, id) {
			if scan >= "11:00" {
				afternoon = append(afternoon, strings.Fields(scan)[0])
			}
		}
		require.NotEmpty(t, afternoon, "child %d of %s arrives", id, student.Class)
		if student.Class == "Klasse 4b" {
			assert.True(t, strings.HasPrefix(afternoon[0], "11:0"), "Klasse 4b has no lessons after 11:00: %v", afternoon)
			continue
		}
		wave := waveOf[classGrade(student.Class)]
		assert.True(t, afternoon[0] >= wave && afternoon[0] < nextWave[wave], "child %d of %s arrives with its wave at %s, not %s", id, student.Class, wave, afternoon[0])
	}

	var pickups []string
	for _, call := range callsTo(calls, "/api/iot/attendance/toggle") {
		if call.Body["action"] == "confirm_daily_checkout" {
			pickups = append(pickups, call.Clock)
		}
	}
	assert.Equal(t, slices.Repeat([]string{"12:45"}, earlyPickupCount), pickups, "a few first graders go home after lunch")
}

// The Frühbetreuung and the Mittagessen start as planned blocks at their
// planned time, with their planned staff; duties, cancelled and later blocks
// stay as they are.
func TestRunFullDay_StartsThePlannedBlocksAtTheirTime(t *testing.T) {
	t.Parallel()

	calls, _ := runClockedDay(t)

	lifecycle := map[string]string{}
	for _, call := range calls {
		if strings.HasPrefix(call.Path, "/api/timetable/instances/") && call.Method == "POST" {
			lifecycle[strings.TrimPrefix(call.Path, "/api/timetable/instances/")] = call.Clock
		}
	}
	assert.Equal(t, "07:30", lifecycle[fmt.Sprintf("%d/start", mockEarlyCare)])
	assert.Equal(t, "07:58", lifecycle[fmt.Sprintf("%d/complete", mockEarlyCare)], "the early care ends when the children go to class")
	assert.Equal(t, "12:00", lifecycle[fmt.Sprintf("%d/start", mockLunch)])
	for _, id := range []int{mockLunchDuty, mockLearningTime, mockCancelled} {
		assert.NotContains(t, lifecycle, fmt.Sprintf("%d/start", id), "block %d is not started", id)
	}

	for _, call := range callsTo(calls, "/api/iot/session/start") {
		assert.NotEqual(t, "07:30", call.Clock, "no kiosk session beside the planned early care")
	}
	for id := firstEarlyMockChild; id < mockSickChild; id++ {
		assert.Equal(t, []string{fmt.Sprintf("07:%02d %d", 28+id, mockOGSRoom)}, checkInsOf(calls, id)[:1],
			"early child %d arrives one per minute in the block's room", id)
		assert.Equal(t, []string{fmt.Sprintf("07:%02d claudia@test.de", 28+id)}, blockCallsOf(calls, mockEarlyCare, id, "check-in"),
			"the substitute on the block checks child %d in, as a planned block has no kiosk", id)
		assert.Equal(t, []string{"07:58 claudia@test.de"}, blockCallsOf(calls, mockEarlyCare, id, "check-out"),
			"and out again when child %d goes to class", id)
	}
	assert.Empty(t, checkInsOf(calls, mockSickChild), "the sick child of the roster stays home")
	for _, call := range callsTo(calls, "/api/iot/attendance/toggle") {
		assert.GreaterOrEqual(t, call.Clock, "11:00", "no kiosk confirms an arrival while only the early care runs")
	}
}

// A kiosk books the children of its own room: every scan into a room comes
// from the kiosk whose session runs there, and the Mittagessen's children
// are checked in on the block by the caregiver who eats with them.
func TestRunFullDay_EachRoomTakesItsChildrenTheProductWay(t *testing.T) {
	t.Parallel()

	calls, _ := runClockedDay(t)

	sessionRoom := map[string]any{}
	for _, call := range calls {
		switch {
		case call.Path == "/api/iot/session/start":
			sessionRoom[call.Caller] = call.Body["room_id"]
		case call.Path == "/api/iot/session/end":
			delete(sessionRoom, call.Caller)
		case call.Path == "/api/iot/checkin" && call.Body["action"] == "checkin" && call.Body["room_id"] != nil:
			assert.Equal(t, call.Body["room_id"], sessionRoom[call.Caller], "%s scan of %v at the kiosk of its room", call.Clock, call.Body["student_rfid"])
		}
	}

	lunch := 0
	for id := firstMockChild; id <= lastMockChild; id++ {
		for _, booked := range blockCallsOf(calls, mockLunch, id, "check-in") {
			assert.True(t, strings.HasSuffix(booked, " julia@test.de"), "child %d: %s", id, booked)
			lunch++
		}
	}
	assert.Positive(t, lunch, "the Mittagessen takes its roster on the block")
}

// Only caregivers with a shift that covers the session supervise, and none who
// is planned on a block meanwhile. Everybody opens the afternoon together.
func TestRunFullDay_SupervisorsHaveAShiftAndNoParallelBlock(t *testing.T) {
	t.Parallel()

	calls, _ := runClockedDay(t)

	var starts []string
	for _, call := range callsTo(calls, "/api/iot/session/start") {
		starts = append(starts, fmt.Sprintf("%s %v %v", call.Clock, call.Body["room_id"], call.Body["supervisor_ids"]))
	}
	assert.Equal(t, []string{
		fmt.Sprintf("11:00 %d [10]", mockOGSRoom),
		fmt.Sprintf("12:00 %d [11]", mockCreativeRoom),
		fmt.Sprintf("12:00 %d [13]", mockOGSRoom),
		fmt.Sprintf("12:00 %d [15]", mockReadingRoom),
	}, starts, "Julia takes the early class until her Mittagessen; Sandra is sick, Nicole's shift is cancelled")

	var block *stampedCall
	for i, call := range calls {
		if call.Path == "/api/timetable/instances" && call.Method == "POST" && call.Body["title"] == openRoomBlockTitle {
			block = &calls[i]
		}
	}
	require.NotNil(t, block)
	assert.Equal(t, []any{10.0}, block.Body["staff_ids"], "the 14:00 block goes to a caregiver on shift without a session or block then")
}

// Children follow today's plan: a full room passes them on, the early class
// moves on at noon, and an admitted child the seed state does not list comes
// with its class.
func TestRunFullDay_PlacesChildrenByTodaysPlan(t *testing.T) {
	t.Parallel()

	calls, _, sick := runClockedDayWithSick(t)

	mensa := 0
	for id := firstMockChild; id <= lastMockChild; id++ {
		scans := checkInsOf(calls, id)
		if len(scans) > 0 && strings.HasSuffix(scans[len(scans)-1], fmt.Sprint(mockMensa)) {
			mensa++
			assert.LessOrEqual(t, id, int64(40), "only children of the Mittagessen roster eat in the Mensa")
		}
	}
	assert.Equal(t, mockMensaPlaces, mensa, "the Mensa fills up, the rest of its roster goes to the other rooms")

	state := clockedDayState("")
	for _, student := range state.Students {
		if student.Class != "Klasse 4b" || sick[student.ID] {
			continue
		}
		scans := checkInsOf(calls, student.ID)
		require.Len(t, scans, 2, "child %d of Klasse 4b", student.ID)
		assert.True(t, strings.HasPrefix(scans[0], "11:0"), "Klasse 4b arrives at 11:00: %v", scans)
		assert.True(t, strings.HasPrefix(scans[1], "12:00"), "and moves on when the afternoon opens: %v", scans)
	}

	assert.NotEmpty(t, callsTo(calls, fmt.Sprintf("/api/students/%d/rfid", mockAPIOnlyChild)), "the admitted child gets a tag")
	require.NotEmpty(t, checkInsOf(calls, mockAPIOnlyChild))
	assert.Equal(t, "12:0", checkInsOf(calls, mockAPIOnlyChild)[0][:4], "and arrives with Klasse 1a")
}

// The day leaves messages at the times of its waves: colleagues on shift
// write to the school admin, parents and the OGS write to each other.
func TestRunFullDay_WritesMessagesDuringTheDay(t *testing.T) {
	t.Parallel()

	calls, _, sick := runClockedDayWithSick(t)

	early := 0
	for _, student := range clockedDayState("").Students {
		if student.Class == "Klasse 4b" && !sick[student.ID] {
			early++
		}
	}
	var team []string
	for _, call := range callsTo(calls, fmt.Sprintf("/api/staff-messages/threads/%d", mockTeamThread)) {
		team = append(team, call.Clock+" "+call.Caller+" "+fmt.Sprint(call.Body["body"]))
	}
	assert.Equal(t, []string{
		"07:45 claudia@test.de Guten Morgen! Die Frühbetreuung läuft, 5 Kinder sind schon da.",
		fmt.Sprintf("11:31 julia@test.de Kurze Info: %d Kinder sind schon bei mir, der Unterricht ist ausgefallen.", early),
		fmt.Sprintf("12:40 markus@test.de Alle Klassen sind da, %d Kinder sind heute bei uns.", 100-6+1),
	}, team)
	for _, call := range callsTo(calls, "/api/staff-messages/threads/open") {
		assert.Equal(t, "900", call.Body["account_id"], "the colleagues write to the school admin")
	}

	var parents []string
	for _, call := range calls {
		if strings.HasPrefix(call.Path, "/parent/me/messages/children/") || call.Path == fmt.Sprintf("/api/messages/threads/%d", mockParentThread) {
			parents = append(parents, fmt.Sprintf("%s %s %s %s %v", call.Clock, call.Method, call.Path, call.Caller, call.Body["handled_up_to_message_id"]))
		}
	}
	assert.Equal(t, []string{
		"08:30 POST /parent/me/messages/children/9 eltern9@test.de <nil>",
		"12:46 GET /parent/me/messages/children/1 sabine@test.de <nil>",
		"12:46 GET /api/messages/threads/41 katarzyna@test.de <nil>",
		"12:46 POST /api/messages/threads/41 katarzyna@test.de 7",
		"12:50 GET /api/messages/threads/41 katarzyna@test.de <nil>",
		"12:50 POST /api/messages/threads/41 katarzyna@test.de 7",
		"12:50 POST /parent/me/messages/children/10 eltern10@test.de <nil>",
	}, parents, "the OGS opens each conversation and answers up to the newest message it shows; the first parent's chat ends with a recent OGS answer")
}

// Without a clock setter every wave runs at once on the plan's times, so a
// local run in the evening still starts the planned blocks and the sessions.
func TestRunFullDay_WithoutClockFollowsThePlan(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: berlinTime(t, "2026-10-20 20:15")}
	srv, recorded := clockedDayServer(t, clock)
	statePath := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, WriteSeedState(clockedDayState(srv.URL), statePath))
	require.NoError(t, RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath, Now: clock.Now}))

	calls := recorded()
	assert.Empty(t, clock.Moves())
	assert.NotEmpty(t, callsTo(calls, fmt.Sprintf("/api/timetable/instances/%d/start", mockEarlyCare)))
	assert.NotEmpty(t, callsTo(calls, fmt.Sprintf("/api/timetable/instances/%d/start", mockLunch)))
	assert.Len(t, callsTo(calls, "/api/iot/session/start"), 4)
	assert.NotEmpty(t, checkInsOf(calls, 1))
}

func TestPlanArrivals_FollowsClassWavesAndExceptions(t *testing.T) {
	t.Parallel()

	children := []todayChild{
		{SeedStudent: SeedStudent{ID: 1, Class: "Klasse 3b"}},
		{SeedStudent: SeedStudent{ID: 2, Class: "Klasse 1a"}},
		{SeedStudent: SeedStudent{ID: 3, Class: "Klasse 4b"}, ArrivalException: at(11, 0)},
		{SeedStudent: SeedStudent{ID: 4, Class: "Klasse 2b"}, Absent: true},
		{SeedStudent: SeedStudent{ID: 5, Class: "Gast"}},
	}
	plan := planArrivals(children)

	var got []string
	for _, arrival := range plan.pending {
		got = append(got, fmt.Sprintf("%s %d", arrival.at, arrival.childID))
	}
	assert.Equal(t, []string{"11:00 3", "12:00 2", "12:15 1", "12:35 5"}, got,
		"an exception sets the time, an absent child never comes, a class without a grade comes last")
	assert.Equal(t, []int64{2}, plan.pickups)
	assert.Equal(t, []int64{5}, plan.unterwegs)
}

func TestArrivalPlan_WaitQueuesBeforeTheChildrenOfThatTime(t *testing.T) {
	t.Parallel()

	plan := arrivalPlan{pending: []scheduledArrival{{at: at(11, 0), childID: 3}, {at: at(12, 0), childID: 2}, {at: at(12, 15), childID: 1}}}
	plan.pending = plan.pending[1:] // child 3 arrives, but no room is open
	plan.wait(3, sessionsStart)

	var got []string
	for _, arrival := range plan.pending {
		got = append(got, fmt.Sprintf("%s %d", arrival.at, arrival.childID))
	}
	assert.Equal(t, []string{"12:00 3", "12:00 2", "12:15 1"}, got, "the waiting child comes first when the afternoon rooms open")
}

func TestRoomPlan_TriesTheBlockRoomFirstThenTheSessionsInTurn(t *testing.T) {
	t.Parallel()

	plan := newRoomPlan()
	plan.openBlock(dayBlock{RoomID: 8, StudentIDs: []int64{1}})
	plan.openSession(6, "d1")
	plan.openSession(7, "d2")

	assert.Equal(t, []int64{8, 6, 7}, plan.candidates(1))
	assert.Equal(t, map[int64]string{6: "d1", 7: "d2"}, plan.sessionDevice, "each session room knows its kiosk")
	assert.Equal(t, []int64{7, 6}, plan.candidates(2))
	assert.Equal(t, []int64{6, 7}, plan.candidates(3))
	assert.True(t, plan.isOpen(8))

	plan.openBlock(dayBlock{RoomID: 9})
	assert.True(t, plan.isOpen(9), "a block without a roster still holds its room")

	plan.closeBlock(dayBlock{RoomID: 8, StudentIDs: []int64{1}})
	assert.False(t, plan.isOpen(8))
}

func TestShiftCovers_NeedsAStartedShiftThatHasNotEnded(t *testing.T) {
	t.Parallel()

	shift := plannedShift{StartTime: "08:00:00", EndTime: "16:30:00"}
	assert.True(t, shiftCovers(shift, at(7, 58)), "five minutes early count")
	assert.True(t, shiftCovers(shift, at(12, 0)))
	assert.False(t, shiftCovers(shift, at(7, 50)))
	assert.False(t, shiftCovers(shift, at(16, 30)))
	assert.False(t, shiftCovers(plannedShift{StartTime: "08:00", Cancelled: true}, at(12, 0)))
	assert.True(t, shiftCovers(plannedShift{StartTime: "08:00"}, at(23, 0)), "a shift without an end runs on")
}

// Mock child IDs of the simulated school: 1 to 100; the early children start
// at 2.
const (
	firstMockChild      int64 = 1
	firstEarlyMockChild int64 = 2
	lastMockChild       int64 = 100
)

func TestMergeChildren_TheCarePlanCountsOnlyOnACareDay(t *testing.T) {
	t.Parallel()

	seeded := []SeedStudent{{ID: 1}, {ID: 2}, {ID: 3}}
	careDay := mergeChildren(seeded, []apiChild{
		{ID: "1", DayPlanningStatus: "comes_today"},
		{ID: "2", DayPlanningStatus: "not_coming_today"},
		{ID: "3", DayPlanningStatus: "comes_today", Sick: true},
	})
	assert.Equal(t, []bool{false, true, true}, absences(careDay), "a care day keeps the plan")

	weekend := mergeChildren(seeded, []apiChild{
		{ID: "1", DayPlanningStatus: "not_coming_today"},
		{ID: "2", DayPlanningStatus: "not_coming_today"},
		{ID: "3", DayPlanningStatus: "not_coming_today", Sick: true},
	})
	assert.Equal(t, []bool{false, false, true}, absences(weekend), "a day that expects nobody says nothing about who comes; sick children stay home")
}

func absences(children []todayChild) []bool {
	absent := make([]bool, len(children))
	for i, child := range children {
		absent[i] = child.Absent
	}
	return absent
}
