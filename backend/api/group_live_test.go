package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
)

func TestGroupLiveRejectsMissingSourcesAtComposition(t *testing.T) {
	t.Parallel()

	query, err := newGroupLive(groupLiveSources{})

	assert.ErrorIs(t, err, errGroupLiveSources, "missing wiring fails composition, not the first request")
	assert.Nil(t, query)
}

// TestGroupLiveDecideDayFollowsTheSharedPrecedence pins the projection to the same
// day-planning rules the student search and the timetable use: actual
// presence turns an absent decision into unplanned attendance, but a
// timeless exception and a plan with a time keep their own reasons.
func TestGroupLiveDecideDayFollowsTheSharedPrecedence(t *testing.T) {
	t.Parallel()

	decide := groupLivePlanning{}.DecideDay
	const exception = `"Arrival":{"IsException":true,"Notes":"Zahnarzt"}`

	cases := []struct {
		name   string
		inputs string
		want   string
	}{
		{"absent with exception", `{` + exception + `}`,
			`{"Reason":"arrival_exception","ExceptionNotes":"Zahnarzt"}`},
		{"present despite exception", `{"Present":true,` + exception + `}`,
			`{"ComesToday":true,"Reason":"unplanned_attendance"}`},
		{"checked out keeps the exception", `{` + exception + `}`,
			`{"Reason":"arrival_exception","ExceptionNotes":"Zahnarzt"}`},
		{"pickup schedule", `{"Present":true,"Pickup":{"PickupTime":"2026-08-21T15:00:00Z"}}`,
			`{"ComesToday":true,"Reason":"pickup_schedule"}`},
		{"sick but present", `{"Present":true,"Sick":true,"HasTimetable":true}`,
			`{"ComesToday":true,"Reason":"unplanned_attendance"}`},
		{"timetable", `{"HasTimetable":true}`,
			`{"ComesToday":true,"Reason":"timetable"}`},
		{"no plan", `{}`, `{"Reason":"no_plan"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inputs := decodeAs(t, argumentOf(decide), tc.inputs)
			assert.Equal(t, decodeAs(t, resultOf(decide), tc.want), decide(inputs))
		})
	}
}

func TestGroupLiveCalendarRendersBerlinWallClock(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, time.August, 21, 6, 5, 0, 0, time.UTC)
	c := groupLiveCalendar{now: func() time.Time { return fixed }}

	assert.Equal(t, "2026-08-21", string(c.Today()))
	assert.Equal(t, "08:05", c.Clock(fixed), "instants render in Berlin summer time")
}

// groupLiveSettingsDouble is a func-field double for the projection's settings port.
// A nil field answers the zero value.
type groupLiveSettingsDouble struct {
	ResolveBoolFn   func(ctx context.Context, key string) (bool, error)
	ResolveStringFn func(ctx context.Context, key string) (string, error)
	PrefetchFn      func(ctx context.Context, keys []string) (context.Context, error)
}

func (s *groupLiveSettingsDouble) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.ResolveBoolFn != nil {
		return s.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

func (s *groupLiveSettingsDouble) ResolveString(ctx context.Context, key string) (string, error) {
	if s.ResolveStringFn != nil {
		return s.ResolveStringFn(ctx, key)
	}
	return "", nil
}

func (s *groupLiveSettingsDouble) Prefetch(ctx context.Context, keys []string) (context.Context, error) {
	if s.PrefetchFn != nil {
		return s.PrefetchFn(ctx, keys)
	}
	return ctx, nil
}

func TestGroupLiveSettingsPreparePrefetchesTheProjectionKeys(t *testing.T) {
	t.Parallel()

	type prefetched struct{}
	ctx := context.Background()
	var requested []string
	prepared, err := groupLiveSettings{settings: &groupLiveSettingsDouble{PrefetchFn: func(ctx context.Context, keys []string) (context.Context, error) {
		requested = keys
		return context.WithValue(ctx, prefetched{}, true), nil
	}}}.Prepare(ctx)
	require.NoError(t, err)
	assert.Equal(t, true, prepared.Value(prefetched{}), "the request continues with the prefetched context")
	assert.Contains(t, requested, settingsplatform.KeySessionEndTime)
	assert.Contains(t, requested, settingsplatform.KeyPresenceMode)

	_, err = groupLiveSettings{settings: &groupLiveSettingsDouble{PrefetchFn: func(ctx context.Context, _ []string) (context.Context, error) {
		return ctx, errors.New("settings down")
	}}}.Prepare(ctx)
	require.EqualError(t, err, "settings down")
}
