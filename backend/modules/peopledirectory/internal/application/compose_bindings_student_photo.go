package application

import (
	context "context"
	errors "errors"
	time "time"

	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

type DisabledStudentPhotos struct{}

func (DisabledStudentPhotos) PhotoFeatureEnabled(context.Context) (bool, error)    { return false, nil }
func (DisabledStudentPhotos) ActingAccountID(context.Context) int64                { return 0 }
func (DisabledStudentPhotos) QueueUnlinkStoredPhoto(context.Context, string) error { return nil }
func (DisabledStudentPhotos) UnlinkStoredPhoto(string)                             {}
func (DisabledStudentPhotos) BroadcastPhotoChange(int64, int64, string)            {}
func (DisabledStudentPhotos) RecordPhotoConsent(context.Context, internalports.StudentPhotoConsentSnapshot, internalports.StudentPhotoConsentSnapshot, *int64, time.Time) error {
	return ErrStudentPhotoRuntimeUnbound
}

var ErrStudentPhotoRuntimeUnbound = errors.New("people directory: student photo runtime is not bound")

// studentPhotoPort is the root's photo runtime, or the disabled one.
func StudentPhotoPort(runtime StudentPhotoRuntime) internalports.StudentPhotoRuntime {
	if runtime == nil {
		return DisabledStudentPhotos{}
	}
	return runtime
}

type StudentPhotoRuntime interface {
	PhotoFeatureEnabled(ctx context.Context) (bool, error)
	ActingAccountID(ctx context.Context) int64
	QueueUnlinkStoredPhoto(context.Context, string) error
	UnlinkStoredPhoto(storedURL string)
	BroadcastPhotoChange(tenantID, studentID int64, source string)
	RecordPhotoConsent(
		ctx context.Context,
		before, after internalports.StudentPhotoConsentSnapshot,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}
