package students

import (
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
)

type dailyDeparture struct {
	Modes       []users.DepartureMode
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
		return dailyDeparture{Modes: []users.DepartureMode{users.DepartureAlone}, Configured: true}
	}
	if !student.DepartureRuleConfigured {
		return dailyDeparture{}
	}
	if mode, ok := legacyDepartureMode(student.PickupStatus); ok {
		return dailyDeparture{Modes: []users.DepartureMode{mode}, Configured: true}
	}
	return dailyDeparture{
		LegacyLabel: strings.TrimSpace(student.PickupStatus),
		Configured:  true,
	}
}

func legacyDepartureMode(status string) (users.DepartureMode, bool) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(normalized, "alleine"),
		normalized == "selbst",
		normalized == "self",
		normalized == "alone",
		normalized == "walk_home",
		normalized == "geht_alleine":
		return users.DepartureAlone, true
	case strings.Contains(normalized, "abgeholt"),
		normalized == "parent",
		normalized == "parents",
		normalized == "guardian",
		normalized == "pickup",
		normalized == "picked_up",
		normalized == "wird_abgeholt":
		return users.DeparturePickup, true
	case strings.Contains(normalized, "bus"):
		return users.DepartureBus, true
	case strings.Contains(normalized, "anderem kind"), normalized == "accompanied":
		return users.DepartureAccompanied, true
	default:
		return "", false
	}
}

func departureDayKey(weekday time.Weekday) (string, bool) {
	switch weekday {
	case time.Monday:
		return users.PickupDayMonday, true
	case time.Tuesday:
		return users.PickupDayTuesday, true
	case time.Wednesday:
		return users.PickupDayWednesday, true
	case time.Thursday:
		return users.PickupDayThursday, true
	case time.Friday:
		return users.PickupDayFriday, true
	default:
		return "", false
	}
}

func dailyDepartureMatchesFilter(departure dailyDeparture, filter string) bool {
	switch filter {
	case "self":
		return slices.Contains(departure.Modes, users.DepartureAlone)
	case "pickedUp":
		return slices.Contains(departure.Modes, users.DeparturePickup)
	case "none":
		return !departure.Configured
	case "other":
		return departure.Configured &&
			!slices.Contains(departure.Modes, users.DepartureAlone) &&
			!slices.Contains(departure.Modes, users.DeparturePickup)
	default:
		return false
	}
}
