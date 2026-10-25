package simulate

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

// The mock school day: room and block IDs the assertions name.
const (
	mockCreativeRoom = 6
	mockOGSRoom      = 7
	mockMensa        = 8
	mockReadingRoom  = 9
	mockMensaPlaces  = 20
	mockEarlyCare    = 501
	mockLunch        = 502
	mockLunchDuty    = 503
	mockLearningTime = 504
	mockCancelled    = 505
	mockSickChild    = 7   // on the early care roster, reported sick
	mockAbsentChild  = 8   // no care plan for today
	mockAPIOnlyChild = 200 // an admitted child the seed state does not list
	mockTeamThread   = 31
	mockParentThread = 41
	mockParentMsg    = 7
)

// mockShifts are today's shifts: the Frühdienst at 07:30, the regular Dienst
// at 08:00 and 08:30, a late shift, a cancelled one, and staff without a
// login.
var mockShifts = []map[string]any{
	{"staff_id": 1, "start_time": "08:00:00", "end_time": "16:30:00"},
	{"staff_id": 2, "start_time": "08:30:00", "end_time": "17:00:00"},
	{"staff_id": 10, "start_time": "08:00:00", "end_time": "16:30:00"},
	{"staff_id": 11, "start_time": "08:00:00", "end_time": "16:30:00"},
	{"staff_id": 12, "start_time": "08:00:00", "end_time": "16:30:00"},
	{"staff_id": 13, "start_time": "11:30:00", "end_time": "17:00:00"},
	{"staff_id": 14, "start_time": "08:00:00", "end_time": "16:30:00", "cancelled": true},
	{"staff_id": 15, "start_time": "07:30:00", "end_time": "16:00:00"},
	{"staff_id": 99, "start_time": "08:00:00", "end_time": "16:30:00"},
}

// mockBlocks are today's timetable blocks. Markus (11) is away on the
// Frühbetreuung and Claudia (15) stands in; Julia (10) eats with the
// children, Sandra (12) has the Essensausgabe.
func mockBlocks() []map[string]any {
	ids := func(from, to int) []int {
		var out []int
		for id := from; id <= to; id++ {
			out = append(out, id)
		}
		return out
	}
	staff := func(id int, absent bool) map[string]any { return map[string]any{"staff_id": id, "is_absent": absent} }
	return []map[string]any{
		{"id": mockEarlyCare, "title": "Frühbetreuung", "start_time": "07:30", "end_time": "08:30", "status": "planned", "activity_type": "care",
			"room_id": mockOGSRoom, "staff": []any{staff(11, true), staff(15, false)}, "student_ids": ids(2, mockSickChild)},
		{"id": mockLunch, "title": "Mittagessen", "start_time": "12:00", "end_time": "13:00", "status": "planned", "activity_type": "care",
			"room_id": mockMensa, "staff": []any{staff(10, false)}, "student_ids": ids(1, 40)},
		{"id": mockLunchDuty, "title": "Essensausgabe", "start_time": "12:00", "end_time": "13:00", "status": "planned", "activity_type": "duty",
			"room_id": mockMensa, "staff": []any{staff(12, false)}},
		{"id": mockLearningTime, "title": "Lernzeit", "start_time": "13:00", "end_time": "14:00", "status": "planned", "activity_type": "care",
			"room_id": mockReadingRoom, "staff": []any{staff(13, false)}, "student_ids": ids(1, 10)},
		{"id": mockCancelled, "title": "Kunst-AG", "start_time": "12:00", "end_time": "13:00", "status": "cancelled", "activity_type": "activity",
			"room_id": mockCreativeRoom, "staff": []any{staff(15, false)}},
	}
}

