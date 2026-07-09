package users

import (
	"context"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// CaregiverDirectory exposes the canonical operational caregiver lookup.
// It is intentionally separate from PersonService so existing mocks do not
// need to implement these methods unless a test actually exercises them.
type CaregiverDirectory interface {
	ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error)
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error)
}

func (s *personService) ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error) {
	caregivers, err := s.TeacherRepo.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, &UsersError{Op: "list active caregivers", Err: err}
	}
	return caregivers, nil
}

func (s *personService) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error) {
	caregiver, err := s.TeacherRepo.FindActiveCaregiverByAccountID(ctx, accountID)
	if err != nil {
		return nil, &UsersError{Op: "find active caregiver by account ID", Err: err}
	}
	return caregiver, nil
}

func CaregiverDirectoryFromPersonService(personService PersonService) (CaregiverDirectory, error) {
	directory, ok := personService.(CaregiverDirectory)
	if !ok {
		return nil, fmt.Errorf("person service does not implement caregiver directory")
	}
	return directory, nil
}
