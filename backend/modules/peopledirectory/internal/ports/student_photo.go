package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentPhotoStore is the persistence port over the photo columns of
// users.students and the per-tenant photo-feature gate.
type StudentPhotoStore interface {
	// FindPhoto reads the photo slice without locking.
	FindPhoto(ctx context.Context, studentID int64) (domain.StudentPhoto, bool, domain.OperationStats, error)
	// LockPhoto re-reads the photo slice under a row lock.
	LockPhoto(ctx context.Context, studentID int64) (domain.StudentPhoto, bool, domain.OperationStats, error)
	// LockPhotoFeature takes the per-tenant advisory lock that serializes
	// photo writes against a feature-disable purge.
	LockPhotoFeature(ctx context.Context) (domain.OperationStats, error)
	// SavePhoto writes the photo slice of one child.
	SavePhoto(ctx context.Context, photo domain.StudentPhoto) (domain.OperationStats, error)
	// PurgePhotos clears every stored photo of the tenant and returns the
	// URLs it detached.
	PurgePhotos(ctx context.Context) ([]string, domain.OperationStats, error)
}

// StudentPhotoRuntime is the caller-supplied surface the photo lifecycle
// needs beyond its own rows: the tenant feature flag, the stored-file cleanup,
// the live refresh, the identity of the acting account and the consent trail.
// Every one of them belongs to another owner, so they arrive as ports instead
// of imports. Who may reach a photo is decided by the caller, like every other
// child-data route.
type StudentPhotoRuntime interface {
	// PhotoFeatureEnabled resolves the tenant's student-photo setting.
	PhotoFeatureEnabled(ctx context.Context) (bool, error)
	// ActingAccountID identifies the account a granted consent is recorded
	// for; 0 when the caller is not an authenticated account.
	ActingAccountID(ctx context.Context) int64
	// UnlinkStoredPhoto deletes a detached file after the commit.
	UnlinkStoredPhoto(storedURL string)
	// BroadcastPhotoChange wakes open tabs; source labels the reason.
	BroadcastPhotoChange(tenantID, studentID int64, source string)
	// RecordPhotoConsent appends the Audit Platform trail entry for a
	// consent that changed with the photo.
	RecordPhotoConsent(ctx context.Context, before, after domain.StudentConsentSnapshot, actorAccountID *int64, changedAt time.Time) error
}
