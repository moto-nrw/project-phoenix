package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e Engine) CurrentStudentConsents(
	ctx context.Context,
	snapshot peopledirectory.StudentConsentSnapshot,
	canManagePhoto bool,
) ([]peopledirectory.StudentConsentState, error) {
	values, err := e.studentConsents.CurrentStates(ctx, domain.StudentConsentSnapshot(snapshot), canManagePhoto)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentConsentState, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentConsentState(value))
	}
	return result, nil
}
