package carerequests

import (
	"encoding/json"
)

// baselineKey is the stored payload key of the plan the family saw (#6990).
// Canonicalizing a client payload drops it, so only the server writes it.
const baselineKey = "baseline"

// baselineValue is one requested field as the family saw it.
type baselineValue struct {
	Weekday  int    `json:"weekday"`
	CareKind string `json:"care_kind"`
	Old      string `json:"old"`
}

type baselineField struct {
	weekday  int
	careKind string
}

// WithBaseline stores, per requested field, the value the family saw when it
// submitted or edited the request: the old side of the diff its own plan read
// renders (#6990). The weekly ask in raw stays unchanged.
func WithBaseline(raw json.RawMessage, seen []DiffEntry) (json.RawMessage, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrInvalidPayload
	}
	values := make([]baselineValue, 0, len(seen))
	for _, entry := range seen {
		if entry.Weekday < 1 || entry.CareKind == "" {
			continue
		}
		values = append(values, baselineValue{Weekday: entry.Weekday, CareKind: entry.CareKind, Old: entry.Old})
	}
	payload[baselineKey] = values
	return json.Marshal(payload)
}

// MarkSubmittedBaseline returns a copy of the live diff in which every field
// whose value changed since the family submitted carries that earlier value in
// SubmittedOld. Requests stored before #6990 carry no baseline and stay
// unmarked; a field the family saw unchanged carries no hint.
func MarkSubmittedBaseline(live []DiffEntry, raw json.RawMessage) []DiffEntry {
	seen := storedBaseline(raw)
	out := append([]DiffEntry(nil), live...)
	if len(seen) == 0 {
		return out
	}
	for i := range out {
		old, found := seen[baselineField{weekday: out[i].Weekday, careKind: out[i].CareKind}]
		if found && old != out[i].Old {
			out[i].SubmittedOld = old
		}
	}
	return out
}

func storedBaseline(raw json.RawMessage) map[baselineField]string {
	var payload struct {
		Baseline []baselineValue `json:"baseline"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	out := make(map[baselineField]string, len(payload.Baseline))
	for _, value := range payload.Baseline {
		out[baselineField{weekday: value.Weekday, careKind: value.CareKind}] = value.Old
	}
	return out
}
