package care

import "encoding/json"

type DepartureModesInspection struct {
	Normalized  json.RawMessage
	Accompanied bool
}

// DeparturePolicy is the workflow's canonical owner-policy boundary. The
// workflow still decides whether an accompanied change is permitted.
type DeparturePolicy interface {
	Inspect(json.RawMessage, bool) (DepartureModesInspection, error)
	DayOrder() []string
}
