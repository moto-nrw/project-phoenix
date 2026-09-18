package users

import (
	"context"
	"errors"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The contracts below are what the retained services and handlers call for the
// child's photo. The lifecycle behind them — the feature gate, the consent it
// depends on, the row lock and the purge — belongs to the People Directory
// owner; #3349 moved it there. The composition root binds an implementation
// (services.NewStudentPhotos).

// ErrPhotoNoTenant refuses a photo route reached without a tenant context.
// Every other outcome is the owner's (modules/peopledirectory).
var ErrPhotoNoTenant = errors.New("no tenant context")

// PhotoSettings is the narrow settings surface the photo feature gate needs.
type PhotoSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// PhotoUnlinker deletes a previously-stored photo file.
type PhotoUnlinker interface {
	UnlinkStored(storedURL string)
}

// CommitUploadRequest carries an already-stored upload into the commit path.
type CommitUploadRequest struct {
	StudentID    int64
	NewStoredURL string
	ConsentAck   bool
}

// StudentPhotoService is the tenant-scoped student-photo lifecycle.
type StudentPhotoService interface {
	CommitUpload(ctx context.Context, req CommitUploadRequest) error
	CommitDelete(ctx context.Context, studentID int64) (clearedURL string, err error)
	LookupForRead(ctx context.Context, studentID int64, filename string) (storedURL string, err error)
	PurgeAllPhotos(ctx context.Context, tenantID int64) (postCommit func(), err error)
	HandleFeatureToggle(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
	// ApplyConsentTransition reconciles a requested photo consent against the
	// row the caller holds, mutating it in place; the caller persists it. The
	// file a withdrawal detaches is removed after the caller's commit.
	ApplyConsentTransition(ctx context.Context, requestedConsent *bool, fresh *userModels.Student)
	ScheduleUnlinkAfterCommit(ctx context.Context, storedURL string)
}

// StudentConsentRecorder appends the Audit Platform trail entry for an
// effective consent change. The composition root binds it
// (database/repositories.NewStudentConsentsFor); Audit Platform owns the trail.
type StudentConsentRecorder interface {
	RecordTransitions(
		ctx context.Context,
		before, after *userModels.Student,
		source string,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}
