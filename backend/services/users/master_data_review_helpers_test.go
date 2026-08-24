package users

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Pure helpers behind the master-data change-request apply/compare path (#1803).
// No database — these pin the field extraction, JSON equality, and
// departure-mode comparison the Decide path relies on to detect no-op changes
// and reject invalid targets/values.

func TestPersonFieldRaw(t *testing.T) {
	t.Parallel()

	bday := timezone.NewDate(2018, 5, 4)
	person := &userModels.Person{FirstName: "Max", LastName: "Mustermann", Birthday: &bday}

	got, err := personFieldRaw(person, "first_name")
	if err != nil || string(got) != `"Max"` {
		t.Errorf("first_name = (%s, %v), want (\"Max\", nil)", got, err)
	}
	got, err = personFieldRaw(person, "last_name")
	if err != nil || string(got) != `"Mustermann"` {
		t.Errorf("last_name = (%s, %v)", got, err)
	}
	got, err = personFieldRaw(person, "birthday")
	if err != nil || string(got) != `"2018-05-04"` {
		t.Errorf("birthday = (%s, %v), want ISO date string", got, err)
	}

	// A nil birthday must serialize as JSON null, not the zero date.
	got, err = personFieldRaw(&userModels.Person{}, "birthday")
	if err != nil || string(got) != "null" {
		t.Errorf("nil birthday = (%s, %v), want (null, nil)", got, err)
	}

	// An unsupported field is a hard "not applicable", not a silent empty value.
	if _, err := personFieldRaw(person, "shoe_size"); !errors.Is(err, ErrReviewInvalidTarget) {
		t.Errorf("unknown field err = %v, want ErrReviewInvalidTarget", err)
	}
}

func TestDecodeDepartureModes(t *testing.T) {
	t.Parallel()

	modes, err := decodeDepartureModes(json.RawMessage(`{"mon":["bus","pickup"]}`))
	if err != nil {
		t.Fatalf("valid decode = %v", err)
	}
	if len(modes["mon"]) != 2 {
		t.Errorf("mon modes = %v, want 2 entries", modes["mon"])
	}

	// JSON null decodes to a non-nil empty map (so a "clear all" request is a
	// real, appliable value rather than a decode failure).
	modes, err = decodeDepartureModes(json.RawMessage(`null`))
	if err != nil || modes == nil {
		t.Errorf("null decode = (%v, %v), want (non-nil empty, nil)", modes, err)
	}

	if _, err := decodeDepartureModes(json.RawMessage(`not-json`)); !errors.Is(err, ErrReviewInvalidValue) {
		t.Errorf("malformed json err = %v, want ErrReviewInvalidValue", err)
	}
	// A syntactically valid but semantically invalid payload (bad weekday) must
	// also fail as an invalid value, not slip through.
	if _, err := decodeDepartureModes(json.RawMessage(`{"funday":["bus"]}`)); !errors.Is(err, ErrReviewInvalidValue) {
		t.Errorf("invalid weekday err = %v, want ErrReviewInvalidValue", err)
	}
}

func TestDepartureModesEqual(t *testing.T) {
	t.Parallel()

	a := userModels.AllowedDepartureModes{"mon": {userModels.DepartureBus, userModels.DeparturePickup}}
	b := userModels.AllowedDepartureModes{"mon": {userModels.DepartureBus, userModels.DeparturePickup}}
	if !departureModesEqual(a, b) {
		t.Error("identical mode sets compared unequal")
	}

	// Different length on a day.
	if departureModesEqual(a, userModels.AllowedDepartureModes{"mon": {userModels.DepartureBus}}) {
		t.Error("differing lengths compared equal")
	}
	// Same length, different content.
	if departureModesEqual(a, userModels.AllowedDepartureModes{"mon": {userModels.DepartureBus, userModels.DepartureAlone}}) {
		t.Error("differing content compared equal")
	}
	// Empty maps are equal (both mean "no restrictions recorded").
	if !departureModesEqual(userModels.AllowedDepartureModes{}, userModels.AllowedDepartureModes{}) {
		t.Error("two empty sets compared unequal")
	}
}

func TestJSONRawEqual(t *testing.T) {
	t.Parallel()

	// Semantic equality: whitespace and key order do not matter.
	if !jsonRawEqual(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"b":2, "a":1}`)) {
		t.Error("semantically equal JSON compared unequal")
	}
	if jsonRawEqual(json.RawMessage(`"07:30"`), json.RawMessage(`"08:00"`)) {
		t.Error("different strings compared equal")
	}
	// A malformed side is treated as not-equal (never panics).
	if jsonRawEqual(json.RawMessage(`{bad`), json.RawMessage(`{}`)) {
		t.Error("malformed JSON compared equal")
	}
}
