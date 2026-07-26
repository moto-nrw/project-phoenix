package users

import "fmt"

// AllowedDepartureModes stores, per weekday, every way a child is allowed to
// leave. Unlike DepartureDays this is intentionally non-exclusive: parents may
// allow both "bus" and "pickup" on the same day.
type AllowedDepartureModes map[string][]DepartureMode

var departureModeOrder = []DepartureMode{
	DepartureAlone,
	DepartureBus,
	DeparturePickup,
	DepartureAccompanied,
}

func (m AllowedDepartureModes) Validate() error {
	for day, modes := range m {
		if !validPickupDays[day] {
			return fmt.Errorf("allowed_departure_modes weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
		seen := map[DepartureMode]bool{}
		for _, mode := range modes {
			if !validDepartureModes[mode] {
				return fmt.Errorf("allowed_departure_modes mode %q must be one of alone/bus/pickup/accompanied", mode)
			}
			if seen[mode] {
				return fmt.Errorf("allowed_departure_modes day %q contains duplicate mode %q", day, mode)
			}
			seen[mode] = true
		}
	}
	return nil
}

func (m AllowedDepartureModes) Normalize() AllowedDepartureModes {
	out := AllowedDepartureModes{}
	for _, day := range PickupDayOrder {
		seen := map[DepartureMode]bool{}
		for _, raw := range m[day] {
			if validDepartureModes[raw] {
				seen[raw] = true
			}
		}
		if len(seen) == 0 {
			continue
		}
		modes := make([]DepartureMode, 0, len(seen))
		for _, mode := range departureModeOrder {
			if seen[mode] {
				modes = append(modes, mode)
			}
		}
		out[day] = modes
	}
	return out
}

func (m AllowedDepartureModes) HasAny() bool {
	for _, day := range PickupDayOrder {
		if len(m[day]) > 0 {
			return true
		}
	}
	return false
}

// HasMode reports whether any weekday allows the given departure mode.
func (m AllowedDepartureModes) HasMode(mode DepartureMode) bool {
	for _, day := range PickupDayOrder {
		if allowedModesContain(m[day], mode) {
			return true
		}
	}
	return false
}

func (m AllowedDepartureModes) BusDays() BusDays {
	out := BusDays{}
	for _, day := range PickupDayOrder {
		if allowedModesContain(m[day], DepartureBus) {
			out[day] = true
		}
	}
	return out
}

func (m AllowedDepartureModes) PickupDays() PickupDays {
	out := PickupDays{}
	for _, day := range PickupDayOrder {
		if allowedModesContain(m[day], DeparturePickup) {
			out[day] = true
		}
	}
	return out
}

// DepartureDays derives the legacy exclusive plan. Priority pickup > bus >
// accompanied > alone, because older consumers can only display one
// instruction and being collected by a person is the highest-stakes one.
func (m AllowedDepartureModes) DepartureDays() DepartureDays {
	out := DepartureDays{}
	for _, day := range PickupDayOrder {
		switch {
		case allowedModesContain(m[day], DeparturePickup):
			out[day] = DeparturePickup
		case allowedModesContain(m[day], DepartureBus):
			out[day] = DepartureBus
		case allowedModesContain(m[day], DepartureAccompanied):
			out[day] = DepartureAccompanied
		}
	}
	return out
}

// LegacyPickupStatus derives the legacy student-level pickup_status string from
// the FULL non-exclusive set of allowed modes — never from the exclusive
// DepartureDays() projection. Priority: any pickup day -> "Wird abgeholt";
// otherwise any accompanied day -> "Geht mit anderem Kind"; otherwise the child
// goes home alone. Deriving from the projection would drop the accompanied
// signal on a day that ALSO allows bus (the projection ranks bus above
// accompanied), silently bucketing a safety-sensitive accompanied child as a
// self-goer for legacy search/list/admin-filter consumers (#1694). Bus days do
// not count as pickup, matching the historical semantics of this field.
func (m AllowedDepartureModes) LegacyPickupStatus() string {
	if m.HasMode(DeparturePickup) {
		return PickupStatusPickedUp
	}
	if m.HasMode(DepartureAccompanied) {
		return PickupStatusAccompanied
	}
	return PickupStatusGoesAlone
}

// Merge returns the union of two non-exclusive plans: a weekday allows a mode if
// EITHER input allows it. Used when a single enrollment submission carries more
// than one field targeting student.allowed_departure_modes (#1694); union is
// lossless here precisely because the model is non-exclusive, so a later
// bus/pickup-only field can never silently drop an accompanied mode an earlier
// field allowed. The result is normalized (deduped, canonical mode order).
func (m AllowedDepartureModes) Merge(other AllowedDepartureModes) AllowedDepartureModes {
	out := AllowedDepartureModes{}
	for _, day := range PickupDayOrder {
		combined := make([]DepartureMode, 0, len(m[day])+len(other[day]))
		combined = append(combined, m[day]...)
		combined = append(combined, other[day]...)
		if len(combined) > 0 {
			out[day] = combined
		}
	}
	return out.Normalize()
}

func AllowedDepartureModesFromDeparture(days DepartureDays) AllowedDepartureModes {
	out := AllowedDepartureModes{}
	for _, day := range PickupDayOrder {
		switch days.ModeFor(day) {
		case DepartureBus:
			out[day] = []DepartureMode{DepartureBus}
		case DeparturePickup:
			out[day] = []DepartureMode{DeparturePickup}
		case DepartureAccompanied:
			out[day] = []DepartureMode{DepartureAccompanied}
		}
	}
	return out
}

func AllowedDepartureModesFromLegacy(bus BusDays, pickup PickupDays) AllowedDepartureModes {
	out := AllowedDepartureModes{}
	for _, day := range PickupDayOrder {
		modes := make([]DepartureMode, 0, 2)
		if bus[day] {
			modes = append(modes, DepartureBus)
		}
		if pickup[day] {
			modes = append(modes, DeparturePickup)
		}
		if len(modes) > 0 {
			out[day] = modes
		}
	}
	return out
}

func allowedModesContain(modes []DepartureMode, want DepartureMode) bool {
	for _, mode := range modes {
		if mode == want {
			return true
		}
	}
	return false
}
