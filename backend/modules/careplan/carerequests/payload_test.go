package carerequests_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

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

func carePayload(t *testing.T, days ...map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"weekdays": days})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBuildCareScheduleChanges_Valid(t *testing.T) {
	t.Parallel()

	changes, err := carerequests.ParseWeekly(carePayload(t,
		careDay(1, "bus", "07:30", "15:00"),
		careDay(5, "pickup", "", "16:30"),
	))
	if err != nil {
		t.Fatalf("carerequests.ParseWeekly valid = %v", err)
	}
	if changes.Modes["mon"] != "bus" {
		t.Errorf("monday mode = %q, want bus", changes.Modes["mon"])
	}
	if changes.Arrivals[1] != "07:30" {
		t.Errorf("monday arrival = %q, want 07:30", changes.Arrivals[1])
	}
	if changes.Pickups[1] != "15:00" {
		t.Errorf("monday pickup = %q, want 15:00", changes.Pickups[1])
	}
	if changes.Modes["fri"] != "pickup" {
		t.Errorf("friday mode = %q, want pickup", changes.Modes["fri"])
	}
	// Friday carried no arrival, so it must not appear in the arrivals bucket.
	if _, ok := changes.Arrivals[5]; ok {
		t.Error("friday arrival should be absent")
	}
}

func TestBuildCareScheduleChanges_CareDays(t *testing.T) {
	t.Parallel()

	active := careDay(2, "pickup", "", "15:30")
	active["scheduled"] = true
	inactive := careDay(4, "", "", "")
	inactive["scheduled"] = false

	changes, err := carerequests.ParseWeekly(carePayload(t, active, inactive))
	if err != nil {
		t.Fatalf("carerequests.ParseWeekly care days = %v", err)
	}
	if !changes.Scheduled[2] || changes.Scheduled[4] {
		t.Fatalf("scheduled changes = %v, want Tuesday active and Thursday inactive", changes.Scheduled)
	}
}

func TestBuildCareScheduleChanges_Rejections(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload json.RawMessage
	}{
		{"malformed JSON", json.RawMessage("{")},
		{"non-object payload", json.RawMessage("[]")},
		{"non-array weekdays", json.RawMessage(`{"weekdays":"invalid"}`)},
		{"wrong weekday type", json.RawMessage(`{"weekdays":[{"weekday":"1"}]}`)},
		{"weekday below range", carePayload(t, careDay(0, "bus", "", ""))},
		{"weekend weekday", carePayload(t, careDay(6, "bus", "", ""))},
		{"unknown departure mode", carePayload(t, careDay(1, "teleport", "", ""))},
		// "accompanied" is a real DepartureMode but is intentionally NOT an
		// allowed parent-request target (safety-sensitive, #1694) — the switch
		// must reject it rather than fall through.
		{"accompanied mode not requestable", carePayload(t, careDay(1, "accompanied", "", ""))},
		{"midnight arrival (reads as 'no time')", carePayload(t, careDay(1, "", "00:00", ""))},
		{"midnight pickup", carePayload(t, careDay(1, "", "", "00:00"))},
		{"malformed arrival", carePayload(t, careDay(1, "", "7:3", ""))},
		{"out-of-range clock", carePayload(t, careDay(1, "", "25:00", ""))},
		{"active day without plan", carePayload(t, map[string]any{"weekday": 1, "scheduled": true})},
		{"inactive day with pickup", carePayload(t, map[string]any{"weekday": 1, "scheduled": false, "pickup": "15:00"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := carerequests.ParseWeekly(tc.payload)
			if !errors.Is(err, carerequests.ErrInvalidPayload) {
				t.Errorf("carerequests.ParseWeekly(%s) err = %v, want carerequests.ErrInvalidPayload", tc.name, err)
			}
		})
	}
}

func TestCanonicalizeCareSchedulePayload_DedupsAndOrders(t *testing.T) {
	t.Parallel()

	canon, err := carerequests.CanonicalizeWeekly(carePayload(t,
		careDay(5, "", "", "16:00"),    // Friday first (out of order)
		careDay(1, "bus", "07:30", ""), // Monday
		careDay(1, "", "", "15:00"),    // Monday again (merge pickup in)
	))
	if err != nil {
		t.Fatalf("canonicalize = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(canon, &payload); err != nil {
		t.Fatal(err)
	}
	weekdays, ok := payload["weekdays"].([]any)
	if !ok {
		t.Fatalf("weekdays type = %T, want []any", payload["weekdays"])
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

	_, err := carerequests.CanonicalizeWeekly(carePayload(t))
	if !errors.Is(err, carerequests.ErrInvalidPayload) {
		t.Errorf("canonicalize(no weekdays) err = %v, want carerequests.ErrInvalidPayload", err)
	}
}
