package students

import (
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
)

func dailyDepartureModes(student StudentResponse, date timezone.Date) []users.DepartureMode {
	if !student.HasFullAccess {
		return nil
	}
	day, ok := departureDayKey(date.Weekday())
	if !ok {
		return nil
	}
	allowed := student.AllowedDepartureModes.Normalize()
	if modes := allowed[day]; len(modes) > 0 {
		return modes
	}
	if allowed.HasAny() {
		return []users.DepartureMode{users.DepartureAlone}
	}
	if student.DepartureRuleConfigured {
		return []users.DepartureMode{legacyDepartureMode(student.PickupStatus)}
	}
	return nil
}

func legacyDepartureMode(status string) users.DepartureMode {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(normalized, "abgeholt"), normalized == "pickup", normalized == "picked_up":
		return users.DeparturePickup
	case strings.Contains(normalized, "bus"):
		return users.DepartureBus
	case strings.Contains(normalized, "anderem kind"), normalized == "accompanied":
		return users.DepartureAccompanied
	default:
		return users.DepartureAlone
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

func dailyDepartureMatchesFilter(modes []users.DepartureMode, filter string) bool {
	switch filter {
	case "self":
		return slices.Contains(modes, users.DepartureAlone)
	case "pickedUp":
		return slices.Contains(modes, users.DeparturePickup)
	case "none":
		return len(modes) == 0
	case "other":
		return len(modes) > 0 &&
			!slices.Contains(modes, users.DepartureAlone) &&
			!slices.Contains(modes, users.DeparturePickup)
	default:
		return false
	}
}
