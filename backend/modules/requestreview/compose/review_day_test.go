package compose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	"github.com/stretchr/testify/require"
)

func TestQueuesPreferTheResolvedUrgencyDate(t *testing.T) {
	t.Parallel()
	clock := func() careplan.Date { return "2026-09-12" }
	care := &careFake{pending: []*careplan.CareScheduleReviewItem{{Request: &careplan.CareScheduleChangeRequest{RequestKind: "pickup_change", Payload: json.RawMessage(`{"date":"2026-09-11"}`)}}}}
	offering := &offeringFake{pending: []*careplan.OfferingReviewItem{{Request: &careplan.OfferingChangeRequest{EffectiveFrom: "2026-09-11"}}}}
	excused := &excusedFake{pending: []*excusedrequests.ReviewItem{{Request: &excusedrequests.Request{Dates: []excusedrequests.Date{"2026-09-11"}}}}}
	queues := map[string]requestreview.Queue{
		"care": nativeCareQueue(care, clock), "offering": nativeOfferingQueue(offering, clock), "excused": nativeExcusedQueue(excused, clock),
	}
	for name, queue := range queues {
		rows, _, err := queue.Open(context.Background(), requestreview.QueueFilter{UrgentDate: "2026-09-11"})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.True(t, rows[0].UrgentToday, name)
		require.False(t, rows[0].Past, "the shared day must win over a clock that crossed midnight: "+name)
		for _, fallback := range []string{"", "invalid"} {
			rows, _, err = queue.Open(context.Background(), requestreview.QueueFilter{UrgentDate: fallback})
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.True(t, rows[0].Past, "an absent or invalid day falls back to the clock: "+name)
			require.Equal(t, name == "offering", rows[0].UrgentToday, "past offerings stay urgent: "+name)
		}
	}
}