// mockChildren is GET /api/students: one child sick, one without a plan for
// today, Klasse 4b with lessons cancelled until 11:00, and an admitted child
// the seed state does not know.
func mockChildren(state *SeedState) []map[string]any {
	rows := make([]map[string]any, 0, len(state.Students)+1)
	for _, student := range state.Students {
		row := map[string]any{"id": student.ID, "first_name": student.FirstName, "school_class": student.Class, "day_planning_status": "comes_today"}
		switch {
		case student.ID == mockSickChild:
			row["sick"], row["day_planning_status"] = true, "not_coming_today"
		case student.ID == mockAbsentChild:
			row["day_planning_status"] = "not_coming_today"
		case student.Class == "Klasse 4b":
			row["arrival_time"], row["arrival_is_exception"] = "11:00", true
		}
		rows = append(rows, row)
	}
	return append(rows, map[string]any{"id": mockAPIOnlyChild, "first_name": "Lina", "school_class": "Klasse 1a", "day_planning_status": "comes_today"})
}

// mockDay is the server state the mock keeps: who is in which room, which
// kiosk runs a session in which room, and which blocks run.
type mockDay struct {
	mu       sync.Mutex
	room     map[string]float64
	sessions map[string]float64
	running  map[int]bool
	calls    []stampedCall
	failed   []stampedCall
}

func (d *mockDay) inRoom(room float64) int {
	n := 0
	for _, r := range d.room {
		if r == room {
			n++
		}
	}
	return n
}

// clockedDayServer answers the full-day API on a clock. The token of a login
// is its e-mail address, so every call names its caller. The Mensa holds
// mockMensaPlaces children.
func clockedDayServer(t *testing.T, clock *fakeClock) (*simulationHTTPTestServer, func() []stampedCall) {
	t.Helper()
	day := &mockDay{room: map[string]float64{}, sessions: map[string]float64{}, running: map[int]bool{}}
	var state *SeedState
	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		call := stampedCall{Method: r.Method, Path: r.URL.Path, Clock: clock.Now().In(simulationBerlin).Format("15:04"),
			Caller: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), Body: body}
		day.mu.Lock()
		defer day.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		status, data := day.answer(call, state)
		if status != simulationHTTPStatusOK {
			day.failed = append(day.failed, call)
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(data)
			return
		}
		day.calls = append(day.calls, call)
		w.WriteHeader(status)
		if call.Path == "/auth/login" || call.Path == "/parent/auth/login" {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": fmt.Sprint(body["email"])})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
	})
	t.Cleanup(srv.Close)
	state = clockedDayState(srv.URL)
	return srv, func() []stampedCall {
		day.mu.Lock()
		defer day.mu.Unlock()
		return append([]stampedCall(nil), day.calls...)
	}
}

