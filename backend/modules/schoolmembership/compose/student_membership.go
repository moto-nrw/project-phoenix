package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (e engine) EnrollStudent(ctx context.Context, input schoolmembership.StudentEnrollment) (int64, error) {
	id, err := e.service.EnrollStudent(ctx, domain.StudentEnrollment(input))
	return id, mapError(err)
}

func (e engine) RenewStudentEnrollment(ctx context.Context, input schoolmembership.StudentEnrollment) (int64, error) {
	id, err := e.service.RenewStudentEnrollment(ctx, domain.StudentEnrollment(input))
	return id, mapError(err)
}

func (e engine) AssignStudentGroup(ctx context.Context, studentID int64, groupID *int64) (bool, error) {
	return e.service.AssignStudentGroup(ctx, studentID, groupID)
}
