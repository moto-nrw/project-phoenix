package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e Engine) FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error) {
	storedURL, err := e.studentPhotos.FindPhoto(ctx, studentID, filename)
	return storedURL, mapPhotoError(err)
}

func (e Engine) CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error {
	return mapPhotoError(e.studentPhotos.CommitPhoto(ctx, studentID, storedURL, consentAck))
}

func (e Engine) ClearStudentPhoto(ctx context.Context, studentID int64) (string, error) {
	clearedURL, err := e.studentPhotos.ClearPhoto(ctx, studentID)
	return clearedURL, mapPhotoError(err)
}

func (e Engine) PurgeStudentPhotos(ctx context.Context) ([]string, error) {
	urls, err := e.studentPhotos.PurgePhotos(ctx)
	return urls, mapPhotoError(err)
}

func (e Engine) ApplyStudentPhotoConsent(
	ctx context.Context,
	current peopledirectory.StudentPhotoState,
	requestedConsent *bool,
) (peopledirectory.StudentPhotoState, error) {
	updated, err := e.studentPhotos.ApplyPhotoConsent(ctx, domain.StudentPhoto{
		StudentID:           current.StudentID,
		PhotoPath:           current.PhotoPath,
		PhotoConsentGivenAt: current.PhotoConsentGivenAt,
		PhotoConsentGivenBy: current.PhotoConsentGivenBy,
	}, requestedConsent)
	if err != nil {
		return current, mapPhotoError(err)
	}
	return peopledirectory.StudentPhotoState{
		StudentID:           updated.StudentID,
		PhotoPath:           updated.PhotoPath,
		PhotoConsentGivenAt: updated.PhotoConsentGivenAt,
		PhotoConsentGivenBy: updated.PhotoConsentGivenBy,
	}, nil
}

func (e Engine) ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string) error {
	return mapPhotoError(e.studentPhotos.ScheduleUnlink(ctx, storedURL))
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
	default:
		return MapError(err)
	}
}
