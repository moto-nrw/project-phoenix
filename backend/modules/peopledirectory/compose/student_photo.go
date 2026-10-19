package compose

import (
	"context"
	"errors"
	"time"

	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentPhotoConsentSnapshot is the photo slice of a consent change the
// composition root records in the Audit Platform trail.
type StudentPhotoConsentSnapshot = internalports.StudentPhotoConsentSnapshot

// StudentPhotoRuntime is everything the photo lifecycle needs beyond the
// directory's own rows: the tenant feature flag (Settings Platform), the
// acting account (Identity), the stored-file cleanup and live refresh
// (Delivery), and the consent trail (Audit Platform). The composition root
// binds them, so the owner keeps its rules without importing four other
// owners.
type StudentPhotoRuntime interface {
	PhotoFeatureEnabled(ctx context.Context) (bool, error)
	ActingAccountID(ctx context.Context) int64
	QueueUnlinkStoredPhoto(context.Context, string) error
	UnlinkStoredPhoto(storedURL string)
	BroadcastPhotoChange(tenantID, studentID int64, source string)
	RecordPhotoConsent(
		ctx context.Context,
		before, after StudentPhotoConsentSnapshot,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}

type disabledStudentPhotos struct{}

func (disabledStudentPhotos) PhotoFeatureEnabled(context.Context) (bool, error)    { return false, nil }
func (disabledStudentPhotos) ActingAccountID(context.Context) int64                { return 0 }
func (disabledStudentPhotos) QueueUnlinkStoredPhoto(context.Context, string) error { return nil }
func (disabledStudentPhotos) UnlinkStoredPhoto(string)                             {}
func (disabledStudentPhotos) BroadcastPhotoChange(int64, int64, string)            {}
func (disabledStudentPhotos) RecordPhotoConsent(context.Context, StudentPhotoConsentSnapshot, StudentPhotoConsentSnapshot, *int64, time.Time) error {
	return errStudentPhotoRuntimeUnbound
}

var errStudentPhotoRuntimeUnbound = errors.New("people directory: student photo runtime is not bound")

// studentPhotoPort is the root's photo runtime, or the disabled one.
func studentPhotoPort(runtime StudentPhotoRuntime) internalports.StudentPhotoRuntime {
	if runtime == nil {
		return disabledStudentPhotos{}
	}
	return runtime
}
