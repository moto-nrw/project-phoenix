package timetable

import "fmt"

// Substitute time conflicts (WP-B12) are soft warnings of the staffing
// saves. A substitute carries no implicit guarantee that their day is
// otherwise free; if they are already scheduled on another instance that
// time-overlaps with one of the targeted instances, the save surfaces a
// "substitute_time_conflict" warning — informational only, never blocking.
//
// Distinct from the start-time conflicts because the input and semantics
// differ: here a set of target instances is compared against the
// substitute's OTHER same-day assignments, not against live sessions.

// SubstituteConflictKind is the stable JSON value for the one warning kind
// DetectSubstituteTimeConflicts produces.
const SubstituteConflictKind = "substitute_time_conflict"

// SubstituteTimeConflict is one soft warning in a staffing save response.
type SubstituteTimeConflict struct {
	Kind       string `json:"kind"`              // always "substitute_time_conflict"
	InstanceID int64  `json:"instance_id"`       // the substituted target that collides
	OtherID    int64  `json:"other_instance_id"` // the conflicting foreign instance
	Message    string `json:"message"`           // German-facing copy
}

// SubstituteConflictInstance is the minimal shape of a target or foreign
// instance, so the detection stays free of storage.
type SubstituteConflictInstance struct {
	ID        int64
	StartMin  int    // minutes-since-midnight, Berlin-local
	EndMin    int    // exclusive end
	StartHHMM string // "14:30" — carried through verbatim into the message
}

// DetectSubstituteTimeConflicts returns one warning per (target, foreign) pair
// whose time windows overlap (half-open: target.start < foreign.end AND
// foreign.start < target.end). Same-minute edges (target.end == foreign.start)
// do not collide.
//
// Targets: the instances being substituted.
// Foreigns: the substitute's OTHER same-day non-absent assignments (caller
// must pre-filter out the targets themselves and any absent rows).
//
// Iteration walks targets in the order given, then foreigns within each
// target — callers rely on that ordering.
func DetectSubstituteTimeConflicts(
	targets []SubstituteConflictInstance,
	foreigns []SubstituteConflictInstance,
) []SubstituteTimeConflict {
	if len(targets) == 0 || len(foreigns) == 0 {
		return nil
	}

	var warnings []SubstituteTimeConflict
	for _, tgt := range targets {
		for _, other := range foreigns {
			if tgt.ID == other.ID {
				continue
			}
			if tgt.StartMin < other.EndMin && other.StartMin < tgt.EndMin {
				warnings = append(warnings, SubstituteTimeConflict{
					Kind:       SubstituteConflictKind,
					InstanceID: tgt.ID,
					OtherID:    other.ID,
					Message: fmt.Sprintf(
						"Vertretung ist auf Instanz %d um %s eingetragen",
						other.ID, other.StartHHMM,
					),
				})
			}
		}
	}
	return warnings
}

// MinutesOfTime returns minutes-since-midnight for a clock value. Callers
// convert an instance's TIME columns (a zero-year time after decoding) into
// the StartMin/EndMin fields; the date component is intentionally ignored.
func MinutesOfTime(hour, minute int) int {
	return hour*60 + minute
}
