package presenceprojectionhttp

import (
	context "context"
	slices "slices"
	strings "strings"
	time "time"

	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type dailyDeparture struct {
	Modes       []departure.DepartureMode
	LegacyLabel string
	Configured  bool
	// Exception marks a day Abholart (#3945) that replaces the weekly plan.
	Exception bool
}

func dailyDepartureForDate(student StudentResponse, date calendar.Date) dailyDeparture {
	if !student.HasFullAccess {
		return dailyDeparture{}
	}
	if student.DayDepartureMode != "" {
		return dailyDeparture{Modes: []departure.DepartureMode{departure.DepartureMode(student.DayDepartureMode)}, Configured: true, Exception: true}
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

// departurePlanDate is the day whose weekday's departure plan applies on
// date: the Friday before a weekend that follows Friday's plan (#3921), else
// date itself. A weekday never reads the setting.
func departurePlanDate(ctx context.Context, date calendar.Date) (calendar.Date, error) {
	weekday, err := calendar.PlanWeekday(ctx, date)
	if err != nil {
		return date, err
	}
	return date.AddDays(weekday - calendar.ISOWeekday(date)), nil
}

// applyDayDepartures loads the Abholart of the listed day (#3945) for every
// full-access row in one read, before the pickup-rule filter and the
// pagination run, so both see the day's value and not the weekly plan's.
func (rs *Resource) applyDayDepartures(ctx context.Context, responses []StudentResponse, date calendar.Date) error {
	if rs.PickupScheduleService == nil {
		return nil
	}
	ids := make([]int64, 0, len(responses))
	for i := range responses {
		if responses[i].HasFullAccess {
			ids = append(ids, responses[i].ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	modes, err := rs.PickupScheduleService.GetBulkDayDepartureModesForDate(ctx, ids, date)
	if err != nil {
		return err
	}
	for i := range responses {
		if responses[i].HasFullAccess {
			responses[i].DayDepartureMode = modes[responses[i].ID]
		}
	}
	return nil
}
