// Package carerequests defines the native Care Plan review vocabulary.
package carerequests

import (
	"sort"
	"strings"
)

const (
	KindArrival       = "arrival"
	KindPickup        = "pickup"
	KindDepartureMode = "departure_mode"
	KindScheduled     = "scheduled"
)

type DiffEntry struct {
	Label    string
	Old      string
	New      string
	Weekday  int
	CareKind string
	OldModes []string
	NewMode  string
}

type WeekdayChange struct {
	Weekday   int    `json:"weekday"`
	Scheduled *bool  `json:"scheduled,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Arrival   string `json:"arrival,omitempty"`
	Pickup    string `json:"pickup,omitempty"`
}

type WeeklyChange struct {
	Weekdays []WeekdayChange `json:"weekdays"`
}

// WeeklyPlan is the live owner-resolved baseline. ArrivalDays preserves a
// scheduled day whose class has no dismissal time yet.
type WeeklyPlan struct {
	ArrivalDays    map[int]bool
	ArrivalTimes   map[int]string
	PickupTimes    map[int]string
	DepartureModes map[string][]string
}

var weekdayNames = [...]string{"", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag"}
var weekdayKeys = [...]string{"", "mon", "tue", "wed", "thu", "fri"}

// WeeklyDiff renders exactly the requested fields, including equal values.
// Neither the request order nor the live baseline is mutated.
func WeeklyDiff(changes []WeekdayChange, current WeeklyPlan) []DiffEntry {
	return weeklyEntries(changes, &current)
}

// WeeklySummary renders only the stored ask. Decided requests must not compare
// themselves with a live baseline that has changed since the decision.
func WeeklySummary(changes []WeekdayChange) []DiffEntry {
	return weeklyEntries(changes, nil)
}

func weeklyEntries(changes []WeekdayChange, current *WeeklyPlan) []DiffEntry {
	weekdays := append([]WeekdayChange(nil), changes...)
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i].Weekday < weekdays[j].Weekday })
	var entries []DiffEntry
	for _, day := range weekdays {
		if day.Weekday < 1 || day.Weekday > 5 {
			continue
		}
		name := weekdayNames[day.Weekday]
		if day.Scheduled != nil {
			entry := DiffEntry{Label: name + " · Betreuungstag", New: careDayLabel(*day.Scheduled), Weekday: day.Weekday, CareKind: KindScheduled}
			if current != nil {
				entry.Old = "Keine Angaben"
				if current.ArrivalDays[day.Weekday] || current.PickupTimes[day.Weekday] != "" {
					entry.Old = careDayLabel(true)
				} else if len(current.ArrivalDays) > 0 || len(current.PickupTimes) > 0 {
					entry.Old = careDayLabel(false)
				}
			}
			entries = append(entries, entry)
		}
		if day.Mode != "" {
			entry := DiffEntry{Label: name + " · Abholart", New: departureLabel(day.Mode), Weekday: day.Weekday, CareKind: KindDepartureMode, NewMode: day.Mode}
			if current != nil {
				modes := current.DepartureModes[weekdayKeys[day.Weekday]]
				if len(modes) == 0 {
					modes = []string{"alone"}
				}
				entry.OldModes = append([]string(nil), modes...)
				labels := make([]string, 0, len(modes))
				for _, mode := range modes {
					labels = append(labels, departureLabel(mode))
				}
				entry.Old = strings.Join(labels, " / ")
			}
			entries = append(entries, entry)
		}
		if day.Arrival != "" {
			entry := DiffEntry{Label: name + " · Bringzeit", New: day.Arrival, Weekday: day.Weekday, CareKind: KindArrival}
			if current != nil {
				entry.Old = dashIfEmpty(current.ArrivalTimes[day.Weekday])
			}
			entries = append(entries, entry)
		}
		if day.Pickup != "" {
			entry := DiffEntry{Label: name + " · Abholzeit", New: day.Pickup, Weekday: day.Weekday, CareKind: KindPickup}
			if current != nil {
				entry.Old = dashIfEmpty(current.PickupTimes[day.Weekday])
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

func careDayLabel(scheduled bool) string {
	if scheduled {
		return "In der OGS"
	}
	return "Nicht in der OGS"
}

func departureLabel(mode string) string {
	switch mode {
	case "bus":
		return "Fährt Bus"
	case "pickup":
		return "Wird abgeholt"
	case "accompanied":
		return "Geht mit anderem Kind/Person"
	default:
		return "Geht alleine"
	}
}

func dashIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
