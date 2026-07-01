package users

import "fmt"

// DepartureMode is how a child leaves on a given weekday. It unifies the two
// formerly independent boolean maps (BusDays + PickupDays) into a single
// mutually-exclusive choice per day, so a day can no longer be both "Buskind"
// and "wird abgeholt" at the same time (#1610).
type DepartureMode string

const (
	// DepartureAlone — the child goes home on their own ("geht alleine").
	// This is the default for any weekday not present in a DepartureDays map.
	DepartureAlone DepartureMode = "alone"
	// DepartureBus — the child rides the bus ("fährt Bus").
	DepartureBus DepartureMode = "bus"
	// DeparturePickup — the child is collected by a person ("wird abgeholt").
	DeparturePickup DepartureMode = "pickup"
	// DepartureAccompanied — the child leaves together with another child or a
	// named person ("geht mit anderem Kind/Person"). Who they leave with is
	// captured separately in the free-text companion note on the student, and
	// may be formalized into a departure group (#1694).
	DepartureAccompanied DepartureMode = "accompanied"
)

var validDepartureModes = map[DepartureMode]bool{
	DepartureAlone:       true,
	DepartureBus:         true,
	DeparturePickup:      true,
	DepartureAccompanied: true,
}

// GermanLabel renders the mode as a standalone, capitalized German label for
// chat/diff and other parent-facing surfaces ("Fährt Bus" / "Wird abgeholt" /
// "Geht alleine"). It is the single source of truth for this wording so the
// parent messaging diff and any other consumer cannot drift. (Staff CSV exports
// use their own mid-sentence lowercase phrasing in api/enrollment, which is a
// deliberately different register.)
func (m DepartureMode) GermanLabel() string {
	switch m {
	case DepartureBus:
		return "Fährt Bus"
	case DeparturePickup:
		return "Wird abgeholt"
	case DepartureAccompanied:
		// MUST be explicit: an accompanied child leaves WITH another child/person
		// and must never fall through to "Geht alleine" (safety-sensitive, #1694).
		// Letting it hit the default mislabels the child as a self-goer on the
		// staff confirm diff, so staff would approve a schedule change believing
		// the wrong current state.
		return "Geht mit anderem Kind/Person"
	default:
		return "Geht alleine"
	}
}

// DepartureDays stores, per weekday, how the child leaves. A weekday not
// present (or explicitly DepartureAlone) means the child goes home alone that
// day, so the map only ever needs to carry the bus/pickup days and stays small.
// Keys are the canonical weekday abbreviations (mon/tue/wed/thu/fri) shared with
// BusDays and PickupDays.
type DepartureDays map[string]DepartureMode

