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
}

func (m AllowedDepartureModes) Validate() error {
	for day, modes := range m {
		if !validPickupDays[day] {
			return fmt.Errorf("allowed_departure_modes weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
		seen := map[DepartureMode]bool{}
		for _, mode := range modes {
			if !validDepartureModes[mode] {
				return fmt.Errorf("allowed_departure_modes mode %q must be one of alone/bus/pickup", mode)
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

// DepartureDays derives the legacy exclusive plan. Pickup wins over bus and
// bus wins over alone because older consumers can only display one instruction.
func (m AllowedDepartureModes) DepartureDays() DepartureDays {
	out := DepartureDays{}
	for _, day := range PickupDayOrder {
		switch {
		case allowedModesContain(m[day], DeparturePickup):
			out[day] = DeparturePickup
		case allowedModesContain(m[day], DepartureBus):
			out[day] = DepartureBus
		}
	}
	return out
}

func AllowedDepartureModesFromDeparture(days DepartureDays) AllowedDepartureModes {
	out := AllowedDepartureModes{}
	for _, day := range PickupDayOrder {
		switch days.ModeFor(day) {
		case DepartureBus:
			out[day] = []DepartureMode{DepartureBus}
		case DeparturePickup:
			out[day] = []DepartureMode{DeparturePickup}
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
