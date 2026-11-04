package me

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// SchoolSettings resolves the request tenant's settings inside its tenant
// transaction: the tenant override, else the registry default.
type SchoolSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// schoolSwitches are the school's settings the signed-in app shell reads
// outside the settings page (#4996): the navigation, the Elternzugänge page
// and the Betreuungsplan read them here instead of loading the whole settings
// schema. They are signed-in context only; the public tenant resolve never
// carries them.
type schoolSwitches struct {
	ParentNewsEnabled     bool   `json:"parent_news_enabled"`
	ParentInviteMode      string `json:"parent_invite_mode"`
	TimetableDayStartTime string `json:"timetable_day_start_time"`
	TimetableDayEndTime   string `json:"timetable_day_end_time"`
}

// errSchoolSwitchesUnavailable marks a navigation that failed on its school
// switches; the cause stays out of the response.
var errSchoolSwitchesUnavailable = errors.New("school switches unavailable")

var schoolSwitchKeys = []string{
	settings.KeyParentNewsEnabled,
	settings.KeyGuardianParentInviteMode,
	settings.KeyTimetableDayStartTime,
	settings.KeyTimetableDayEndTime,
}

// loadSchoolSwitches resolves every switch in one batch. An unreadable setting
// fails the request instead of rendering a guessed value.
func loadSchoolSwitches(ctx context.Context, source SchoolSettings) (schoolSwitches, error) {
	ctx, err := settings.PrefetchSettingsOrError(ctx, source, schoolSwitchKeys...)
	if err != nil {
		return schoolSwitches{}, fmt.Errorf("resolve school switches: %w", err)
	}
	var switches schoolSwitches
	if switches.ParentNewsEnabled, err = source.ResolveBool(ctx, settings.KeyParentNewsEnabled); err != nil {
		return schoolSwitches{}, fmt.Errorf("resolve %s: %w", settings.KeyParentNewsEnabled, err)
	}
	stringFields := []struct {
		key    string
		target *string
	}{
		{settings.KeyGuardianParentInviteMode, &switches.ParentInviteMode},
		{settings.KeyTimetableDayStartTime, &switches.TimetableDayStartTime},
		{settings.KeyTimetableDayEndTime, &switches.TimetableDayEndTime},
	}
	for _, field := range stringFields {
		if *field.target, err = source.ResolveString(ctx, field.key); err != nil {
			return schoolSwitches{}, fmt.Errorf("resolve %s: %w", field.key, err)
		}
	}
	return switches, nil
}

// withSchoolSwitches adds the switches to the navigation row under
// "school_switches"; every key the row renders stays as it was.
func withSchoolSwitches(row any, switches schoolSwitches) (map[string]json.RawMessage, error) {
	encoded, err := json.Marshal(row)
	if err != nil {
		return nil, fmt.Errorf("encode navigation row: %w", err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("navigation row is not an object: %w", err)
	}
	if fields["school_switches"], err = json.Marshal(switches); err != nil {
		return nil, fmt.Errorf("encode school switches: %w", err)
	}
	return fields, nil
}
