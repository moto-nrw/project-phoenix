package domain

import (
	"fmt"
	"strings"
	"time"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassArrivalBaselineFromTimes reads retained weekday maps without changing
// their stored keys. Invalid clocks do not create an expected arrival.
func ClassArrivalBaselineFromTimes(class string, values map[string]string) *ClassArrivalBaseline {
	times := make(map[int]time.Time, 5)
	for day := 1; day <= 5; day++ {
		key, _ := ArrivalWeekdayKey(day)
		if clock, ok := classArrivalClock(values, key); ok {
			times[day] = clock
		}
	}
	return &ClassArrivalBaseline{SchoolClass: class, ArrivalTimes: times}
}

func classArrivalClock(values map[string]string, key string) (time.Time, bool) {
	for storedDay, hhmm := range values {
		if strings.ToLower(strings.TrimSpace(storedDay)) != key {
			continue
		}
		clock, err := time.Parse("15:04", hhmm)
		if err != nil {
			return time.Time{}, false
		}
		return calendar.NormalizeWallClock(clock), true
	}
	return time.Time{}, false
}

// NormalizeClassArrivalTimes canonicalizes the weekday map: lowercased day
// codes, trimmed zero-padded HH:MM values, empty values dropped. An empty
// result becomes nil so the jsonb column stores an empty object rather than
// half-filled noise. Mirrors normalizePickupTimes, minus the available-days
// check — a class has no availability, only a timetable.
func NormalizeClassArrivalTimes(times map[string]string) (map[string]string, error) {
	if len(times) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(times))
	for day, hhmm := range times {
		key := strings.ToLower(strings.TrimSpace(day))
		value := strings.TrimSpace(hhmm)
		if value == "" {
			continue
		}
		iso, ok := canonicalDayToISOWeekday(key)
		if !ok {
			return nil, fmt.Errorf("arrival_times key %q is not a known day abbreviation", day)
		}
		if iso > 5 {
			return nil, fmt.Errorf("arrival_times day %q must be Monday through Friday", key)
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return nil, fmt.Errorf("arrival_times value for %q must be HH:MM, got %q", key, hhmm)
		}
		out[key] = parsed.Format("15:04")
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// ArrivalWeekdayKey maps Monday..Friday onto the day codes the weekday
// maps are keyed by.
func ArrivalWeekdayKey(weekday int) (string, bool) {
	day, ok := arrivalWeekdayKeys[weekday]
	return day, ok
}

var arrivalWeekdayKeys = map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri"}
