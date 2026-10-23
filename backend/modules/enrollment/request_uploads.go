package enrollment

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// RequestUploadStorageKind is the private object-store prefix of the files
// parents attach to an enrollment request (#4783).
const RequestUploadStorageKind = "enrollment-uploads"

// UnclaimedUploadLifetime is how long an upload waits for its submission.
// After it the token no longer claims the upload and the nightly retention
// deletes it.
const UnclaimedUploadLifetime = 24 * time.Hour

// SubmissionRateLimitKeyTypeUploadIP counts the uploads of one IP address.
const SubmissionRateLimitKeyTypeUploadIP = "upload_ip"

// Wire codes of the upload failures. The package may not import api/common.
const (
	CodeUploadInvalid     = "enrollment.upload_invalid"
	CodeUploadExpired     = "enrollment.upload_expired"
	CodeUploadRateLimited = "enrollment.upload_rate_limited"
	CodeUploadNotFound    = "enrollment.upload_not_found"
)

var (
	// ErrUploadFieldNotFound marks an upload for a phase or field that does
	// not take files.
	ErrUploadFieldNotFound = errors.New("enrollment upload field not found")
	// ErrUploadNotFound marks a file that is gone or belongs to another
	// request.
	ErrUploadNotFound = errors.New("enrollment upload not found")
	// ErrUploadRateLimited marks an IP over its upload threshold.
	ErrUploadRateLimited = errors.New("too many enrollment uploads")
)

// RequestUpload is one file a parent attached to an enrollment request.
// StoredName is the object name in the private store; it never leaves the
// backend.
type RequestUpload struct {
	ID          int64
	TenantID    int64
	PhaseID     int64
	RequestID   *int64
	FieldKey    string
	Filename    string
	StoredName  string
	SizeBytes   int64
	ContentType string
	TokenHash   string
	CreatedAt   time.Time
	ClaimedAt   *time.Time
}

// Answer is the stored answer item of the upload.
func (u *RequestUpload) Answer() FileAnswer {
	return FileAnswer{UploadID: strconv.FormatInt(u.ID, 10), Filename: u.Filename, SizeBytes: u.SizeBytes, ContentType: u.ContentType}
}

// UploadInput is one validated file a parent uploads for a file field. The
// transport detected ContentType from the bytes and measured SizeBytes.
type UploadInput struct {
	TenantID    int64
	PhaseID     int64
	FieldKey    string
	Filename    string
	ContentType string
	Extension   string
	SizeBytes   int64
	RemoteIP    string
}

// PreparedUpload is the stored row of an upload whose bytes still have to be
// written, with the token the uploader claims it with.
type PreparedUpload struct {
	Upload *RequestUpload
	Token  string
}

// UploadRejection carries the limits of the field an upload did not fit.
type UploadRejection struct {
	MaxMB        int
	AllowedTypes []string
}

// Error implements error.
func (r *UploadRejection) Error() string { return "upload does not fit the file field" }

// UploadIntake stores the uploads of the public form in the caller's tenant
// transaction. PrepareUpload checks the field and the rate limit and stores
// the row; once that commits the caller writes the bytes and, when that
// fails, deletes the row again through RequestUploadRecords.
type UploadIntake interface {
	PrepareUpload(ctx context.Context, input UploadInput) (*PreparedUpload, error)
}

// UploadAccess is one staff read of an upload: the actor and their role.
type UploadAccess struct {
	RequestID      int64
	UploadID       int64
	ActorAccountID int64
	ActorRole      string
}

// RequestUploadReads serves the uploads of a request to staff. OpenUpload
// records the access before it returns the file; without the record the
// file is not served.
type RequestUploadReads interface {
	ListRequestUploads(ctx context.Context, requestID int64) ([]RequestUpload, error)
	OpenRequestUpload(ctx context.Context, access UploadAccess) (*RequestUpload, error)
}

// UploadRetention deletes the uploads of the school in context: unclaimed
// ones after UnclaimedUploadLifetime, those of a decided request after the
// school's window. It reports the deleted rows and the window in days.
type UploadRetention interface {
	DeleteExpiredUploads(ctx context.Context) (deleted, retentionDays int, err error)
}

// UploadObjectSweep removes the stored objects of deleted uploads of the
// school in context and settles their cleanup intents.
type UploadObjectSweep interface {
	SweepUploadObjects(ctx context.Context, remove func(ctx context.Context, tenantID int64, storedName string) error) (int, error)
}

// UploadCleanupIntent is one queued removal of a deleted upload's object.
type UploadCleanupIntent struct {
	ID         int64
	TenantID   int64
	StoredName string
}
