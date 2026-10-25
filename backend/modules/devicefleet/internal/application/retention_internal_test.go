package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

func TestUnregisteredTagScanRetentionUsesBerlinCalendarDays(t *testing.T) {
	t.Parallel()

	// The PostgreSQL retention capability runs with Europe/Berlin. Just after
	// the DST fallback, subtracting days in UTC would produce a cutoff one hour
	// later than the database allows.
	now := time.Date(2026, time.October, 25, 1, 30, 0, 0, time.UTC)
	store := &retentionScanStore{}
	service := &Service{deps: Dependencies{
		Scans:   store,
		Tx:      retentionTransaction{},
		Now:     func() time.Time { return now },
		Observe: func(ports.Observation) {},
	}}

	_, days, err := service.DeleteUnregisteredTagScansAfterRetention(context.Background())
	if err != nil {
		t.Fatalf("DeleteUnregisteredTagScansAfterRetention() error = %v", err)
	}
	if days != devicefleet.UnregisteredTagScanRetentionDays {
		t.Fatalf("retention days = %d, want %d", days, devicefleet.UnregisteredTagScanRetentionDays)
	}
	want := now.In(calendar.Berlin).AddDate(0, 0, -devicefleet.UnregisteredTagScanRetentionDays).Add(-retentionClockMargin)
	if !store.cutoff.Equal(want) {
		t.Errorf("retention cutoff = %s, want %s", store.cutoff, want)
	}
}
