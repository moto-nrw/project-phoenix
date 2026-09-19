package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentConsentHistory is the Audit Platform seam behind the shared consent
// projection. The composition root binds it: Audit Platform owns the consent
// trail, People Directory only folds it together with the live timestamps.
type StudentConsentHistory interface {
	// LatestPhotoWithdrawal returns when the child's photo consent was last
	// withdrawn, or nil when the latest recorded photo event is not a
	// withdrawal.
	LatestPhotoWithdrawal(ctx context.Context, studentID int64) (*time.Time, error)
}

func (e engine) CurrentStudentConsents(
	ctx context.Context,
	snapshot peopledirectory.StudentConsentSnapshot,
	canManagePhoto bool,
) ([]peopledirectory.StudentConsentState, error) {
	values, err := e.studentConsents.CurrentStates(ctx, domain.StudentConsentSnapshot(snapshot), canManagePhoto)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]peopledirectory.StudentConsentState, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentConsentState(value))
	}
	return result, nil
}
