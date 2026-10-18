package departure

func WithAccompaniedDays(allowed AllowedDepartureModes, days []string) AllowedDepartureModes {
	out := make(AllowedDepartureModes, len(allowed)+len(days))
	for day, modes := range allowed {
		out[day] = append([]DepartureMode(nil), modes...)
	}
	for _, day := range days {
		if !validCompanionWeekday(day) {
			continue
		}
		already := false
		for _, mode := range out[day] {
			if mode == DepartureAccompanied {
				already = true
			}
		}
		if !already {
			out[day] = append(out[day], DepartureAccompanied)
		}
	}
	return out.Normalize()
}
func validCompanionWeekday(day string) bool {
	switch day {
	case PickupDayMonday, PickupDayTuesday, PickupDayWednesday, PickupDayThursday, PickupDayFriday:
		return true
	}
	return false
}
