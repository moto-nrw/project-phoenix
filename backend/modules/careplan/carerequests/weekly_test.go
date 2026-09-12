package carerequests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeparturePresentationPreservesModeSets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		modes []string
		label string
		keys  []string
	}{
		{"empty means alone", nil, "Geht alleine", []string{"alone"}},
		{"bus", []string{"bus"}, "Fährt Bus", []string{"bus"}},
		{"multiple modes preserve order", []string{"bus", "pickup"}, "Fährt Bus / Wird abgeholt", []string{"bus", "pickup"}},
		{"accompanied never means alone", []string{"accompanied"}, "Geht mit anderem Kind/Person", []string{"accompanied"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := WeeklyDiff([]WeekdayChange{{Weekday: 1, Mode: "pickup"}}, WeeklyPlan{DepartureModes: map[string][]string{"mon": tc.modes}})
			require.Len(t, entries, 1)
			require.Equal(t, tc.label, entries[0].Old)
			require.Equal(t, tc.keys, entries[0].OldModes)
			require.Equal(t, "pickup", entries[0].NewMode)
			require.Equal(t, "Wird abgeholt", entries[0].New)
		})
	}
}

func TestWeeklyDiffKeepsBlankTimePlaceholder(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]string{"": "—", "   ": "—", "07:30": "07:30", " Fährt Bus ": " Fährt Bus "} {
		entries := WeeklyDiff([]WeekdayChange{{Weekday: 1, Arrival: "12:00"}}, WeeklyPlan{ArrivalTimes: map[int]string{1: value}})
		require.Equal(t, want, entries[0].Old)
	}
}

func TestWeeklyDiffAndHistorySeparateLiveFactsFromStoredAsk(t *testing.T) {
	t.Parallel()
	changes := []WeekdayChange{{Weekday: 5, Scheduled: new(false)}, {Weekday: 1, Scheduled: new(true), Mode: "alone", Arrival: "12:00", Pickup: "16:00"}, {Weekday: 6, Mode: "bus"}}
	plan := WeeklyPlan{ArrivalDays: map[int]bool{1: true}, PickupTimes: map[int]string{1: "16:00"}, DepartureModes: map[string][]string{"mon": {"pickup", "bus"}}}
	diff := WeeklyDiff(changes, plan)
	require.Len(t, diff, 5)
	require.Equal(t, []string{KindScheduled, KindDepartureMode, KindArrival, KindPickup, KindScheduled}, []string{diff[0].CareKind, diff[1].CareKind, diff[2].CareKind, diff[3].CareKind, diff[4].CareKind})
	require.Equal(t, "In der OGS", diff[0].Old, "a care day need not have an arrival time")
	require.Equal(t, "Nicht in der OGS", diff[4].Old)
	require.Equal(t, "16:00", diff[3].Old, "equal requested fields still render")
	require.Equal(t, 5, changes[0].Weekday, "sorting must not mutate the stored ask")
	diff[1].OldModes[0] = "alone"
	require.Equal(t, []string{"pickup", "bus"}, plan.DepartureModes["mon"])
	for _, entry := range WeeklySummary(changes) {
		require.Empty(t, entry.Old)
		require.Nil(t, entry.OldModes)
	}
	unknown := WeeklyDiff([]WeekdayChange{{Weekday: 1, Scheduled: new(true)}}, WeeklyPlan{})
	require.Equal(t, "Keine Angaben", unknown[0].Old)
}
