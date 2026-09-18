package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentPhotoConsentSnapshot is the photo slice of a consent change the
// composition root records in the Audit Platform trail.
type StudentPhotoConsentSnapshot struct {
	StudentID           int64
	PhotoConsentGivenAt *time.Time
}

// StudentPhotoRuntime is everything the photo lifecycle needs beyond the
// directory's own rows: the tenant feature flag (Settings Platform), the
// acting account (Identity), the stored-file cleanup and live refresh
// (Delivery), and the consent trail (Audit Platform). The composition root
// binds them, so the owner keeps its rules without importing four other
// owners.
type StudentPhotoRuntime interface {
	PhotoFeatureEnabled(ctx context.Context) (bool, error)
	ActingAccountID(ctx context.Context) int64
	UnlinkStoredPhoto(storedURL string)
	BroadcastPhotoChange(tenantID, studentID int64, source string)
	RecordPhotoConsent(
		ctx context.Context,
		before, after StudentPhotoConsentSnapshot,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}

// studentPhotoRuntime adapts the composition root's public-typed seam to the
// module's internal port.
type studentPhotoRuntime struct{ resolve func() StudentPhotoRuntime }

// bound resolves the runtime the composition root eventually supplies. Until
// it does, every photo route reports the feature as disabled rather than
// writing through half a runtime.
func (a studentPhotoRuntime) bound() StudentPhotoRuntime {
	if a.resolve == nil {
		return nil
	}
	return a.resolve()
}

func (a studentPhotoRuntime) PhotoFeatureEnabled(ctx context.Context) (bool, error) {
	runtime := a.bound()
	if runtime == nil {
		return false, nil
	}
	return runtime.PhotoFeatureEnabled(ctx)
}
func (a studentPhotoRuntime) ActingAccountID(ctx context.Context) int64 {
	runtime := a.bound()
	if runtime == nil {
		return 0
	}
	return runtime.ActingAccountID(ctx)
}
func (a studentPhotoRuntime) UnlinkStoredPhoto(storedURL string) {
	if runtime := a.bound(); runtime != nil {
		runtime.UnlinkStoredPhoto(storedURL)
	}
}
func (a studentPhotoRuntime) BroadcastPhotoChange(tenantID, studentID int64, source string) {
	if runtime := a.bound(); runtime != nil {
		runtime.BroadcastPhotoChange(tenantID, studentID, source)
	}
}

func (a studentPhotoRuntime) RecordPhotoConsent(
	ctx context.Context,
	before, after domain.StudentConsentSnapshot,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	runtime := a.bound()
	if runtime == nil {
		return errors.New("people directory: student photo runtime is not bound")
	}
	return runtime.RecordPhotoConsent(
		ctx,
		StudentPhotoConsentSnapshot{StudentID: before.StudentID, PhotoConsentGivenAt: before.PhotoConsentGivenAt},
		StudentPhotoConsentSnapshot{StudentID: after.StudentID, PhotoConsentGivenAt: after.PhotoConsentGivenAt},
		actorAccountID,
		changedAt,
	)
}

func (e engine) FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error) {
	storedURL, err := e.studentPhotos.FindPhoto(ctx, studentID, filename)
	return storedURL, mapPhotoError(err)
}

func (e engine) CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error {
	return mapPhotoError(e.studentPhotos.CommitPhoto(ctx, studentID, storedURL, consentAck))
}

func (e engine) ClearStudentPhoto(ctx context.Context, studentID int64) (string, error) {
	clearedURL, err := e.studentPhotos.ClearPhoto(ctx, studentID)
	return clearedURL, mapPhotoError(err)
}

func (e engine) PurgeStudentPhotos(ctx context.Context) ([]string, error) {
	urls, err := e.studentPhotos.PurgePhotos(ctx)
	return urls, mapPhotoError(err)
}

func (e engine) ApplyStudentPhotoConsent(
	ctx context.Context,
	current peopledirectory.StudentPhotoState,
	requestedConsent *bool,
) peopledirectory.StudentPhotoState {
	updated := e.studentPhotos.ApplyPhotoConsent(ctx, domain.StudentPhoto{
		StudentID:           current.StudentID,
		PhotoPath:           current.PhotoPath,
		PhotoConsentGivenAt: current.PhotoConsentGivenAt,
		PhotoConsentGivenBy: current.PhotoConsentGivenBy,
	}, requestedConsent)
	return peopledirectory.StudentPhotoState{
		StudentID:           updated.StudentID,
		PhotoPath:           updated.PhotoPath,
		PhotoConsentGivenAt: updated.PhotoConsentGivenAt,
		PhotoConsentGivenBy: updated.PhotoConsentGivenBy,
	}
}

func (e engine) ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string) {
	e.studentPhotos.ScheduleUnlink(ctx, storedURL)
}

func mapPhotoError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrPhotoFeatureDisabledMid):
		return peopledirectory.ErrPhotoFeatureDisabledMid
	case errors.Is(err, domain.ErrPhotoFeatureDisabled):
		return peopledirectory.ErrPhotoFeatureDisabled
	case errors.Is(err, domain.ErrPhotoStudentForbidden):
		return peopledirectory.ErrPhotoStudentForbidden
	case errors.Is(err, domain.ErrPhotoConsentWithdrawn):
		return peopledirectory.ErrPhotoConsentWithdrawn
	case errors.Is(err, domain.ErrPhotoConsentRequired):
		return peopledirectory.ErrPhotoConsentRequired
	case errors.Is(err, domain.ErrPhotoFilenameMismatch):
		return peopledirectory.ErrPhotoFilenameMismatch
	case errors.Is(err, domain.ErrPhotoNotSet):
		return peopledirectory.ErrPhotoNotSet
	case errors.Is(err, domain.ErrPhotoNoTenant):
		return peopledirectory.ErrPhotoNoTenant
	default:
		return mapError(err)
	}
}
