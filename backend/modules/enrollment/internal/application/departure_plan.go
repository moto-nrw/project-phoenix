package application

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

// enrollmentRebaseUntouchedDeparturePlan preserves the legacy field
// precedence while rebasing untouched hydrated fields onto the state read
// under the student row lock. Only intentional edits win.
func enrollmentRebaseUntouchedDeparturePlan(student *Student, current *Student) {
	baseline := student.DepartureBaseline
	if baseline == nil || current == nil {
		return
	}
	if student.AllowedDepartureModes != nil &&
		enrollmentAllowedDepartureModesEqual(student.AllowedDepartureModes, baseline.AllowedDepartureModes) {
		student.AllowedDepartureModes = current.AllowedDepartureModes
	}
	if student.DepartureDays != nil &&
		enrollmentDepartureDaysEqual(student.DepartureDays, baseline.DepartureDays) {
		student.DepartureDays = current.DepartureDays
	}
	if student.BusDays != nil && enrollmentBusDaysEqual(student.BusDays, baseline.BusDays) {
		student.BusDays = current.BusDays
	}
	if student.PickupDays != nil && enrollmentPickupDaysEqual(student.PickupDays, baseline.PickupDays) {
		student.PickupDays = current.PickupDays
	}
	// PickupStatus needs no rebase: hydration always leaves PickupDays non-nil,
	// and enrollmentResolvedPickupDays only falls back to the legacy status
	// string when PickupDays is nil.
}

func enrollmentResolveAllowedDepartureModes(student *Student, current *Student) departure.AllowedDepartureModes {
	if student.AllowedDepartureModes != nil && enrollmentShouldUseAllowedDepartureModes(student, current) {
		return student.AllowedDepartureModes.Normalize()
	}
	if current == nil {
		if student.DepartureDays != nil {
			return departure.AllowedDepartureModesFromDeparture(student.DepartureDays).Normalize()
		}
		return departure.AllowedDepartureModesFromLegacy(student.BusDays, enrollmentResolvedPickupDays(student)).Normalize()
	}

	pickup := enrollmentResolvedPickupDays(student)
	busChanged := student.BusDays != nil && !enrollmentBusDaysEqual(student.BusDays, current.BusDays)
	pickupChanged := pickup != nil && !enrollmentPickupDaysEqual(pickup, current.PickupDays)
	if busChanged || pickupChanged {
		return enrollmentMergeLegacyDepartureModes(current.AllowedDepartureModes, student.BusDays, pickup, busChanged, pickupChanged)
	}

	if student.DepartureDays != nil && !enrollmentDepartureDaysEqual(student.DepartureDays, current.DepartureDays) {
		return departure.AllowedDepartureModesFromDeparture(student.DepartureDays).Normalize()
	}
	return current.AllowedDepartureModes.Normalize()
}

func enrollmentShouldUseAllowedDepartureModes(student *Student, current *Student) bool {
	if current == nil {
		return true
	}
	if !enrollmentAllowedDepartureModesEqual(student.AllowedDepartureModes, current.AllowedDepartureModes) {
		return true
	}
	pickup := enrollmentResolvedPickupDays(student)
	departureChanged := student.DepartureDays != nil &&
		!enrollmentDepartureDaysEqual(student.DepartureDays, current.DepartureDays)
	legacyChanged := (student.BusDays != nil && !enrollmentBusDaysEqual(student.BusDays, current.BusDays)) ||
		(pickup != nil && !enrollmentPickupDaysEqual(pickup, current.PickupDays))
	return !departureChanged && !legacyChanged
}

func enrollmentResolvedPickupDays(student *Student) departure.PickupDays {
	if student.PickupDays != nil {
		return student.PickupDays
	}
	if student.PickupStatus != nil {
		return departure.PickupDaysFromLegacyStatus(*student.PickupStatus)
	}
	return nil
}

func enrollmentMergeLegacyDepartureModes(current departure.AllowedDepartureModes, bus departure.BusDays, pickup departure.PickupDays, busChanged, pickupChanged bool) departure.AllowedDepartureModes {
	current = current.Normalize()
	out := departure.AllowedDepartureModes{}
	for _, day := range departure.PickupDayOrder {
		modes := map[departure.DepartureMode]bool{}
		for _, mode := range current[day] {
			modes[mode] = true
		}
		if busChanged {
			modes[departure.DepartureBus] = bus[day]
		}
		if pickupChanged {
			modes[departure.DeparturePickup] = pickup[day]
		}
		for _, mode := range []departure.DepartureMode{departure.DepartureAlone, departure.DepartureBus, departure.DeparturePickup, departure.DepartureAccompanied} {
			if modes[mode] {
				out[day] = append(out[day], mode)
			}
		}
	}
	return out.Normalize()
}

func enrollmentAllowedDepartureModesEqual(a, b departure.AllowedDepartureModes) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range departure.PickupDayOrder {
		am := a[day]
		bm := b[day]
		if len(am) != len(bm) {
			return false
		}
		for i := range am {
			if am[i] != bm[i] {
				return false
			}
		}
	}
	return true
}

func enrollmentDepartureDaysEqual(a, b departure.DepartureDays) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range departure.PickupDayOrder {
		if a.ModeFor(day) != b.ModeFor(day) {
			return false
		}
	}
	return true
}

func enrollmentBusDaysEqual(a, b departure.BusDays) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range departure.PickupDayOrder {
		if a[day] != b[day] {
			return false
		}
	}
	return true
}

func enrollmentPickupDaysEqual(a, b departure.PickupDays) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range departure.PickupDayOrder {
		if a[day] != b[day] {
			return false
		}
	}
	return true
}