func (d DepartureDays) Validate() error {
	for day, mode := range d {
		if !validPickupDays[day] {
			return fmt.Errorf("departure_days weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
		if !validDepartureModes[mode] {
			return fmt.Errorf("departure_days mode %q must be one of alone/bus/pickup/accompanied", mode)
		}
	}
	return nil
}

// Normalize strips unknown weekday keys and drops days set to DepartureAlone
// (the default), so the stored map only carries explicit bus/pickup days.
func (d DepartureDays) Normalize() DepartureDays {
	out := DepartureDays{}
	for _, day := range PickupDayOrder {
		switch d[day] {
		case DepartureBus:
			out[day] = DepartureBus
		case DeparturePickup:
			out[day] = DeparturePickup
		case DepartureAccompanied:
			out[day] = DepartureAccompanied
		}
	}
	return out
}

// ModeFor returns the mode for a weekday, defaulting to DepartureAlone for any
// day not explicitly set to a non-alone mode.
func (d DepartureDays) ModeFor(day string) DepartureMode {
	switch mode := d[day]; mode {
	case DepartureBus, DeparturePickup, DepartureAccompanied:
		return mode
	default:
		return DepartureAlone
	}
}

// HasAny reports whether any weekday is set to a non-alone mode.
func (d DepartureDays) HasAny() bool {
	for _, day := range PickupDayOrder {
		if d.ModeFor(day) != DepartureAlone {
			return true
		}
	}
	return false
}

// HasMode reports whether any weekday is set to the given departure mode.
func (d DepartureDays) HasMode(mode DepartureMode) bool {
	for _, m := range d {
		if m == mode {
			return true
		}
	}
	return false
}

// BusDays derives the legacy per-weekday Buskind map: the days whose mode is
// DepartureBus. Kept so consumers that still read BusDays keep working while the
// codebase migrates to DepartureDays as the single source of truth.
func (d DepartureDays) BusDays() BusDays {
	out := BusDays{}
	for _, day := range PickupDayOrder {
		if d.ModeFor(day) == DepartureBus {
			out[day] = true
		}
	}
	return out
}

// PickupDays derives the legacy per-weekday pickup map: the days whose mode is
// DeparturePickup ("wird abgeholt").
func (d DepartureDays) PickupDays() PickupDays {
	out := PickupDays{}
	for _, day := range PickupDayOrder {
		if d.ModeFor(day) == DeparturePickup {
			out[day] = true
		}
	}
	return out
}

// LegacyPickupStatus derives the legacy pickup_status string from the unified
// per-day plan. Priority: any pickup day -> "Wird abgeholt"; otherwise any
// accompanied day -> "Geht mit anderem Kind"; otherwise the child goes home
// alone. An accompanied child leaves WITH a companion and must never be bucketed
// as a self-goer, which is safety-sensitive state (#1694); the dedicated
// non-self value keeps legacy search/list/admin-filter consumers from
// classifying them as "Geht alleine nach Hause". Bus days do not count as pickup
// for this legacy field, matching the historical semantics.
func (d DepartureDays) LegacyPickupStatus() string {
	if d.PickupDays().HasAny() {
		return PickupStatusPickedUp
	}
	if d.HasMode(DepartureAccompanied) {
		return PickupStatusAccompanied
	}
	return PickupStatusGoesAlone
}

// DepartureDaysFromLegacy folds the two formerly independent boolean maps into
// the unified per-day enum. Pickup wins when a legacy day is flagged as both
// Buskind and "wird abgeholt" — being collected by a person is the
// higher-stakes instruction for staff, and such contradictory rows can only
// originate from the pre-unification data model.
func DepartureDaysFromLegacy(bus BusDays, pickup PickupDays) DepartureDays {
	out := DepartureDays{}
	for _, day := range PickupDayOrder {
		switch {
		case pickup[day]:
			out[day] = DeparturePickup
		case bus[day]:
			out[day] = DepartureBus
		}
	}
	return out
}

// departureModeStakes ranks the exclusive modes from highest to lowest stakes,
// used only to merge two exclusive plans for the same weekday. Pickup (collected
// by a named person) outranks accompanied (leaves with another child/person),
// which outranks bus, which outranks going alone. Accompanied MUST beat bus so a
// merge never silently drops the safety-sensitive "Mit anderem Kind" signal in
// favor of a bus instruction (#1694).
var departureModeStakes = map[DepartureMode]int{
	DeparturePickup:      3,
	DepartureAccompanied: 2,
	DepartureBus:         1,
	DepartureAlone:       0,
}

// Merge returns the per-weekday combination of two exclusive plans, keeping the
// higher-stakes mode on any day both set (see departureModeStakes). Used when a
// single enrollment submission carries more than one field targeting the legacy
// exclusive student.departure target; merging — rather than last-field-wins —
// stops a later bus/pickup field from dropping an earlier accompanied day. The
// result is normalized.
func (d DepartureDays) Merge(other DepartureDays) DepartureDays {
	out := DepartureDays{}
	for _, day := range PickupDayOrder {
		a, b := d.ModeFor(day), other.ModeFor(day)
		if departureModeStakes[b] > departureModeStakes[a] {
			a = b
		}
		if a != DepartureAlone {
			out[day] = a
		}
	}
	return out.Normalize()
}
