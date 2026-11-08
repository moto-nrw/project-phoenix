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

// CodeUploadExpired is the wire code of an upload the token no longer claims.
// The package may not import api/common.
const CodeUploadExpired = "enrollment.upload_expired"

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
// transport detected ContentType from the bytes and measured SizeBytes. The
// optional authorization tokens are verified by the intake before it reserves
// storage for the upload.
type UploadInput struct {
	TenantID        int64
	PhaseID         int64
	FieldKey        string
	Filename        string
	ContentType     string
	Extension       string
	SizeBytes       int64
	RemoteIP        string
	LateInviteToken string
	StatusToken     string
	// EnrolleeAccess comes only from the authenticated parents-portal route.
	// The anonymous HTTP route always leaves this at its fail-closed zero value.
	EnrolleeAccess EnrolleeAudienceAccess
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
// transaction. CountUploadAttempt counts the attempt in the IP's upload
// bucket before the caller reads the body; PrepareUpload checks the field and
// stores the row. Once that commits the caller writes the bytes and, when
// that fails, deletes the row again through RequestUploadRecords.
type UploadIntake interface {
	CountUploadAttempt(ctx context.Context, tenantID int64, remoteIP string) error
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
