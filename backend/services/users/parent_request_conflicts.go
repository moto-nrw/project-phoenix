package users

import (
	"fmt"
	"sort"
	"strings"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Conflict keys (#2267, stories 6-10). Two open requests for the same child
// contradict each other when they would write the same thing: the same
// weekday of the weekly plan, the same day's absence, the same Stammdaten
// field, the same offering. Deciding them one after the other silently lets
// the later decision overwrite the earlier one, which is how a family ends up
// with a result nobody chose.
//
// A key names WHAT would be written, never WHAT WITH — two requests asking
// for opposite things must land in the same group, otherwise there is nothing
// to resolve. So the absence key deliberately omits the status, and the care
// key omits the times.
//
// This is a pure function on purpose: the grouping is the part that has to be
// testable without a database, and it is shared by the list and the resolve
// endpoint so the two can never disagree about what conflicts.

// ParentRequestConflictInput is the minimum the key derivation needs. One
// request can produce several keys (a weekly plan touching three weekdays).
type ParentRequestConflictInput struct {
	// RequestType is the wire type: master_data, care_schedule,
	// pickup_change, offering or excused.
	RequestType string
	// Weekdays are the ISO weekdays (1-7) a care-schedule request changes.
	Weekdays []int
	// CareKind separates the care aspects that can be requested independently
	// for one weekday (for example the booking day itself and its pickup
	// time), so two requests touching different aspects do not collide.
	CareKind string
	// Dates are the calendar days an absence or pickup-change request covers,
	// as YYYY-MM-DD.
	Dates []string
	// Target and Field name the Stammdaten value a master-data request writes.
	Target, Field string
	// OfferingID is the offering an offering-change request switches. Ranges
	// are open-ended, so any two requests for one offering overlap.
	OfferingID int64
}

// ParentRequestConflictKeys returns the keys one request occupies, sorted and
// deduplicated. An input the derivation cannot read (an unknown type, a
// request with no scope) returns no keys, which means it conflicts with
// nothing — never guessing is the safe direction here: a missed group costs a
// grouping, a wrong group would reject a request the staff never looked at.
func ParentRequestConflictKeys(input ParentRequestConflictInput) []string {
	var keys []string
	switch input.RequestType {
	case userModels.ParentRequestTypeCareSchedule:
		kind := strings.TrimSpace(input.CareKind)
		if kind == "" {
			kind = "plan"
		}
		for _, weekday := range input.Weekdays {
			if weekday >= 1 && weekday <= 7 {
				keys = append(keys, fmt.Sprintf("care:%d:%s", weekday, kind))
			}
		}
	case userModels.ParentRequestTypeExcusedAbsence:
		for _, date := range input.Dates {
			if date != "" {
				keys = append(keys, "absence:"+date)
			}
		}
	case userModels.ParentRequestTypePickupChange:
		for _, date := range input.Dates {
			if date != "" {
				keys = append(keys, "pickup:"+date)
			}
		}
	case userModels.ParentRequestTypeMasterData:
		if input.Target != "" && input.Field != "" {
			keys = append(keys, "md:"+input.Target+":"+input.Field)
		}
	case userModels.ParentRequestTypeOffering:
		if input.OfferingID > 0 {
			keys = append(keys, fmt.Sprintf("offer:%d", input.OfferingID))
		}
	}
	if len(keys) < 2 {
		return keys
	}
	sort.Strings(keys)
	return slicesCompact(keys)
}

// slicesCompact drops adjacent duplicates from a sorted slice. Written out
// rather than pulled from slices so the returned slice keeps its own backing
// array — callers put these straight on the wire.
func slicesCompact(sorted []string) []string {
	out := sorted[:1]
	for _, value := range sorted[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
