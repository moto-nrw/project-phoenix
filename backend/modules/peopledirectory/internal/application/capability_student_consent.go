package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) CurrentStudentConsents(
	ctx context.Context,
	snapshot peopledirectory.StudentConsentSnapshot,
	canManagePhoto bool,
) ([]peopledirectory.StudentConsentState, error) {
	if snapshot.StudentID <= 0 {
		return nil, invalidStudent("a persisted student is required")
	}

	values, err := s.studentConsents.CurrentStates(ctx, domain.StudentConsentSnapshot(snapshot), canManagePhoto)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentConsentState, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentConsentState(value))
	}
	return result, nil
}
