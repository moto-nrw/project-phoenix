package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

type retentionScans struct {
	ports.UnregisteredTagScanStore
	cutoff time.Time
}

func (s *retentionScans) DeleteExpired(_ context.Context, cutoff time.Time) (int64, domain.OperationStats, error) {
	s.cutoff = cutoff
	return 0, domain.OperationStats{}, nil
}

type retentionTransaction struct{ ports.Transaction }

func (retentionTransaction) RunWrite(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestUnregisteredTagScanRetentionUsesBerlinCalendarDaysAcrossDST(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ now, cutoff string }{
		{"2026-03-29T12:00:00+02:00", "2025-12-29T12:00:00+01:00"},
		{"2026-10-25T12:00:00+01:00", "2026-07-27T12:00:00+02:00"},
	} {
		t.Run(tt.now, func(t *testing.T) {
			t.Parallel()
			now, err := time.Parse(time.RFC3339, tt.now)
			require.NoError(t, err)
			want, err := time.Parse(time.RFC3339, tt.cutoff)
			require.NoError(t, err)
			for _, location := range []*time.Location{time.UTC, calendar.Berlin} {
				scans := &retentionScans{}
				service := &Service{deps: Dependencies{
					Scans: scans, Tx: retentionTransaction{},
					Now: func() time.Time { return now.In(location) }, Observe: func(ports.Observation) {},
				}}
				_, days, err := service.DeleteUnregisteredTagScansAfterRetention(context.Background())
				require.NoError(t, err)
				require.Equal(t, 90, days)
				require.True(t, want.Add(-retentionClockMargin).Equal(scans.cutoff), "%s clock: got %s", location, scans.cutoff)
			}
		})
	}
}
