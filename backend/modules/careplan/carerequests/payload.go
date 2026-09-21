package carerequests

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var ErrInvalidPayload = errors.New("schedule: invalid care request payload")

// WeeklyChanges is the validated, bucketed form of a care-schedule
// payload: per-weekday departure modes (keyed by abbreviation), arrival times
// and pickup times (keyed by weekday number, both "HH:MM"). Produced by
// ParseWeekly and consumed by both the create-time validation and
// the approve-time apply.
type WeeklyChanges struct {
	Scheduled map[int]bool
	Modes     map[string]string
	Arrivals  map[int]string
	Pickups   map[int]string
}

// RequestedFieldGroups reports which independently configurable field
// groups a validated permanent-care request contains.
type RequestedFieldGroups struct {
	Scheduled     bool
	Arrival       bool
	Pickup        bool
	DepartureMode bool
}

// RequestedFields validates a request payload through the same
// parser used by create/apply and exposes only its field groups to the parent
// policy layer.
func RequestedFields(payload json.RawMessage) (RequestedFieldGroups, error) {
	changes, err := ParseWeekly(payload)
	if err != nil {
		return RequestedFieldGroups{}, err
	}
	if changes.IsEmpty() {
		return RequestedFieldGroups{}, fmt.Errorf("%w: no changes", ErrInvalidPayload)
	}
	return RequestedFieldGroups{
		Scheduled:     len(changes.Scheduled) > 0,
		Arrival:       len(changes.Arrivals) > 0,
		Pickup:        len(changes.Pickups) > 0,
		DepartureMode: len(changes.Modes) > 0,
	}, nil
}

func (c WeeklyChanges) IsEmpty() bool {
	return len(c.Scheduled) == 0 && len(c.Modes) == 0 && len(c.Arrivals) == 0 && len(c.Pickups) == 0
}

// CanonicalPayload rebuilds the persistable payload from the bucketed
// changes: exactly one entry per weekday (1=Mon..5=Fri) that has a change, in
// fixed weekday order, carrying only the merged mode/arrival/pickup. This is
// what collapses a direct API client's duplicate or out-of-order weekday
// entries into the same form the apply path already derives.
func (c WeeklyChanges) CanonicalPayload() WeeklyChange {
	weekdays := make([]WeekdayChange, 0, len(weekdayKeys[1:]))
	for i, abbrev := range weekdayKeys[1:] {
		wd := i + 1
		entry := WeekdayChange{Weekday: wd}
		present := false
		if scheduled, ok := c.Scheduled[wd]; ok {
			entry.Scheduled = &scheduled
			present = true
		}
		if mode, ok := c.Modes[abbrev]; ok {
			entry.Mode = string(mode)
			present = true
		}
		if arrival, ok := c.Arrivals[wd]; ok {
			entry.Arrival = arrival
			present = true
		}
		if pickup, ok := c.Pickups[wd]; ok {
			entry.Pickup = pickup
			present = true
		}
		if present {
			weekdays = append(weekdays, entry)
		}
	}
	return WeeklyChange{Weekdays: weekdays}
}

func decodePayload[T any](raw json.RawMessage) (T, error) {
	var out T
	err := json.Unmarshal(raw, &out)
	return out, err
}

// parseWallClock turns an "HH:MM" string into the normalized wall-clock
// time.Time the schedule TIME columns store. Routing through calendar.NormalizeWallClock
// is the mandated normalization for TIME columns (CLAUDE.md rule 11). Preserved
// rows carried through a merge must be re-normalized the same way before
// re-insert, since TIME columns scan back with a driver-chosen year that
// Postgres rejects.
func parseWallClock(hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return calendar.NormalizeWallClock(t), nil
}

// ParseWeekly validates a care-schedule payload (weekday range,
// departure-mode enum, HH:MM format) and buckets it into the per-aspect change
// maps. It is the SINGLE parser shared by the create-time validation and the
// approve-time apply, so the two paths cannot drift. Validation failures wrap
// ErrInvalidPayload; the apply path treats any non-nil error as a
// 500.
func ParseWeekly(payload json.RawMessage) (WeeklyChanges, error) {
	p, err := decodePayload[WeeklyChange](payload)
	if err != nil {
		return WeeklyChanges{}, ErrInvalidPayload
	}
	out := WeeklyChanges{Scheduled: map[int]bool{}, Modes: map[string]string{}, Arrivals: map[int]string{}, Pickups: map[int]string{}}
	for _, day := range p.Weekdays {
		if err := out.mergeWeekday(day); err != nil {
			return WeeklyChanges{}, err
		}
	}
	if err := out.validateScheduledDays(); err != nil {
		return WeeklyChanges{}, err
	}
	return out, nil
}

func (c WeeklyChanges) mergeWeekday(day WeekdayChange) error {
	if day.Weekday < 1 || day.Weekday > 5 {
		return fmt.Errorf("%w: weekday %d", ErrInvalidPayload, day.Weekday)
	}
	if day.Scheduled != nil {
		c.Scheduled[day.Weekday] = *day.Scheduled
	}
	if day.Mode != "" {
		switch day.Mode {
		case "alone", "bus", "pickup":
			c.Modes[weekdayKeys[day.Weekday]] = day.Mode
		default:
			return fmt.Errorf("%w: mode %q", ErrInvalidPayload, day.Mode)
		}
	}
	if err := mergeClockChange(c.Arrivals, day.Weekday, day.Arrival, "arrival"); err != nil {
		return err
	}
	return mergeClockChange(c.Pickups, day.Weekday, day.Pickup, "pickup")
}

func mergeClockChange(changes map[int]string, weekday int, value, kind string) error {
	if value == "" {
		return nil
	}
	// Midnight is the schedule's unset sentinel. Reject it at both creation
	// and approval so a stored request cannot later fail as an unset time.
	clock, err := parseWallClock(value)
	if err != nil || clock.IsZero() {
		return fmt.Errorf("%w: %s %q", ErrInvalidPayload, kind, value)
	}
	changes[weekday] = value
	return nil
}

func (c WeeklyChanges) validateScheduledDays() error {
	for weekday, active := range c.Scheduled {
		_, hasMode := c.Modes[weekdayKeys[weekday]]
		_, hasArrival := c.Arrivals[weekday]
		_, hasPickup := c.Pickups[weekday]
		if active && (!hasPickup || !hasMode) {
			return fmt.Errorf("%w: scheduled weekday %d needs pickup and mode", ErrInvalidPayload, weekday)
		}
		if !active && (hasPickup || hasArrival || hasMode) {
			return fmt.Errorf("%w: inactive weekday %d contains plan values", ErrInvalidPayload, weekday)
		}
	}
	return nil
}

// CanonicalizeWeekly validates and returns the sanitized payload
// to persist. Buckets via the shared parser (which rejects bad weekdays/modes/
// times), then rebuilds one entry per changed weekday so unknown keys and
// duplicate weekday rows from a direct API client never reach storage.
func CanonicalizeWeekly(payload json.RawMessage) (json.RawMessage, error) {
	changes, err := ParseWeekly(payload)
	if err != nil {
		return nil, err
	}
	if changes.IsEmpty() {
		return nil, fmt.Errorf("%w: no changes", ErrInvalidPayload)
	}
	return json.Marshal(changes.CanonicalPayload())
}
