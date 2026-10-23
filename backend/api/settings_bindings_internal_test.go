package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

type roomHistorySettingsDouble struct {
	roomHistorySettingsReader
	failKey string
}

func (s roomHistorySettingsDouble) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == s.failKey {
		return false, errors.New("settings unavailable")
	}
	return true, nil
}

func (s roomHistorySettingsDouble) ResolveInt(_ context.Context, key string) (int, error) {
	if key == s.failKey {
		return 0, errors.New("settings unavailable")
	}
	return 3, nil
}

func TestRoomHistorySettingsPropagateReadFailures(t *testing.T) {
	t.Parallel()
	prefetch := func(ctx context.Context, _ any, _ ...string) context.Context { return ctx }
	for _, key := range []string{"gdpr.attendance_log_enabled", "gdpr.room_detail_visible_days"} {
		t.Run(key, func(t *testing.T) {
			read := NewRoomHistorySettings(roomHistorySettingsDouble{failKey: key}, nil, prefetch)
			_, _, _, err := read(context.Background())
			require.Error(t, err)
		})
	}
	require.Panics(t, func() { NewRoomHistorySettings(nil, nil, prefetch) })
}

type writeScopeSettingsDouble struct {
	value any
	err   error
}

func (s writeScopeSettingsDouble) Resolve(_ context.Context, key string) (any, error) {
	if key != configModels.KeySchoolPortalWriteScope {
		return nil, errors.New("unexpected key " + key)
	}
	return s.value, s.err
}

// TestClassDayWriteScopeReadsTheMultiSelect pins how the school-portal writes
// read operations.school_portal_write_scope (#4799): every chosen value opens
// its own write, the empty default opens none, and a read failure or a
// malformed row is an error, never an opened write.
func TestClassDayWriteScopeReadsTheMultiSelect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                       string
		value                      any
		arrivalExceptions, reports bool
	}{
		{name: "registry default", value: []string{}},
		{name: "stored empty list", value: []any{}},
		{name: "arrival exceptions only", value: []any{configModels.SchoolPortalWriteScopeClassArrivalExceptions}, arrivalExceptions: true},
		{name: "class trips only", value: []any{configModels.SchoolPortalWriteScopeClassTrips}, reports: true},
		{name: "both", value: []any{configModels.SchoolPortalWriteScopeClassArrivalExceptions, configModels.SchoolPortalWriteScopeClassTrips}, arrivalExceptions: true, reports: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope := ClassDayWriteScope{Settings: writeScopeSettingsDouble{value: tc.value}}
			arrivalExceptions, err := scope.SchoolMayWriteClassArrivalExceptions(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.arrivalExceptions, arrivalExceptions)
			reports, err := scope.SchoolMayReportClassTrips(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.reports, reports)
		})
	}

	for name, settings := range map[string]writeScopeSettingsDouble{
		"read failure":         {err: errors.New("settings unavailable")},
		"legacy single string": {value: configModels.SchoolPortalWriteScopeClassArrivalExceptions},
		"non-string element":   {value: []any{true}},
	} {
		t.Run(name, func(t *testing.T) {
			scope := ClassDayWriteScope{Settings: settings}
			allowed, err := scope.SchoolMayWriteClassArrivalExceptions(context.Background())
			require.Error(t, err)
			require.False(t, allowed)
			allowed, err = scope.SchoolMayReportClassTrips(context.Background())
			require.Error(t, err)
			require.False(t, allowed)
		})
	}
}
