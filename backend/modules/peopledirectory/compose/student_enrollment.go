package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e engine) ReadEnrollmentStudent(ctx context.Context, id int64, lock string) (peopledirectory.EnrollmentRecord, error) {
	value, err := e.students.ReadEnrollment(ctx, id, lock)
	return peopledirectory.EnrollmentRecord(value), mapError(err)
}

func (e engine) LockEnrollmentClassWrites(ctx context.Context) error {
	return mapError(e.students.LockEnrollmentClassWrites(ctx))
}

func (e engine) ApplyEnrollmentProfile(ctx context.Context, id int64, input peopledirectory.EnrollmentProfilePatch) error {
	return mapError(e.students.ApplyEnrollmentProfile(ctx, id, domain.EnrollmentProfilePatch(input)))
}

func (e engine) CreateEnrollmentStudent(ctx context.Context, input peopledirectory.EnrollmentStudent) (peopledirectory.CreatedEnrollmentStudent, error) {
	value, err := e.students.CreateEnrollment(ctx, domain.EnrollmentStudent(input))
	if err != nil {
		return peopledirectory.CreatedEnrollmentStudent{}, mapError(err)
	}
	return peopledirectory.CreatedEnrollmentStudent{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
		PersonID: value.PersonID, SchoolClass: value.SchoolClass, Status: value.Status,
		EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil,
	}, nil
}

func (e engine) RenewEnrollmentStudent(ctx context.Context, id int64, input peopledirectory.EnrollmentStudent) error {
	return mapError(e.students.RenewEnrollment(ctx, id, domain.EnrollmentStudent(input)))
}
