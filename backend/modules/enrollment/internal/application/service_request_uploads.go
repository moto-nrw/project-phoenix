package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The upload rows of #4783. Each one requires the tenant transaction, so an
// upload row is only ever read or written under RLS of its school.

func (m *Service) InsertRequestUpload(ctx context.Context, upload *enrollment.RequestUpload) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertRequestUpload(ctx, upload)
}

func (m *Service) DeleteRequestUpload(ctx context.Context, uploadID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequestUpload(ctx, uploadID)
}

func (m *Service) UnclaimedRequestUploads(ctx context.Context, phaseID int64, fieldKey string, tokenHashes []string, createdAfter time.Time) ([]*enrollment.RequestUpload, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.UnclaimedRequestUploads(ctx, phaseID, fieldKey, tokenHashes, createdAfter)
}

func (m *Service) ClaimRequestUploads(ctx context.Context, uploadIDs []int64, requestID int64, claimedAt time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.ClaimRequestUploads(ctx, uploadIDs, requestID, claimedAt)
}

func (m *Service) RequestUploads(ctx context.Context, requestID int64) ([]*enrollment.RequestUpload, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.RequestUploads(ctx, requestID)
}

func (m *Service) RequestUpload(ctx context.Context, requestID, uploadID int64) (*enrollment.RequestUpload, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.RequestUpload(ctx, requestID, uploadID)
}

func (m *Service) DeleteRequestUploads(ctx context.Context, requestID int64, uploadIDs []int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequestUploads(ctx, requestID, uploadIDs)
}

func (m *Service) DeleteExpiredRequestUploads(ctx context.Context, unclaimedBefore, decidedBefore time.Time) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.DeleteExpiredRequestUploads(ctx, unclaimedBefore, decidedBefore)
}

func (m *Service) PendingUploadCleanups(ctx context.Context, limit int) ([]enrollment.UploadCleanupIntent, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PendingUploadCleanups(ctx, limit)
}

func (m *Service) CompleteUploadCleanup(ctx context.Context, intentID int64, at time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.CompleteUploadCleanup(ctx, intentID, at)
}

func (m *Service) DeleteFinishedUploadCleanups(ctx context.Context, cleanedBefore time.Time) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.DeleteFinishedUploadCleanups(ctx, cleanedBefore)
}
