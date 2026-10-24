package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// SettingUploadRetentionDays is the operator-managed upload window (ADR
// 0048), in days after the decision of the request.
const SettingUploadRetentionDays = "enrollment.upload_retention_days"

// uploadSweepBatchSize caps one sweep pass; the next tick continues.
const uploadSweepBatchSize = 200

// finishedUploadCleanupWindow is how long a settled cleanup intent stays as
// evidence of the removal, like the other file cleanup intents (#4046).
const finishedUploadCleanupWindow = 30 * 24 * time.Hour

// UploadAccessEntry is one staff read of an upload for the GDPR access log.
type UploadAccessEntry struct {
	ActorAccountID int64
	ActorRole      string
	AccessedAt     time.Time
	RequestID      int64
	UploadID       int64
	FieldKey       string
}

// UploadAccessLog appends a staff read of an upload to the access log.
type UploadAccessLog interface {
	RecordUploadAccess(ctx context.Context, entry UploadAccessEntry) error
}

// UploadRetentionSettings resolves tenant overrides or registry defaults.
type UploadRetentionSettings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// RequestUploadDependencies bind the upload reads, the retention and the
// sweep. AccessLog is required to open a file, Settings to expire uploads.
// NotFound reports the records' no-rows error.
type RequestUploadDependencies struct {
	Records   RequestUploadRecords
	AccessLog UploadAccessLog
	Settings  UploadRetentionSettings
	NotFound  func(error) bool
	Logger    *slog.Logger
	Now       func() time.Time
}

// RequestUploads serves the uploads of a request to staff, expires them
// after the school's window and removes the objects of deleted uploads. It
// runs in the caller's tenant transaction.
type RequestUploads struct {
	deps RequestUploadDependencies
}

var (
	_ RequestUploadReads = (*RequestUploads)(nil)
	_ UploadRetention    = (*RequestUploads)(nil)
	_ UploadObjectSweep  = (*RequestUploads)(nil)
)

// NewRequestUploads binds the service to the owner's records.
func NewRequestUploads(deps RequestUploadDependencies) *RequestUploads {
	if deps.Records == nil {
		panic("enrollment request uploads: records are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &RequestUploads{deps: deps}
}

// ListRequestUploads returns the live uploads of the request, oldest first.
func (u *RequestUploads) ListRequestUploads(ctx context.Context, requestID int64) ([]RequestUpload, error) {
	if requestID <= 0 {
		return nil, ErrUploadNotFound
	}
	rows, err := u.deps.Records.RequestUploads(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list request uploads: %w", err)
	}
	out := make([]RequestUpload, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	return out, nil
}

// OpenRequestUpload resolves an upload of the request and records the read.
// The file is only handed out once the access log row is written; a failed
// record refuses the file.
func (u *RequestUploads) OpenRequestUpload(ctx context.Context, access UploadAccess) (*RequestUpload, error) {
	if u.deps.AccessLog == nil {
		return nil, errors.New("enrollment upload access log is not configured")
	}
	upload, err := u.deps.Records.RequestUpload(ctx, access.RequestID, access.UploadID)
	if err != nil {
		if u.deps.NotFound != nil && u.deps.NotFound(err) {
			return nil, ErrUploadNotFound
		}
		return nil, fmt.Errorf("load request upload: %w", err)
	}
	entry := UploadAccessEntry{
		ActorAccountID: access.ActorAccountID,
		ActorRole:      access.ActorRole,
		AccessedAt:     u.deps.Now(),
		RequestID:      access.RequestID,
		UploadID:       upload.ID,
		FieldKey:       upload.FieldKey,
	}
	if err := u.deps.AccessLog.RecordUploadAccess(ctx, entry); err != nil {
		return nil, fmt.Errorf("record upload access: %w", err)
	}
	return upload, nil
}

// DeleteExpiredUploads deletes the unclaimed uploads after a day, the
// uploads of decided requests after the school's window and the settled
// cleanup intents after 30 days. Each upload deletion
// queues its object; the sweep removes the bytes. It never guesses a window:
// an unreadable or non-positive one fails the run.
func (u *RequestUploads) DeleteExpiredUploads(ctx context.Context) (int, int, error) {
	if u.deps.Settings == nil {
		return 0, 0, errors.New("enrollment upload retention is not configured")
	}
	days, err := u.deps.Settings.ResolveInt(ctx, SettingUploadRetentionDays)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve enrollment upload retention: %w", err)
	}
	if days <= 0 {
		return 0, 0, fmt.Errorf("invalid enrollment upload retention days: %d", days)
	}
	now := u.deps.Now()
	deleted, err := u.deps.Records.DeleteExpiredRequestUploads(ctx,
		now.Add(-UnclaimedUploadLifetime), now.Add(-time.Duration(days)*24*time.Hour))
	if err != nil {
		return 0, days, fmt.Errorf("delete expired enrollment uploads: %w", err)
	}
	finished, err := u.deps.Records.DeleteFinishedUploadCleanups(ctx, now.Add(-finishedUploadCleanupWindow))
	if err != nil {
		return deleted, days, fmt.Errorf("delete finished enrollment upload cleanups: %w", err)
	}
	return deleted + finished, days, nil
}

// SweepUploadObjects removes the objects of deleted uploads in the caller's
// tenant transaction. The intents stay locked until it commits, so two passes
// never race on one object. A failed removal leaves its intent for the next
// pass; the failures are reported next to the completed ones.
func (u *RequestUploads) SweepUploadObjects(ctx context.Context, remove func(context.Context, int64, string) error) (int, error) {
	if remove == nil {
		return 0, errors.New("enrollment upload object removal is not configured")
	}
	intents, err := u.deps.Records.PendingUploadCleanups(ctx, uploadSweepBatchSize)
	if err != nil {
		return 0, fmt.Errorf("list enrollment upload cleanups: %w", err)
	}
	var removed int
	var failures []error
	for _, intent := range intents {
		if err := remove(ctx, intent.TenantID, intent.StoredName); err != nil {
			failures = append(failures, fmt.Errorf("remove enrollment upload object: %w", err))
			continue
		}
		if err := u.deps.Records.CompleteUploadCleanup(ctx, intent.ID, u.deps.Now()); err != nil {
			return removed, errors.Join(append(failures, err)...)
		}
		removed++
	}
	if len(intents) >= uploadSweepBatchSize {
		u.deps.Logger.Info("enrollment upload cleanup batch full, more pending",
			slog.Int("batch_size", uploadSweepBatchSize))
	}
	return removed, errors.Join(failures...)
}
