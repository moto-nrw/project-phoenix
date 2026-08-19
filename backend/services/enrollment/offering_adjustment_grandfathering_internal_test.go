package enrollment

import (
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// The classification that feeds the Bestandsschutz exemption (#2186). The
// mixed case is the review blocker: a booking holding manual AND automatic
// days must land in BOTH buckets, or unticking it deletes the automatic days
// its still-selected trigger keeps deriving.
func TestGrandfatheredOfferingsFromLinks(t *testing.T) {
	link := func(id int64, selected, manual, automatic []string) *enrollmentModels.RequestChildOffering {
		return &enrollmentModels.RequestChildOffering{
			CareOfferingID:        id,
			SelectedDays:          selected,
			ManualSelectedDays:    manual,
			AutomaticSelectedDays: automatic,
		}
	}

	tests := []struct {
		name          string
		link          *enrollmentModels.RequestChildOffering
		wantManual    bool
		wantAutomatic bool
	}{
		{
			name:       "manual only",
			link:       link(10, []string{"mon"}, []string{"mon"}, nil),
			wantManual: true,
		},
		{
			name:          "automatic only",
			link:          link(20, []string{"mon"}, nil, []string{"mon"}),
			wantAutomatic: true,
		},
		{
			name:          "both halves",
			link:          link(30, []string{"mon", "tue"}, []string{"tue"}, []string{"mon"}),
			wantManual:    true,
			wantAutomatic: true,
		},
		{
			name:       "legacy link without a breakdown",
			link:       link(40, []string{"mon"}, nil, nil),
			wantManual: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := grandfatheredOfferingsFromLinks([]*enrollmentModels.RequestChildOffering{tc.link})
			require.Equal(t, tc.wantManual, got.Manual[tc.link.CareOfferingID])
			require.Equal(t, tc.wantAutomatic, got.Automatic[tc.link.CareOfferingID])
		})
	}

	t.Run("nil links are skipped", func(t *testing.T) {
		got := grandfatheredOfferingsFromLinks([]*enrollmentModels.RequestChildOffering{nil})
		require.Empty(t, got.Manual)
		require.Empty(t, got.Automatic)
	})
}
