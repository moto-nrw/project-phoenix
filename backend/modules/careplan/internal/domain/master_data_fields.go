package domain

import (
	"encoding/json"
	"reflect"
	"slices"
)

// The value rules of a parent Stammdaten request (#3354). A request carries
// the value the family saw (old) and the value it asks for (new) as raw JSON;
// a decision compares the live value against the old one before it writes the
// new one, so an approval can never silently overwrite a newer office edit.

// DepartureModeAccompanied is the "geht mit anderem Kind/Person" mode. A
// request cannot set it: who the child leaves with is a companion link staff
// maintain on the child's card, not a value a family types.
const DepartureModeAccompanied = "accompanied"

var (
	departureDayOrder  = []string{"mon", "tue", "wed", "thu", "fri"}
	departureModeOrder = []string{"alone", "bus", "pickup", DepartureModeAccompanied}
)

// DepartureModes is the per-weekday set of ways a child may leave.
type DepartureModes map[string][]string

// DecodeDepartureModes reads a stored departure plan. JSON null is the empty
// plan; an unknown weekday, an unknown mode or a mode repeated on one day
// makes the value invalid.
func DecodeDepartureModes(raw json.RawMessage) (DepartureModes, bool) {
	var modes DepartureModes
	if json.Unmarshal(raw, &modes) != nil {
		return nil, false
	}
	if modes == nil {
		modes = DepartureModes{}
	}
	for day, values := range modes {
		if !slices.Contains(departureDayOrder, day) {
			return nil, false
		}
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			if !slices.Contains(departureModeOrder, value) || seen[value] {
				return nil, false
			}
			seen[value] = true
		}
	}
	return modes, true
}

// Normalize orders the weekdays and modes canonically, drops unknown modes
// and omits days without any mode.
func (m DepartureModes) Normalize() DepartureModes {
	out := DepartureModes{}
	for _, day := range departureDayOrder {
		var modes []string
		for _, mode := range departureModeOrder {
			if slices.Contains(m[day], mode) {
				modes = append(modes, mode)
			}
		}
		if len(modes) > 0 {
			out[day] = modes
		}
	}
	return out
}

// HasMode reports whether any weekday allows the mode.
func (m DepartureModes) HasMode(mode string) bool {
	for _, day := range departureDayOrder {
		if slices.Contains(m[day], mode) {
			return true
		}
	}
	return false
}

// SameDepartureModes compares two plans weekday by weekday in their stored
// order; callers normalize both sides first.
func SameDepartureModes(a, b DepartureModes) bool {
	for _, day := range departureDayOrder {
		if !slices.Equal(a[day], b[day]) {
			return false
		}
	}
	return true
}

// JSONString encodes a string value the way a request stores it.
func JSONString(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

// SameJSON compares two stored values by meaning rather than by bytes. An
// undecodable side is never equal to anything.
func SameJSON(a, b json.RawMessage) bool {
	var left, right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

// DisplayJSON renders a stored value for a German sentence: a JSON string
// loses its quotes, anything else is shown as it is stored.
func DisplayJSON(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}
