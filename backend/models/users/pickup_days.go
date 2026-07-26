package users

import "fmt"

const (
	PickupDayMonday    = "mon"
	PickupDayTuesday   = "tue"
	PickupDayWednesday = "wed"
	PickupDayThursday  = "thu"
	PickupDayFriday    = "fri"
)

var PickupDayOrder = []string{
	PickupDayMonday,
	PickupDayTuesday,
	PickupDayWednesday,
	PickupDayThursday,
	PickupDayFriday,
}

var validPickupDays = map[string]bool{
	PickupDayMonday:    true,
	PickupDayTuesday:   true,
	PickupDayWednesday: true,
	PickupDayThursday:  true,
	PickupDayFriday:    true,
}

// Legacy pickup_status string values. The per-weekday model supersedes the
// single global status, but the string is kept (derived) for backward
// compatibility with consumers that still read student.pickup_status.
const (
	PickupStatusPickedUp  = "Wird abgeholt"
	PickupStatusGoesAlone = "Geht alleine nach Hause"
	// PickupStatusAccompanied is the legacy pickup_status for a child whose only
	// non-self departure is the accompanied ("Mit anderem Kind") mode. Such a
	// child does NOT go home alone, so they must never collapse onto
	// PickupStatusGoesAlone: that would let search/list UI and admin filters
	// bucket safety-sensitive accompanied children as self-goers (#1694). It is
	// deliberately neither canonical value, so pickupStatusKind sorts it into the
	// "other"/"Sonstige" bucket rather than "self" or "pickedUp".
	PickupStatusAccompanied = "Geht mit anderem Kind"
)

// PickupDays stores the recurring weekdays on which a child is picked up
// ("wird abgeholt"). A weekday not present (or false) means the child goes
// home alone that day. Mirrors BusDays so the create/edit surfaces and the
// public enrollment form can treat both the same way.
type PickupDays map[string]bool

func (p PickupDays) Validate() error {
	for day := range p {
		if !validPickupDays[day] {
			return fmt.Errorf("pickup_days weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
	}
	return nil
}

func (p PickupDays) HasAny() bool {
	for _, day := range PickupDayOrder {
		if p[day] {
			return true
		}
	}
	return false
}

func (p PickupDays) Normalize() PickupDays {
	out := PickupDays{}
	for _, day := range PickupDayOrder {
		if p[day] {
			out[day] = true
		}
	}
	return out
}

// PickupDaysFromLegacyStatus seeds the per-weekday map from the legacy
// pickup_status string. "Wird abgeholt" implies all five weekdays; any other
// value (including "Geht alleine nach Hause" and the empty/unset string)
// implies no pickup days.
func PickupDaysFromLegacyStatus(status string) PickupDays {
	if status != PickupStatusPickedUp {
		return PickupDays{}
	}
	return PickupDays{
		PickupDayMonday:    true,
		PickupDayTuesday:   true,
		PickupDayWednesday: true,
		PickupDayThursday:  true,
		PickupDayFriday:    true,
	}
}

// LegacyPickupStatus derives the legacy pickup_status string from the
// per-weekday map: any selected day means "Wird abgeholt"; otherwise the
// child goes home alone.
func (p PickupDays) LegacyPickupStatus() string {
	if p.HasAny() {
		return PickupStatusPickedUp
	}
	return PickupStatusGoesAlone
}
