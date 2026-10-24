package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

func TestCareReviewRequestedSummary(t *testing.T) {
	t.Parallel()
	got := requestedCareSummary(json.RawMessage(`{"date":"2026-03-30","pickup_time":"14:30","reason":""}`))
	require.Len(t, got, 1)
	require.Equal(t, "30.03.2026 · Abholzeit", got[0].Label)
	require.Equal(t, "14:30", got[0].New)
	require.Empty(t, got[0].Old)
	require.Equal(t, "pickup", got[0].CareKind)
	require.Nil(t, requestedCareSummary(json.RawMessage(`{`)))
}

func TestCareReviewPendingRequiresReasonButSummaryDoesNot(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"", "   ", strings.Repeat("ü", 256)} {
		t.Run(reason, func(t *testing.T) {
			payload, err := json.Marshal(map[string]string{"date": "2026-03-30", "pickup_time": "14:00", "reason": reason})
			require.NoError(t, err)
			service := &ScheduleReviews{}
			_, err = service.pickupDiff(context.Background(), &careplan.CareScheduleChangeRequest{Payload: payload}, careplan.ReviewStudent{}, &careplan.CareScheduleReviewItem{})
			require.Error(t, err)
			require.Len(t, requestedCareSummary(payload), 1)
		})
	}
}

// #3945: a one-day request may carry only the Abholart. The summary names
// it with the mode label, and the live review compares it with the modes the
// day had at submission.
func TestCareReviewDayDepartureMode(t *testing.T) {
	t.Parallel()
	summary := requestedCareSummary(json.RawMessage(`{"date":"2026-03-30","departure_mode":"bus","reason":"Ausflug"}`))
	require.Len(t, summary, 1)
	require.Equal(t, "30.03.2026 · Abholart", summary[0].Label)
	require.Equal(t, "Fährt Bus", summary[0].New)
	require.Equal(t, "bus", summary[0].NewMode)
	require.Equal(t, "departure_mode", summary[0].CareKind)

	item := &careplan.CareScheduleReviewItem{}
	diff, err := (&ScheduleReviews{}).pickupDiff(context.Background(), &careplan.CareScheduleChangeRequest{
		Payload: json.RawMessage(`{"date":"2026-03-30","departure_mode":"pickup","previous_departure_modes":["alone","bus"],"reason":"Oma holt ab"}`),
	}, careplan.ReviewStudent{}, item)
	require.NoError(t, err)
	require.Len(t, diff, 1)
	require.Equal(t, "Geht alleine / Fährt Bus", diff[0].Old)
	require.Equal(t, []string{"alone", "bus"}, diff[0].OldModes)
	require.Equal(t, "Wird abgeholt", diff[0].New)
	require.False(t, item.ImpactAvailable, "an Abholart-only request touches no block")

	both := requestedCareSummary(json.RawMessage(`{"date":"2026-03-30","pickup_time":"13:00","departure_mode":"pickup","reason":"Arzt"}`))
	require.Len(t, both, 2)
	require.Equal(t, "30.03.2026 · Abholzeit", both[0].Label)
	require.Equal(t, "30.03.2026 · Abholart", both[1].Label)

	require.Nil(t, requestedCareSummary(json.RawMessage(`{"date":"2026-03-30","departure_mode":"accompanied","reason":"x"}`)),
		"guardians cannot ask for in Begleitung")
}

func TestCareReviewSnapshotPreservesDecisionFacts(t *testing.T) {
	t.Parallel()
	got := careReviewSnapshot(json.RawMessage(`{"diff":[{"label":"Montag · Abholart","old":"Alleine","new":"Begleitet","weekday":1,"care_kind":"departure_mode","old_modes":["alone"],"new_mode":"accompanied"}]}`))
	require.Len(t, got, 1)
	require.Equal(t, "Alleine", got[0].Old)
	require.Equal(t, []string{"alone"}, got[0].OldModes)
	require.Equal(t, "accompanied", got[0].NewMode)
	require.Equal(t, 1, got[0].Weekday)
	require.Nil(t, careReviewSnapshot(nil))
	require.Nil(t, careReviewSnapshot(json.RawMessage(`{`)))
}
