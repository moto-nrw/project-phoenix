package peopledirectory

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

// Departure values are the public directory contract for import and enrollment.
type (
	BusDays               = departure.BusDays
	PickupDays            = departure.PickupDays
	DepartureMode         = departure.DepartureMode
	DepartureDays         = departure.DepartureDays
	AllowedDepartureModes = departure.AllowedDepartureModes
)

const (
	BusDayMonday            = departure.BusDayMonday
	BusDayTuesday           = departure.BusDayTuesday
	BusDayWednesday         = departure.BusDayWednesday
	BusDayThursday          = departure.BusDayThursday
	BusDayFriday            = departure.BusDayFriday
	PickupDayMonday         = departure.PickupDayMonday
	PickupDayTuesday        = departure.PickupDayTuesday
	PickupDayWednesday      = departure.PickupDayWednesday
	PickupDayThursday       = departure.PickupDayThursday
	PickupDayFriday         = departure.PickupDayFriday
	PickupStatusPickedUp    = departure.PickupStatusPickedUp
	PickupStatusGoesAlone   = departure.PickupStatusGoesAlone
	PickupStatusAccompanied = departure.PickupStatusAccompanied
	DepartureAlone          = departure.DepartureAlone
	DepartureBus            = departure.DepartureBus
	DeparturePickup         = departure.DeparturePickup
	DepartureAccompanied    = departure.DepartureAccompanied
)

var BusDayOrder = departure.BusDayOrder
var PickupDayOrder = departure.PickupDayOrder

func BusDaysFromLegacyFlag(enabled bool) BusDays { return departure.BusDaysFromLegacyFlag(enabled) }
func PickupDaysFromLegacyStatus(status string) PickupDays {
	return departure.PickupDaysFromLegacyStatus(status)
}
func DepartureDaysFromLegacy(bus BusDays, pickup PickupDays) DepartureDays {
	return departure.DepartureDaysFromLegacy(bus, pickup)
}
func AllowedDepartureModesFromDeparture(days DepartureDays) AllowedDepartureModes {
	return departure.AllowedDepartureModesFromDeparture(days)
}

const MaxDepartureCompanionNoteLen = departure.MaxDepartureCompanionNoteLen

var ErrDepartureCompanionNoteRequired = departure.ErrDepartureCompanionNoteRequired

func NormalizeCompanionNote(days DepartureDays, modes AllowedDepartureModes, note *string, covered map[string]bool) (*string, error) {
	return departure.NormalizeCompanionNote(days, modes, note, covered)
}
