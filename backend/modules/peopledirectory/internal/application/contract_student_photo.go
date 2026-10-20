package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

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
