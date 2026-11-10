package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type VisitAdmissionRead func(context.Context, int64, int64) (ports.VisitAdmission, ports.Stats, error)

// WithVisitAdmissionRead supplies the owner's combined admission read.
func WithVisitAdmissionRead(read VisitAdmissionRead) ServiceOption {
	return func(s *service) { s.visitAdmissionRead = read }
}

func (s *service) visitAdmissionOccupancy(ctx context.Context, studentID, groupID int64) (*int, error) {
	if s.visitAdmissionRead == nil || !s.isDeviceRequest(ctx) {
		// Web writes and bare unit graphs keep the existing separate checks.
		return nil, s.ensureStudentHasNoActiveVisit(ctx, studentID)
	}
	state, _, err := s.visitAdmissionRead(ctx, studentID, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}
	if state.HasOpenVisit {
		return nil, &ActiveError{Op: "CreateVisit", Err: ErrStudentAlreadyActive}
	}
	return &state.GroupOccupancy, nil
}
