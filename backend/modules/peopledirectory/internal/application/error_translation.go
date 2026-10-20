package application

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// MapError maps the owner errors onto the People Directory contract.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrStudentNotFound):
		return peopledirectory.ErrStudentNotFound
	case errors.Is(err, domain.ErrStudentLockBusy):
		return fmt.Errorf("%w: %w", peopledirectory.ErrStudentLockBusy, err)
	case errors.Is(err, domain.ErrCompanionWouldLoseDeparture):
		return peopledirectory.ErrCompanionWouldLoseDeparture
	case errors.Is(err, domain.ErrCompanionLockBusy):
		return peopledirectory.ErrCompanionLockBusy
	case errors.Is(err, domain.ErrStudentInvalid):
		// The reason is the message a handler renders, so it is kept.
		return &peopledirectory.InvalidStudentError{Reason: err.Error()}
	case errors.Is(err, domain.ErrFamilyProtectionUnchanged):
		return peopledirectory.ErrFamilyProtectionUnchanged
	case errors.Is(err, domain.ErrFamilyProtectionInvalid):
		return peopledirectory.ErrFamilyProtectionInvalid
	case errors.Is(err, domain.ErrStudentNoteNotFound):
		return peopledirectory.ErrStudentNoteNotFound
	case errors.Is(err, domain.ErrStudentNoteNotAuthor):
		return peopledirectory.ErrStudentNoteNotAuthor
	case errors.Is(err, domain.ErrStudentNoteImmutable):
		return peopledirectory.ErrStudentNoteImmutable
	case errors.Is(err, domain.ErrStudentNoteDeleteForbidden):
		return peopledirectory.ErrStudentNoteDeleteForbidden
	case errors.Is(err, domain.ErrStudentNoteInvalid):
		return peopledirectory.ErrStudentNoteInvalid
	default:
		return err
	}
}
