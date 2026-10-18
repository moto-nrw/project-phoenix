//go:build testhooks

package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

// DrainEmailForTest drives the real leased worker only for this fixture's
// intent, so parallel test mailers cannot steal another test's messages.
func DrainEmailForTest(ctx context.Context, runtime *Runtime, provider Provider, tenantID, id int64) error {
	store := &fixtureIntentStore{Store: runtime.store, exact: runtime.store, tenantID: tenantID, id: id}
	worker := application.NewWorker(store, providerAdapter{provider: provider}, runtime.observe)
	for !store.finished {
		if _, err := worker.RunOnce(ctx, 1, 3); err != nil {
			return err
		}
		if store.finished {
			return nil
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

type fixtureIntentStore struct {
	ports.Store
	exact        *postgres.Store
	tenantID, id int64
	finished     bool
}

func (s *fixtureIntentStore) Claim(ctx context.Context, transport domain.Transport, _ int, now, expires time.Time) ([]domain.Intent, error) {
	if transport != domain.TransportEmail {
		return nil, nil
	}
	row, acquired, err := s.exact.ClaimEmail(ctx, s.tenantID, s.id, now, expires)
	if err != nil {
		return nil, err
	}
	if acquired {
		return []domain.Intent{row}, nil
	}
	s.finished = row.Status == string(delivery.StateSent) || row.Status == string(delivery.StateDeadLetter) || row.Status == string(delivery.StateCancelled)
	return nil, nil
}

func DeleteSystemEmailForTest(ctx context.Context, runtime *Runtime, id int64) error {
	return runtime.store.DeleteSystemEmailForTest(ctx, id)
}