// answer returns the status and data of one call; the caller holds the lock.
func (d *mockDay) answer(call stampedCall, state *SeedState) (int, any) {
	switch {
	case call.Path == "/health":
		return simulationHTTPStatusOK, "OK"
	case call.Path == "/api/staff-shifts":
		return simulationHTTPStatusOK, mockShifts
	case call.Path == "/api/timetable/instances" && call.Method == "GET":
		return simulationHTTPStatusOK, map[string]any{"instances": mockBlocks()}
	case call.Path == "/api/students" && call.Method == "GET":
		return simulationHTTPStatusOK, mockChildren(state)
	case call.Path == "/api/time-tracking/absences":
		if call.Caller == "sandra@test.de" {
			return simulationHTTPStatusOK, []map[string]any{{"id": 5, "absence_type": "sick"}}
		}
		return simulationHTTPStatusOK, []any{}
	case call.Path == "/api/iot/session/start":
		room, _ := call.Body["room_id"].(float64)
		d.sessions[call.Caller] = room
		return simulationHTTPStatusOK, map[string]any{"active_group_id": 1}
	case call.Path == "/api/iot/session/end":
		delete(d.sessions, call.Caller)
		return simulationHTTPStatusOK, map[string]any{}
	case call.Path == "/api/iot/attendance/toggle":
		if _, ok := d.sessions[call.Caller]; !ok {
			// The device API attributes the toggle to the supervisor of the
			// kiosk's session (authorizeIoTDeviceToggle).
			return simulationHTTPStatusInternalServerError, map[string]string{"code": "general.server"}
		}
		return d.scan(call)
	case call.Path == "/api/iot/checkin":
		return d.scan(call)
	case strings.HasPrefix(call.Path, "/api/timetable/operations/instances/"):
		return d.blockAttendance(call, state)
	case strings.HasPrefix(call.Path, "/api/timetable/instances/") && call.Method == "POST":
		var id int
		var transition string
		if _, err := fmt.Sscanf(call.Path, "/api/timetable/instances/%d/%s", &id, &transition); err == nil {
			d.running[id] = transition == "start"
		}
		return simulationHTTPStatusOK, map[string]any{"id": 77}
	case call.Path == "/api/timetable/periods/bootstrap":
		return simulationHTTPStatusOK, map[string]any{"periods": []map[string]any{{"start_date": "2000-01-01", "end_date": "2100-12-31", "is_active": true}}}
	case call.Path == "/api/staff-messages/recipients":
		return simulationHTTPStatusOK, []map[string]any{{"account_id": "900", "name": "Katarzyna Schmidt"}, {"account_id": "901", "name": "Julia Klein"}}
	case call.Path == "/api/staff-messages/threads/open":
		return simulationHTTPStatusOK, map[string]any{"thread_id": fmt.Sprint(mockTeamThread)}
	case call.Path == fmt.Sprintf("/api/messages/threads/%d", mockParentThread) && call.Method == "GET":
		return simulationHTTPStatusOK, map[string]any{"thread_id": fmt.Sprint(mockParentThread), "messages": []map[string]any{{"id": fmt.Sprint(mockParentMsg)}}}
	case call.Path == fmt.Sprintf("/api/messages/threads/%d", mockParentThread) && call.Body["handled_up_to_message_id"] != fmt.Sprint(mockParentMsg):
		// A parent wrote in the thread: a reply names the newest message the
		// staff member saw (requireHandledBoundary).
		return simulationHTTPStatusConflict, map[string]string{"code": "general.business_rejection"}
	case strings.HasPrefix(call.Path, "/parent/me/messages/children/"):
		return simulationHTTPStatusOK, map[string]any{"thread_id": fmt.Sprint(mockParentThread), "messages": []map[string]any{{"id": fmt.Sprint(mockParentMsg)}}}
	default:
		return simulationHTTPStatusOK, map[string]any{"id": 77}
	}
}

// blockAttendance books a check-in or check-out on a running block. Its
// planned caregivers and the admin may book the children of its roster; the
// Mensa fills up. A block the mock does not plan (the open-room block) is
// booked by the admin.
func (d *mockDay) blockAttendance(call stampedCall, state *SeedState) (int, any) {
	var id, studentID int
	var action string
	if _, err := fmt.Sscanf(call.Path, "/api/timetable/operations/instances/%d/students/%d/%s", &id, &studentID, &action); err != nil || !d.running[id] {
		return simulationHTTPStatusConflict, map[string]string{"code": "timetable.instance_not_active"}
	}
	room, staff, roster := 10.0, []any{}, []int{studentID}
	for _, block := range mockBlocks() {
		if block["id"] == id {
			room, staff = float64(block["room_id"].(int)), block["staff"].([]any)
			roster, _ = block["student_ids"].([]int)
		}
	}
	allowed := call.Caller == state.Accounts.Admin[0].Email
	for _, planned := range staff {
		entry := planned.(map[string]any)
		for _, account := range state.Accounts.Betreuer {
			allowed = allowed || (account.Email == call.Caller && int(account.StaffID) == entry["staff_id"] && entry["is_absent"] == false)
		}
	}
	if !allowed || !slices.Contains(roster, studentID) {
		return simulationHTTPStatusForbidden, map[string]string{"code": "timetable.operation_forbidden"}
	}
	tag := tagOf(int64(studentID))
	if action == "check-out" {
		delete(d.room, tag)
		return simulationHTTPStatusOK, map[string]any{}
	}
	if room == mockMensa && d.room[tag] != mockMensa && d.inRoom(mockMensa) >= mockMensaPlaces {
		return simulationHTTPStatusConflict, map[string]string{"code": codeBlockRoomCapacityExceeded}
	}
	d.room[tag] = room
	return simulationHTTPStatusOK, map[string]any{}
}

