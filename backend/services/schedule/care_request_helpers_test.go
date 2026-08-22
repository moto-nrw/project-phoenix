package schedule

import (
	"errors"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// careDay is a small helper to build a weekday payload entry for the map-based
// jsonb shape the service validates.
func careDay(weekday int, mode, arrival, pickup string) map[string]any {
	e := map[string]any{"weekday": weekday}
	if mode != "" {
		e["mode"] = mode
	}
	if arrival != "" {
		e["arrival"] = arrival
	}
	if pickup != "" {
		e["pickup"] = pickup
	}
	return e
}

func carePayload(days ...map[string]any) map[string]any {
	return map[string]any{"weekdays": days}
}

// Pure-function tests for the care-request diff/label helpers (#1803). These
// carry no database dependency and pin the human-facing and structured
// representations of a day's departure modes that the staff diff and the
// localized parents portal both render.

func TestGermanAllowedDepartureModes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		modes []usersModels.DepartureMode
		want  string
	}{
		{
			name:  "empty set means the child goes home alone",
			modes: nil,
			want:  "Geht alleine",
		},
		{
			name:  "single mode renders its German label",
			modes: []usersModels.DepartureMode{usersModels.DepartureBus},
			want:  "Fährt Bus",
		},
		{
			name: "multiple modes are joined with a slash in order",
			modes: []usersModels.DepartureMode{
				usersModels.DepartureBus,
				usersModels.DeparturePickup,
			},
			want: "Fährt Bus / Wird abgeholt",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := germanAllowedDepartureModes(tc.modes); got != tc.want {
				t.Errorf("germanAllowedDepartureModes(%v) = %q, want %q", tc.modes, got, tc.want)
			}
		})
	}
}

