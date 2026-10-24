//go:build testhooks

package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

type exactEmailClaim interface {
	ClaimEmail(context.Context, int64, int64, time.Time, time.Time) (domain.Intent, bool, error)
}

// DrainEmailForTest drives the real leased worker only for this fixture's
// intent, so parallel test mailers cannot steal another test's messages.
func DrainEmailForTest(ctx context.Context, base ports.Store, provider ports.Provider, observe ports.Observer, tenantID, id int64) error {
	store := &fixtureIntentStore{Store: base, exact: base.(exactEmailClaim), tenantID: tenantID, id: id}
	worker := NewWorker(store, provider, observe)
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
	exact        exactEmailClaim
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
	s.finished = row.Status == string(domain.StateSent) || row.Status == string(domain.StateDeadLetter) || row.Status == string(domain.StateCancelled)
	return nil, nil
}

// DeleteSystemEmailForTest owns cleanup's independent unit of work just as
// worker transitions do. The row adapter requires the existing admin scope.
func DeleteSystemEmailForTest(ctx context.Context, commands *WorkerCommands, id int64) error {
	return commands.transactions.WithinAdmin(ctx, func(txCtx context.Context) error {
		return commands.WorkerRecords.(interface {
			DeleteSystemEmailForTest(context.Context, int64) error
		}).DeleteSystemEmailForTest(txCtx, id)
	})
}