// scan books the kiosk scans into the rooms; the Mensa fills up.
func (d *mockDay) scan(call stampedCall) (int, any) {
	tag := fmt.Sprint(call.Body["student_rfid"])
	switch {
	case tag == "DEMO-UNREGISTERED-TAG":
		return simulationHTTPStatusNotFound, map[string]string{"code": codeRFIDTagNotFound}
	case call.Body["action"] == "checkin":
		room, _ := call.Body["room_id"].(float64)
		if room == mockMensa && d.room[tag] != mockMensa && d.inRoom(mockMensa) >= mockMensaPlaces {
			return simulationHTTPStatusConflict, map[string]string{"code": codeRoomCapacityExceeded}
		}
		d.room[tag] = room
	case call.Body["action"] == "checkout" || call.Body["action"] == "confirm_daily_checkout":
		delete(d.room, fmt.Sprint(call.Body["rfid"]))
		delete(d.room, tag)
	}
	return simulationHTTPStatusOK, map[string]any{"id": 1}
}

// clockedDayState is a school of 100 children in Klassen 1a to 4b, five
// caregivers, the school admin of the screenshots and four parents.
func clockedDayState(baseURL string) *SeedState {
	students := make([]SeedStudent, 100)
	for i := range students {
		students[i] = SeedStudent{ID: int64(i + 1), FirstName: fmt.Sprintf("Kind%d", i+1), LastName: fmt.Sprint(i), Class: fmt.Sprintf("Klasse %d%s", i%4+1, []string{"a", "b"}[i/50])}
	}
	return &SeedState{
		BaseURL:   baseURL,
		DevicePIN: "1234",
		Bootstrap: SeedStateBootstrap{TenantSlug: "sonnenhof"},
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{
				{Email: "admin@test.de", Password: "p", StaffID: 1},
				{Key: teamChatLeadKey, Email: "katarzyna@test.de", Password: "p", StaffID: 2, Name: "Katarzyna Schmidt"},
			},
			Betreuer: []AccountCredentials{
				{Email: "julia@test.de", Password: "p", StaffID: 10, Name: "Julia Klein"},
				{Email: "markus@test.de", Password: "p", StaffID: 11, Name: "Markus Wolf"},
				{Email: "sandra@test.de", Password: "p", StaffID: 12, Name: "Sandra Schröder"},
				{Email: "heike@test.de", Password: "p", StaffID: 13, Name: "Heike Hartmann"},
				{Email: "nicole@test.de", Password: "p", StaffID: 14, Name: "Nicole Schwarz"},
				{Email: "claudia@test.de", Password: "p", StaffID: 15, Name: "Claudia Wagner"},
			},
		},
		Parents: []ParentCredentials{
			{Email: "sabine@test.de", Password: "p", Name: "Sabine Schneider", StudentIDs: []int64{1}},
			{Email: "thomas@test.de", Password: "p", Name: "Thomas Schneider", StudentIDs: []int64{1}},
			{Email: "eltern9@test.de", Password: "p", StudentIDs: []int64{9}},
			{Email: "eltern10@test.de", Password: "p", StudentIDs: []int64{10}},
		},
		Devices: map[string]SeedDevice{
			"d1": {APIKey: "key-1", Name: "Scanner 1"}, "d2": {APIKey: "key-2", Name: "Scanner 2"}, "d3": {APIKey: "key-3", Name: "Scanner 3"},
		},
		Students: students,
		Rooms: map[string]int64{
			"Kreativraum": mockCreativeRoom, "OGS-Raum 1": mockOGSRoom, "Mensa": mockMensa, "Leseecke": mockReadingRoom, "Sporthalle": 10,
		},
		Activities: map[string]int64{"Basteln": 61, "Lesen": 60, "Hausaufgaben": 62},
	}
}
