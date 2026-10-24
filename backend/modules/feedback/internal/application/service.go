package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
)

type Service struct {
	store    ports.Store
	settings ports.Settings
	tx       ports.Transaction
	observe  ports.Observer
	today    func() domain.Date
}

func New(store ports.Store, settings ports.Settings, tx ports.Transaction, observe ports.Observer, today func() domain.Date) *Service {
	if store == nil || settings == nil || tx == nil || observe == nil || today == nil {
		panic("feedback application: all dependencies are required")
	}
	return &Service{store: store, settings: settings, tx: tx, observe: observe, today: today}
}

func (s *Service) Available(ctx context.Context) (available bool, err error) {
	err = s.run(ctx, "availability", func(txCtx context.Context) (domain.OperationStats, error) {
		available, err = s.settings.FeedbackEnabled(txCtx)
		return domain.OperationStats{}, err
	})
	return available, err
}

// Deletion, its preview counts and retention use run directly so switching
// feedback off never prevents erasure of already stored entries.
func (s *Service) runWhenEnabled(ctx context.Context, operation string, fn func(context.Context) (domain.OperationStats, error)) error {
	return s.run(ctx, operation, func(txCtx context.Context) (domain.OperationStats, error) {
		enabled, err := s.settings.FeedbackEnabled(txCtx)
		if err != nil {
			return domain.OperationStats{}, fmt.Errorf("feedback: resolve availability: %w", err)
		}
		if !enabled {
			return domain.OperationStats{}, feedback.ErrFeatureDisabled
		}
		return fn(txCtx)
	})
}

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context) (domain.OperationStats, error)) (err error) {
	started := time.Now()
	var stats domain.OperationStats
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = s.tx.Run(ctx, func(txCtx context.Context) error {
		stats, err = fn(txCtx)
		return err
	})
	if err != nil {
		stats.Rows = 0
	}
	return err
}
