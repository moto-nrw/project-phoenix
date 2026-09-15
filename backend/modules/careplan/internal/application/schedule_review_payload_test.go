package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
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
			payload, err := json.Marshal(reviewPickupPayload{Date: "2026-03-30", PickupTime: "14:00", Reason: reason})
			require.NoError(t, err)
			service := &ScheduleReviews{}
			_, err = service.pickupDiff(context.Background(), &careplan.CareScheduleChangeRequest{Payload: payload}, ports.ReviewStudent{}, &careplan.CareScheduleReviewItem{})
			require.Error(t, err)
			require.Len(t, requestedCareSummary(payload), 1)
		})
	}
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
