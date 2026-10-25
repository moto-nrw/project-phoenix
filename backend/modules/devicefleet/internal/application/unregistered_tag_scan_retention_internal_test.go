package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

type retentionScanStore struct {
	ports.UnregisteredTagScanStore
	cutoff time.Time
}

func (s *retentionScanStore) DeleteExpired(_ context.Context, cutoff time.Time) (int64, domain.OperationStats, error) {
	s.cutoff = cutoff
	return 0, domain.OperationStats{}, nil
}

type retentionTransaction struct{ ports.Transaction }

func (retentionTransaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

func TestDeleteUnregisteredTagScansAfterRetentionUsesFixedUTCWindow(t *testing.T) {
	t.Parallel()

	// Migration 1.15.606 evaluates the database guard in UTC. The October
	// DST rollback must not shorten the fixed 90 times 24-hour window.
	now := time.Date(2026, time.October, 25, 1, 30, 0, 0, time.UTC)
	store := &retentionScanStore{}
	service := &Service{deps: Dependencies{
		Scans: store,
		Tx:    retentionTransaction{},
		Now:   func() time.Time { return now },
		Observe: func(ports.Observation) {
		},
	}}

	_, days, err := service.DeleteUnregisteredTagScansAfterRetention(context.Background())

	if err != nil {
		t.Fatalf("DeleteUnregisteredTagScansAfterRetention() error = %v", err)
	}
	if days != devicefleet.UnregisteredTagScanRetentionDays {
		t.Fatalf("retention days = %d, want %d", days, devicefleet.UnregisteredTagScanRetentionDays)
	}
	want := now.Add(-time.Duration(devicefleet.UnregisteredTagScanRetentionDays)*24*time.Hour - retentionClockMargin)
	if !store.cutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", store.cutoff, want)
	}
}
