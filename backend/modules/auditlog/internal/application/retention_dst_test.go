package audit

import (
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

func TestAuditRetentionCutoffUsesBerlinCalendarAcrossDST(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		now, cutoff string
		years, days int
	}{
		{"2026-03-29T12:00:00+02:00", "2025-12-29T12:00:00+01:00", 0, AuthEventRetentionDays},
		{"2026-10-25T12:00:00+01:00", "2026-07-27T12:00:00+02:00", 0, AuthEventRetentionDays},
		{"2026-03-29T12:00:00+02:00", "2023-03-29T12:00:00+02:00", DeletionLogRetentionYears, 0},
		{"2026-10-25T12:00:00+01:00", "2023-10-25T12:00:00+02:00", DeletionLogRetentionYears, 0},
	} {
		now, err := time.Parse(time.RFC3339, tt.now)
		require.NoError(t, err)
		want, err := time.Parse(time.RFC3339, tt.cutoff)
		require.NoError(t, err)
		for _, location := range []*time.Location{time.UTC, calendar.Berlin} {
			got := auditRetentionCutoff(now.In(location), tt.years, tt.days)
			require.True(t, want.Add(-auditModels.RetentionClockMargin).Equal(got), "%s, %s clock: got %s", tt.now, location, got)
		}
	}
}