func TestDepartureModeKeys(t *testing.T) {
	t.Parallel()

	// An empty set must resolve to the explicit "alone" key, never an
	// ambiguous empty slice a localized client would render as blank.
	if got := departureModeKeys(nil); len(got) != 1 || got[0] != string(usersModels.DepartureAlone) {
		t.Errorf("departureModeKeys(nil) = %v, want [%q]", got, usersModels.DepartureAlone)
	}

	got := departureModeKeys([]usersModels.DepartureMode{
		usersModels.DepartureBus,
		usersModels.DeparturePickup,
	})
	want := []string{string(usersModels.DepartureBus), string(usersModels.DeparturePickup)}
	if len(got) != len(want) {
		t.Fatalf("departureModeKeys len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("departureModeKeys[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCareDashIfEmpty(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":            "—",
		"   ":         "—", // whitespace-only collapses to the em dash placeholder
		"07:30":       "07:30",
		" Fährt Bus ": " Fährt Bus ", // non-empty content is returned verbatim
	}
	for in, want := range cases {
		if got := careDashIfEmpty(in); got != want {
			t.Errorf("careDashIfEmpty(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsCareRequestPendingUniqueViolation(t *testing.T) {
	t.Parallel()

	if isCareRequestPendingUniqueViolation(nil) {
		t.Error("isCareRequestPendingUniqueViolation(nil) = true, want false")
	}
	// A generic, unrelated error is not the partial-unique-index violation.
	if isCareRequestPendingUniqueViolation(errors.New("boom")) {
		t.Error("isCareRequestPendingUniqueViolation(generic) = true, want false")
	}
}

// buildCareScheduleChanges is the SINGLE validator shared by create-time
// validation and approve-time apply, so its accept/reject decisions are the
// business contract both paths depend on.
func TestBuildCareScheduleChanges_Valid(t *testing.T) {
	t.Parallel()

	changes, err := buildCareScheduleChanges(carePayload(
		careDay(1, "bus", "07:30", "15:00"),
		careDay(5, "pickup", "", "16:30"),
	))
	if err != nil {
		t.Fatalf("buildCareScheduleChanges valid = %v", err)
	}
	if changes.modes[usersModels.PickupDayMonday] != usersModels.DepartureBus {
		t.Errorf("monday mode = %q, want bus", changes.modes[usersModels.PickupDayMonday])
	}
	if changes.arrivals[1] != "07:30" {
		t.Errorf("monday arrival = %q, want 07:30", changes.arrivals[1])
	}
	if changes.pickups[1] != "15:00" {
		t.Errorf("monday pickup = %q, want 15:00", changes.pickups[1])
	}
	if changes.modes[usersModels.PickupDayFriday] != usersModels.DeparturePickup {
		t.Errorf("friday mode = %q, want pickup", changes.modes[usersModels.PickupDayFriday])
	}
	// Friday carried no arrival, so it must not appear in the arrivals bucket.
	if _, ok := changes.arrivals[5]; ok {
		t.Error("friday arrival should be absent")
	}
}

func TestBuildCareScheduleChanges_CareDays(t *testing.T) {
	t.Parallel()

	active := careDay(2, "pickup", "", "15:30")
	active["scheduled"] = true
	inactive := careDay(4, "", "", "")
	inactive["scheduled"] = false

	changes, err := buildCareScheduleChanges(carePayload(active, inactive))
	if err != nil {
		t.Fatalf("buildCareScheduleChanges care days = %v", err)
	}
	if !changes.scheduled[2] || changes.scheduled[4] {
		t.Fatalf("scheduled changes = %v, want Tuesday active and Thursday inactive", changes.scheduled)
	}
}

func TestBuildCareScheduleChanges_Rejections(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"weekday below range", carePayload(careDay(0, "bus", "", ""))},
		{"weekend weekday", carePayload(careDay(6, "bus", "", ""))},
		{"unknown departure mode", carePayload(careDay(1, "teleport", "", ""))},
		// "accompanied" is a real DepartureMode but is intentionally NOT an
		// allowed parent-request target (safety-sensitive, #1694) — the switch
		// must reject it rather than fall through.
		{"accompanied mode not requestable", carePayload(careDay(1, "accompanied", "", ""))},
		{"midnight arrival (reads as 'no time')", carePayload(careDay(1, "", "00:00", ""))},
		{"midnight pickup", carePayload(careDay(1, "", "", "00:00"))},
		{"malformed arrival", carePayload(careDay(1, "", "7:3", ""))},
		{"out-of-range clock", carePayload(careDay(1, "", "25:00", ""))},
		{"active day without plan", carePayload(map[string]any{"weekday": 1, "scheduled": true})},
		{"inactive day with pickup", carePayload(map[string]any{"weekday": 1, "scheduled": false, "pickup": "15:00"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildCareScheduleChanges(tc.payload)
			if !errors.Is(err, ErrInvalidCareRequestPayload) {
				t.Errorf("buildCareScheduleChanges(%s) err = %v, want ErrInvalidCareRequestPayload", tc.name, err)
			}
		})
	}
}

// canonicalizeCareSchedulePayload sanitizes a direct API client's payload:
// duplicate/out-of-order weekday rows collapse into one entry per weekday in
// fixed Mon..Fri order, and a payload with no actual change is rejected.
func TestCanonicalizeCareSchedulePayload_DedupsAndOrders(t *testing.T) {
	t.Parallel()

	canon, err := canonicalizeCareSchedulePayload(carePayload(
		careDay(5, "", "", "16:00"),    // Friday first (out of order)
		careDay(1, "bus", "07:30", ""), // Monday
		careDay(1, "", "", "15:00"),    // Monday again (merge pickup in)
	))
	if err != nil {
		t.Fatalf("canonicalize = %v", err)
	}
	weekdays, ok := canon["weekdays"].([]any)
	if !ok {
		t.Fatalf("weekdays type = %T, want []any", canon["weekdays"])
	}
	if len(weekdays) != 2 {
		t.Fatalf("weekday entries = %d, want 2 (Mon+Fri merged)", len(weekdays))
	}
	first := weekdays[0].(map[string]any)
	if int(first["weekday"].(float64)) != 1 {
		t.Errorf("first entry weekday = %v, want 1 (Monday, reordered first)", first["weekday"])
	}
	// The two Monday rows merged: mode + arrival from the first, pickup from the second.
	if first["mode"] != "bus" || first["arrival"] != "07:30" || first["pickup"] != "15:00" {
		t.Errorf("merged Monday = %v, want bus/07:30/15:00", first)
	}
}

func TestCanonicalizeCareSchedulePayload_RejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := canonicalizeCareSchedulePayload(carePayload())
	if !errors.Is(err, ErrInvalidCareRequestPayload) {
		t.Errorf("canonicalize(no weekdays) err = %v, want ErrInvalidCareRequestPayload", err)
	}
}

func TestParseCareWallClock(t *testing.T) {
	t.Parallel()

	tm, err := parseCareWallClock("07:30")
	if err != nil {
		t.Fatalf("parseCareWallClock(07:30) = %v", err)
	}
	if tm.Hour() != 7 || tm.Minute() != 30 {
		t.Errorf("parsed = %02d:%02d, want 07:30", tm.Hour(), tm.Minute())
	}
	// Midnight normalizes to the zero time.Time, which the schedule validators
	// treat as "no time set" — the reason build/canonicalize reject "00:00".
	if mid, err := parseCareWallClock("00:00"); err != nil || !mid.IsZero() {
		t.Errorf("parseCareWallClock(00:00) = (%v, %v), want (zero, nil)", mid, err)
	}
	if _, err := parseCareWallClock("nope"); err == nil {
		t.Error("parseCareWallClock(nope) err = nil, want parse error")
	}
}

func TestCareStructToMapRoundTrip(t *testing.T) {
	t.Parallel()

	in := careSchedulePayload{Weekdays: []careWeekdayPayload{{Weekday: 2, Mode: "bus", Arrival: "08:00"}}}
	m, err := careStructToMap(in)
	if err != nil {
		t.Fatalf("careStructToMap = %v", err)
	}
	out, err := decodeCarePayload[careSchedulePayload](m)
	if err != nil {
		t.Fatalf("decodeCarePayload = %v", err)
	}
	if len(out.Weekdays) != 1 || out.Weekdays[0].Weekday != 2 || out.Weekdays[0].Mode != "bus" {
		t.Errorf("round-trip = %+v, want weekday 2 / bus / 08:00", out.Weekdays)
	}
	// omitempty drops the untouched pickup field on the way through JSON.
	if out.Weekdays[0].Pickup != "" {
		t.Errorf("pickup = %q, want empty", out.Weekdays[0].Pickup)
	}
}
