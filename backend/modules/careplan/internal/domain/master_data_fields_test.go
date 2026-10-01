package domain

import (
	"encoding/json"
	"testing"
)

// The value rules behind the Stammdaten decision (#1803, #3354). No database —
// these pin the departure-plan decoding and comparison and the JSON equality
// the decision relies on to detect a stale baseline and refuse invalid values.

func TestDecodeDepartureModes(t *testing.T) {
	t.Parallel()

	modes, ok := DecodeDepartureModes(json.RawMessage(`{"mon":["bus","pickup"]}`))
	if !ok {
		t.Fatal("valid plan did not decode")
	}
	if len(modes["mon"]) != 2 {
		t.Errorf("mon modes = %v, want 2 entries", modes["mon"])
	}

	// JSON null decodes to a non-nil empty plan, so a "clear all" request is a
	// real, appliable value rather than a decode failure.
	modes, ok = DecodeDepartureModes(json.RawMessage(`null`))
	if !ok || modes == nil {
		t.Errorf("null decode = (%v, %v), want (non-nil empty, true)", modes, ok)
	}

	for name, raw := range map[string]string{
		"malformed json":    `not-json`,
		"invalid weekday":   `{"funday":["bus"]}`,
		"unknown mode":      `{"mon":["spaceship"]}`,
		"mode given twice":  `{"mon":["bus","bus"]}`,
		"not a plan at all": `123`,
	} {
		if _, ok := DecodeDepartureModes(json.RawMessage(raw)); ok {
			t.Errorf("%s decoded as a valid plan", name)
		}
	}
}

func TestDepartureModesNormalizeAndCompare(t *testing.T) {
	t.Parallel()

	a := DepartureModes{"mon": {"bus", "pickup"}}
	b := DepartureModes{"mon": {"pickup", "bus"}, "tue": {}}
	if !SameDepartureModes(a.Normalize(), b.Normalize()) {
		t.Error("the same plan in a different order compared unequal after normalizing")
	}
	if SameDepartureModes(a, DepartureModes{"mon": {"bus"}}) {
		t.Error("differing lengths compared equal")
	}
	if SameDepartureModes(a, DepartureModes{"mon": {"bus", "alone"}}) {
		t.Error("differing content compared equal")
	}
	// Empty plans are equal (both mean "no restrictions recorded").
	if !SameDepartureModes(DepartureModes{}, DepartureModes{}) {
		t.Error("two empty plans compared unequal")
	}
	if _, kept := b.Normalize()["tue"]; kept {
		t.Error("a weekday without any mode must be omitted")
	}
	if !(DepartureModes{"wed": {DepartureModeAccompanied}}).HasMode(DepartureModeAccompanied) {
		t.Error("HasMode missed the accompanied weekday")
	}
}

func TestSameJSON(t *testing.T) {
	t.Parallel()

	// Semantic equality: whitespace and key order do not matter.
	if !SameJSON(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"b":2, "a":1}`)) {
		t.Error("semantically equal JSON compared unequal")
	}
	if SameJSON(json.RawMessage(`"07:30"`), json.RawMessage(`"08:00"`)) {
		t.Error("different strings compared equal")
	}
	// A malformed side is treated as not-equal (never panics).
	if SameJSON(json.RawMessage(`{bad`), json.RawMessage(`{}`)) {
		t.Error("malformed JSON compared equal")
	}
	if got := DisplayJSON(JSONString("Moritz")); got != "Moritz" {
		t.Errorf("DisplayJSON(string) = %q, want the bare text", got)
	}
	if got := DisplayJSON(json.RawMessage(`{"mon":["bus"]}`)); got != `{"mon":["bus"]}` {
		t.Errorf("DisplayJSON(object) = %q, want the stored JSON", got)
	}
}
