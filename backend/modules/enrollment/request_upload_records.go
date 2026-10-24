package enrollment

import (
	"context"
	"time"
)

// RequestUploadRecords are the owner's rows of the files parents attach to
// their requests (#4783). Every method requires the tenant transaction.
type RequestUploadRecords interface {
	InsertRequestUpload(ctx context.Context, upload *RequestUpload) error
	DeleteRequestUpload(ctx context.Context, uploadID int64) error
	// UnclaimedRequestUploads locks the unclaimed uploads of one phase field
	// whose token hash is listed and that were created after createdAfter.
	UnclaimedRequestUploads(ctx context.Context, phaseID int64, fieldKey string, tokenHashes []string, createdAfter time.Time) ([]*RequestUpload, error)
	ClaimRequestUploads(ctx context.Context, uploadIDs []int64, requestID int64, claimedAt time.Time) error
	RequestUploads(ctx context.Context, requestID int64) ([]*RequestUpload, error)
	// RequestUpload loads one upload of a request; a missing or foreign one
	// reports the store's no-rows error.
	RequestUpload(ctx context.Context, requestID, uploadID int64) (*RequestUpload, error)
	DeleteRequestUploads(ctx context.Context, requestID int64, uploadIDs []int64) error
	// DeleteExpiredRequestUploads deletes the unclaimed uploads created
	// before unclaimedBefore and the uploads of requests whose children were
	// all decided before decidedBefore.
	DeleteExpiredRequestUploads(ctx context.Context, unclaimedBefore, decidedBefore time.Time) (int, error)
	// PendingUploadCleanups locks a batch of queued object removals; the
	// caller removes the objects and completes the intents in the same
	// transaction.
	PendingUploadCleanups(ctx context.Context, limit int) ([]UploadCleanupIntent, error)
	CompleteUploadCleanup(ctx context.Context, intentID int64, at time.Time) error
	// DeleteFinishedUploadCleanups deletes the intents whose object was
	// removed before the cutoff.
	DeleteFinishedUploadCleanups(ctx context.Context, cleanedBefore time.Time) (int, error)
}
