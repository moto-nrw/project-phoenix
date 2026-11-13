package carerequests

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaselineMarksOnlyFieldsChangedSinceSubmission(t *testing.T) {
	t.Parallel()
	ask := []WeekdayChange{{Weekday: 3, Pickup: "16:00", Mode: "pickup"}}
	submitted := WeeklyDiff(ask, WeeklyPlan{PickupTimes: map[int]string{3: "15:00"}, DepartureModes: map[string][]string{"wed": {"bus"}}})
	raw, err := WithBaseline(json.RawMessage(`{"weekdays":[{"weekday":3,"pickup":"16:00","mode":"pickup"}]}`), submitted)
	require.NoError(t, err)
	require.Equal(t, ask, mustWeeklyChange(t, raw).Weekdays, "the stored ask stays unchanged")

	// Staff moved the pickup to 16:30 after the family submitted; the Abholart is untouched.
	live := WeeklyDiff(ask, WeeklyPlan{PickupTimes: map[int]string{3: "16:30"}, DepartureModes: map[string][]string{"wed": {"bus"}}})
	marked := MarkSubmittedBaseline(live, raw)
	require.Len(t, marked, 2)
	require.Equal(t, KindDepartureMode, marked[0].CareKind)
	require.Empty(t, marked[0].SubmittedOld, "an unchanged field carries no hint")
	require.Equal(t, KindPickup, marked[1].CareKind)
	require.Equal(t, "16:30", marked[1].Old)
	require.Equal(t, "15:00", marked[1].SubmittedOld)
	require.Empty(t, live[1].SubmittedOld, "marking must not mutate the live diff")
}

func TestBaselineAbsentForOlderRequestsAndClientInput(t *testing.T) {
	t.Parallel()
	live := WeeklyDiff([]WeekdayChange{{Weekday: 1, Pickup: "16:00"}}, WeeklyPlan{PickupTimes: map[int]string{1: "14:00"}})
	require.Empty(t, MarkSubmittedBaseline(live, json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`))[0].SubmittedOld)
	require.Empty(t, MarkSubmittedBaseline(live, json.RawMessage(`not json`))[0].SubmittedOld)

	// A client cannot plant a baseline: canonicalizing drops unknown keys.
	canonical, err := CanonicalizeWeekly(json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}],"baseline":[{"weekday":1,"care_kind":"pickup","old":"09:00"}]}`))
	require.NoError(t, err)
	require.Empty(t, MarkSubmittedBaseline(live, canonical)[0].SubmittedOld)
}

func TestBaselineSurvivesTheDecisionSnapshot(t *testing.T) {
	t.Parallel()
	entries := []DiffEntry{{Label: "Mittwoch · Abholzeit", Old: "16:30", New: "16:00", Weekday: 3, CareKind: KindPickup, SubmittedOld: "15:00"}}
	require.Equal(t, entries, FreezeDecision(entries).Entries())
}

func TestWeeklyImpactCoversTheSubmittedHint(t *testing.T) {
	t.Parallel()
	plain := []DiffEntry{{Label: "Mittwoch · Abholzeit", Old: "16:30", New: "16:00"}}
	hinted := []DiffEntry{{Label: "Mittwoch · Abholzeit", Old: "16:30", New: "16:00", SubmittedOld: "15:00"}}
	require.NotEqual(t, WeeklyImpactContent(plain), WeeklyImpactContent(hinted))
}

func mustWeeklyChange(t *testing.T, raw json.RawMessage) WeeklyChange {
	t.Helper()
	var change WeeklyChange
	require.NoError(t, json.Unmarshal(raw, &change))
	return change
}
