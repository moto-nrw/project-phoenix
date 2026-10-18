package peopledirectory

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

// Departure values are the public directory contract for import and enrollment.
type (
	BusDays               = departure.BusDays
	PickupDays            = departure.PickupDays
	DepartureMode         = departure.DepartureMode
	DepartureDays         = departure.DepartureDays
	AllowedDepartureModes = departure.AllowedDepartureModes
	// CompanionLink is one "läuft mit" companion of a child with the
	// weekdays they walk together.
	CompanionLink = departure.CompanionLink
)

// FilterCompanionLinksToDays keeps only the weekdays the given set allows.
func FilterCompanionLinksToDays(links []CompanionLink, allowedDays map[string]bool) []CompanionLink {
	return departure.FilterCompanionLinksToDays(links, allowedDays)
}

// FormatCompanionLinks renders the "läuft mit" links for the offline lists.
func FormatCompanionLinks(links []CompanionLink) string { return departure.FormatCompanionLinks(links) }

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

// HasCompanionNote reports whether the free-text "mit wem" note carries the
// detail. A blank note is no answer.
func HasCompanionNote(note *string) bool { return departure.HasCompanionNote(note) }

// MissingAccompaniedDays returns, in weekday order, the requested weekdays the
// child's plan does not yet allow leaving with another child on: the days a
// confirmed companion link widens the plan by.
func MissingAccompaniedDays(modes AllowedDepartureModes, days DepartureDays, requested []string) []string {
	return departure.Plan{AllowedDepartureModes: modes, DepartureDays: days}.MissingAccompaniedDays(requested)
}

const MaxDepartureCompanionNoteLen = departure.MaxDepartureCompanionNoteLen

var ErrDepartureCompanionNoteRequired = departure.ErrDepartureCompanionNoteRequired

func NormalizeCompanionNote(days DepartureDays, modes AllowedDepartureModes, note *string, covered map[string]bool) (*string, error) {
	return departure.NormalizeCompanionNote(days, modes, note, covered)
}
