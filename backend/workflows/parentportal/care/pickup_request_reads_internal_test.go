package care

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type pickupRequestBaseline struct {
	careplan.PickupScheduleService
	calls int
}

func (b *pickupRequestBaseline) GetEffectivePickupTimeForDate(context.Context, int64, timezone.Date) (*careplan.EffectivePickupTime, error) {
	b.calls++
	clock := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC))
	return &careplan.EffectivePickupTime{PickupTime: &clock}, nil
}

func TestPickupRequestBaselineEnrichmentPreservesFrozenHistory(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, status, payload, want string
		calls                       int
	}{
		{"pending without baseline", "pending",
			`{"date":"2026-09-21","pickup_time":"16:00","reason":"Termin"}`,
			`{"date":"2026-09-21","pickup_time":"16:00","reason":"Termin","previous_pickup_time":"15:30"}`, 1},
		{"pending with frozen baseline", "pending",
			`{"date":"2026-09-21","pickup_time":"16:00","previous_pickup_time":"14:00"}`,
			`{"date":"2026-09-21","pickup_time":"16:00","previous_pickup_time":"14:00"}`, 0},
		{"decided without baseline", "approved",
			`{"date":"2026-09-21","pickup_time":"16:00"}`,
			`{"date":"2026-09-21","pickup_time":"16:00"}`, 0},
		{"invalid historical date", "pending",
			`{"date":"invalid","pickup_time":"16:00"}`,
			`{"date":"invalid","pickup_time":"16:00"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := &pickupRequestBaseline{}
			svc := &Service{Config: Config{PickupSchedules: baseline, Logger: slog.Default()}}
			original := []byte(tt.payload)
			snapshot := []byte(`{"diff":[{"label":"Frozen","old":"14:00","new":"16:00"}]}`)
			rows := []carerequests.Request{{Status: tt.status, Payload: original, DecisionSnapshot: snapshot}}
			svc.enrichLegacyPickupChangeRequests(context.Background(), rows[0].StudentID, rows)
			require.Len(t, rows, 1)
			assert.JSONEq(t, tt.want, string(rows[0].Payload))
			assert.Equal(t, tt.payload, string(original), "the persisted payload buffer must not be changed")
			assert.Equal(t, `{"diff":[{"label":"Frozen","old":"14:00","new":"16:00"}]}`, string(rows[0].DecisionSnapshot))
			assert.Equal(t, tt.calls, baseline.calls)
		})
	}
}
