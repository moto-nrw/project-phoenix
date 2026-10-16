package compose

import (
	"context"
	"errors"
	"time"

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
