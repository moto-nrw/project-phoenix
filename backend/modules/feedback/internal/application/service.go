package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
)

var (
	ErrInvalidSettings = errors.New("feedback settings are invalid")
)

type CleanupResult struct {
	Rows          int
	RetentionDays int
}

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

func (s *Service) Create(ctx context.Context, input domain.Entry) (entry domain.Entry, err error) {
	err = s.run(ctx, "submit", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		entry, stats, err = s.store.Create(txCtx, input)
		return stats, err
	})
	return entry, err
}

func (s *Service) CreateBatch(ctx context.Context, inputs []domain.Entry) (entries []domain.Entry, err error) {
	err = s.run(ctx, "submit_batch", func(txCtx context.Context) (stats domain.OperationStats, batchErr error) {
		entries = make([]domain.Entry, 0, len(inputs))
		for _, input := range inputs {
			entry, current, createErr := s.store.Create(txCtx, input)
			stats.Queries += current.Queries
			stats.Rows += current.Rows
			stats.StatementDuration += current.StatementDuration
			if createErr != nil {
				return stats, createErr
			}
			entries = append(entries, entry)
		}
		return stats, nil
	})
	return entries, err
}

func (s *Service) Get(ctx context.Context, id int64) (entry domain.Entry, err error) {
	err = s.run(ctx, "read", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		entry, stats, err = s.store.Get(txCtx, id)
		return stats, err
	})
	return entry, err
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.run(ctx, "delete", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.store.Delete(txCtx, id)
	})
}

func (s *Service) List(ctx context.Context, filter domain.Filter) (entries []domain.Entry, err error) {
	err = s.run(ctx, "read_list", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		entries, stats, err = s.store.List(txCtx, filter)
		return stats, err
	})
	return entries, err
}

func (s *Service) Cleanup(ctx context.Context) (result CleanupResult, err error) {
	err = s.run(ctx, "retention_cleanup", func(txCtx context.Context) (domain.OperationStats, error) {
		retentionDays, resolveErr := s.settings.FeedbackRetentionDays(txCtx)
		if resolveErr != nil {
			return domain.OperationStats{}, fmt.Errorf("feedback: resolve retention: %w", resolveErr)
		}
		if retentionDays <= 0 {
			return domain.OperationStats{}, ErrInvalidSettings
		}
		result.RetentionDays = retentionDays
		cutoff := s.today().AddDays(-retentionDays)
		stats, deleteErr := s.store.DeleteOlderThan(txCtx, cutoff)
		result.Rows = int(stats.Rows)
		return stats, deleteErr
	})
	return result, err
}

func (s *Service) CountForStudent(ctx context.Context, studentID int64) (count int, err error) {
	err = s.run(ctx, "count_student", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		count, stats, err = s.store.CountForStudent(txCtx, studentID)
		return stats, err
	})
	return count, err
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
