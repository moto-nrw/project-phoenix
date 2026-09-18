package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// ErrStudentConsentHistoryUnavailable reports a directory composed without the
// consent trail. A withdrawn photo consent would otherwise read as "never
// granted", which is the one state staff must not be shown.
var ErrStudentConsentHistoryUnavailable = errors.New("people directory: student consent history is not configured")

// StudentConsentService serves the shared consent projection both portals
// render, so staff cannot see a different state than parents.
type StudentConsentService struct {
	history ports.StudentConsentHistory
	observe ports.Observer
}

func NewStudentConsents(history ports.StudentConsentHistory, observe ports.Observer) *StudentConsentService {
	if observe == nil {
		panic("people directory application: student consent observer is required")
	}
	return &StudentConsentService{history: history, observe: observe}
}

// CurrentStates resolves the four consent states. The trail is consulted only
// when the child holds no live photo consent — a granted consent is the
// current state by definition, so the extra read would be wasted.
func (s *StudentConsentService) CurrentStates(
	ctx context.Context,
	snapshot domain.StudentConsentSnapshot,
	canManagePhoto bool,
) (result []domain.StudentConsentState, err error) {
	err = s.run(ctx, "current_student_consents", func() error {
		var photoWithdrawnAt *time.Time
		if snapshot.PhotoConsentGivenAt == nil {
			if s.history == nil {
				return ErrStudentConsentHistoryUnavailable
			}
			photoWithdrawnAt, err = s.history.LatestPhotoWithdrawal(ctx, snapshot.StudentID)
			if err != nil {
				return err
			}
		}
		result = domain.CurrentStudentConsents(snapshot, canManagePhoto, photoWithdrawnAt)
		return nil
	})
	return result, err
}

func (s *StudentConsentService) run(ctx context.Context, operation string, fn func() error) error {
	return observeRun(ctx, s.observe, operation, passthroughRun, func(context.Context, *domain.OperationStats) error {
		return fn()
	})
}
