package carerequests

import (
	"encoding/json"
	"testing"
)

func TestParseWallClock(t *testing.T) {
	t.Parallel()

	tm, err := parseWallClock("07:30")
	if err != nil {
		t.Fatalf("parseWallClock(07:30) = %v", err)
	}
	if tm.Hour() != 7 || tm.Minute() != 30 {
		t.Errorf("parsed = %02d:%02d, want 07:30", tm.Hour(), tm.Minute())
	}
	// Midnight normalizes to the zero time.Time, which the schedule validators
	// treat as "no time set" — the reason build/canonicalize reject "00:00".
	if mid, err := parseWallClock("00:00"); err != nil || !mid.IsZero() {
		t.Errorf("parseWallClock(00:00) = (%v, %v), want (zero, nil)", mid, err)
	}
	if _, err := parseWallClock("nope"); err == nil {
		t.Error("parseWallClock(nope) err = nil, want parse error")
	}
}

func TestDecodePayloadRoundTrip(t *testing.T) {
	t.Parallel()

	in := WeeklyChange{Weekdays: []WeekdayChange{{Weekday: 2, Mode: "bus", Arrival: "08:00"}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal = %v", err)
	}
	out, err := decodePayload[WeeklyChange](raw)
	if err != nil {
		t.Fatalf("decodePayload = %v", err)
	}
	if len(out.Weekdays) != 1 || out.Weekdays[0].Weekday != 2 || out.Weekdays[0].Mode != "bus" || out.Weekdays[0].Arrival != "08:00" {
		t.Errorf("round-trip = %+v, want weekday 2 / bus / 08:00", out.Weekdays)
	}
	// omitempty drops the untouched pickup field on the way through JSON.
	if out.Weekdays[0].Pickup != "" {
		t.Errorf("pickup = %q, want empty", out.Weekdays[0].Pickup)
	}
	if _, err := decodePayload[WeeklyChange](json.RawMessage(`{"weekdays":`)); err == nil {
		t.Error("decodePayload(truncated) err = nil, want decode error")
	}
}
