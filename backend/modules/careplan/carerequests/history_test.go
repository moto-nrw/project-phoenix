package carerequests

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecisionSnapshotPreservesStoredJSON(t *testing.T) {
	t.Parallel()
	const stored = `{"diff":[{"label":"Montag · Abholart","old":"Fährt Bus","new":"Wird abgeholt","weekday":1,"care_kind":"departure_mode","old_modes":["bus"],"new_mode":"pickup"}]}`
	var snapshot DecisionSnapshot
	require.NoError(t, json.Unmarshal([]byte(stored), &snapshot))
	entries := snapshot.Entries()
	require.Equal(t, []DiffEntry{{Label: "Montag · Abholart", Old: "Fährt Bus", New: "Wird abgeholt", Weekday: 1, CareKind: KindDepartureMode, OldModes: []string{"bus"}, NewMode: "pickup"}}, entries)
	encoded, err := json.Marshal(FreezeDecision(entries))
	require.NoError(t, err)
	require.JSONEq(t, stored, string(encoded))
	entries[0].OldModes[0] = "alone"
	require.Equal(t, "bus", snapshot.Entries()[0].OldModes[0])
}

func TestDecisionSnapshotDoesNotFollowLaterEdits(t *testing.T) {
	t.Parallel()
	diff := []DiffEntry{{OldModes: []string{"bus"}, NewMode: "pickup"}}
	snapshot := FreezeDecision(diff)
	diff[0].OldModes[0] = "alone"
	diff[0].NewMode = "bus"
	require.Equal(t, []string{"bus"}, snapshot.Entries()[0].OldModes)
	require.Equal(t, "pickup", snapshot.Entries()[0].NewMode)
	var absent *DecisionSnapshot
	require.Nil(t, absent.Entries())
	require.NotNil(t, FreezeDecision(nil).Entries(), "an empty snapshot differs from an absent snapshot")
}

func TestRequestedSummaryKeepsReasonlessPickupHistory(t *testing.T) {
	t.Parallel()
	require.Equal(t, []DiffEntry{{Label: "20.09.2026 · Abholzeit", New: "16:00", CareKind: KindPickup}}, RequestedSummary(json.RawMessage(`{"date":"2026-09-20","pickup_time":"16:00","reason":""}`)))
	require.Nil(t, RequestedSummary(json.RawMessage(`{"weekdays":"invalid"}`)))
}

func TestStoredPickupTermsDropsMalformedPreviousTime(t *testing.T) {
	t.Parallel()
	payload := map[string]any{"date": "2026-09-20", "pickup_time": "16:00", "previous_pickup_time": "yesterday"}
	terms := StoredPickupTerms(historyPayload(t, payload))
	require.NotNil(t, terms)
	require.Equal(t, "16:00", terms.PickupTime)
	require.Empty(t, terms.PreviousPickupTime)
	payload["previous_pickup_time"] = "15:00"
	require.Equal(t, "15:00", StoredPickupTerms(historyPayload(t, payload)).PreviousPickupTime)
	payload["pickup_time"] = "invalid"
	require.Nil(t, StoredPickupTerms(historyPayload(t, payload)))
}

func historyPayload(t *testing.T, payload map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return raw
}
