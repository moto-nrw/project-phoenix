package students

import (
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type dailyDeparture struct {
	Modes       []departure.DepartureMode
	LegacyLabel string
	Configured  bool
}

func dailyDepartureForDate(student StudentResponse, date timezone.Date) dailyDeparture {
	if !student.HasFullAccess {
		return dailyDeparture{}
	}
	day, ok := departureDayKey(date.Weekday())
	if !ok {
		return dailyDeparture{}
	}
	allowed := student.AllowedDepartureModes.Normalize()
	if modes := allowed[day]; len(modes) > 0 {
		return dailyDeparture{Modes: modes, Configured: true}
	}
	if allowed.HasAny() {
		return dailyDeparture{Modes: []departure.DepartureMode{departure.DepartureAlone}, Configured: true}
	}
	if !student.DepartureRuleConfigured {
		return dailyDeparture{}
	}
	if mode, ok := legacyDepartureMode(student.PickupStatus); ok {
		return dailyDeparture{Modes: []departure.DepartureMode{mode}, Configured: true}
	}
	return dailyDeparture{
		LegacyLabel: strings.TrimSpace(student.PickupStatus),
		Configured:  true,
	}
}

func legacyDepartureMode(status string) (departure.DepartureMode, bool) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(normalized, "alleine"),
		normalized == "selbst",
		normalized == "self",
		normalized == "alone",
		normalized == "walk_home",
		normalized == "geht_alleine":
		return departure.DepartureAlone, true
	case strings.Contains(normalized, "abgeholt"),
		normalized == "parent",
		normalized == "parents",
		normalized == "guardian",
		normalized == "pickup",
		normalized == "picked_up",
		normalized == "wird_abgeholt":
		return departure.DeparturePickup, true
	case strings.Contains(normalized, "bus"):
		return departure.DepartureBus, true
	case strings.Contains(normalized, "anderem kind"), normalized == "accompanied":
		return departure.DepartureAccompanied, true
	default:
		return "", false
	}
}

func departureDayKey(weekday time.Weekday) (string, bool) {
	switch weekday {
	case time.Monday:
		return departure.PickupDayMonday, true
	case time.Tuesday:
		return departure.PickupDayTuesday, true
	case time.Wednesday:
		return departure.PickupDayWednesday, true
	case time.Thursday:
		return departure.PickupDayThursday, true
	case time.Friday:
		return departure.PickupDayFriday, true
	default:
		return "", false
	}
}

func dailyDepartureMatchesFilter(daily dailyDeparture, filter string) bool {
	switch filter {
	case "self":
		return slices.Contains(daily.Modes, departure.DepartureAlone)
	case "pickedUp":
		return slices.Contains(daily.Modes, departure.DeparturePickup)
	case "none":
		return !daily.Configured
	case "other":
		return daily.Configured &&
			!slices.Contains(daily.Modes, departure.DepartureAlone) &&
			!slices.Contains(daily.Modes, departure.DeparturePickup)
	default:
		return false
	}
}
