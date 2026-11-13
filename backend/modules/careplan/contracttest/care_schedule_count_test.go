package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

func TestCareScheduleCountRetainsVisibleRows(t *testing.T) {
	t.Parallel()
	f := newCareFixture(t)
	f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))
	query := nativeCareReviews(t, f)
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		visible int
	}{
		{"staff", f.staffCtx(f.staffAccount), 1},
		{"guardian", f.nonAdminCtx(f.chain.AccountID), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			full, cursor, err := query.ListPending(tc.ctx, careplan.RequestQueueFilter{Limit: 1})
			require.NoError(t, err)
			require.Len(t, full, tc.visible)
			counted, countCursor, err := query.ListPending(tc.ctx, careplan.RequestQueueFilter{Limit: 1, OmitDiff: true})
			require.NoError(t, err)
			require.Equal(t, cursor, countCursor)
			for _, item := range full {
				require.NotEmpty(t, item.Diff)
				// The impact token is that of the diff (#6843).
				item.Diff, item.ImpactToken = nil, ""
			}
			require.Equal(t, full, counted, "counting must retain the same requests, identity and visibility")
		})
	}
}
