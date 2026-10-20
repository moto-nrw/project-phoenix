package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) ReadEnrollmentStudent(ctx context.Context, id int64, lock string) (peopledirectory.EnrollmentRecord, error) {
	if id <= 0 || (lock != "" && lock != "update" && lock != "nowait") {
		return peopledirectory.EnrollmentRecord{}, &peopledirectory.InvalidStudentError{Reason: "invalid enrollment student read"}
	}

	value, err := s.students.ReadEnrollment(ctx, id, lock)
	return value, MapError(err)
}

func (s *Service) LockEnrollmentClassWrites(ctx context.Context) error {
	return MapError(s.students.LockEnrollmentClassWrites(ctx))
}

func (s *Service) ApplyEnrollmentProfile(ctx context.Context, id int64, input peopledirectory.EnrollmentProfilePatch) error {
	if id <= 0 {
		return &peopledirectory.InvalidStudentError{Reason: "student ID is required"}
	}
	if err := normalizeEnrollmentDeparture(&input); err != nil {
		return err
	}

	return MapError(s.students.ApplyEnrollmentProfile(ctx, id, domain.EnrollmentProfilePatch(input)))
}

func (s *Service) CreateEnrollmentStudent(ctx context.Context, input peopledirectory.EnrollmentStudent) (peopledirectory.CreatedEnrollmentStudent, error) {
	if input.PersonID <= 0 {
		return peopledirectory.CreatedEnrollmentStudent{}, &peopledirectory.InvalidStudentError{Reason: "person ID is required"}
	}
	if err := normalizeEnrollmentStudent(&input); err != nil {
		return peopledirectory.CreatedEnrollmentStudent{}, err
	}
	if input.InitialProfile != nil {
		profile := *input.InitialProfile
		if err := normalizeEnrollmentDeparture(&profile); err != nil {
			return peopledirectory.CreatedEnrollmentStudent{}, err
		}
		input.InitialProfile = &profile
	}

	value, err := s.students.CreateEnrollment(ctx, enrollmentInput(input))
	if err != nil {
		return peopledirectory.CreatedEnrollmentStudent{}, MapError(err)
	}
	return peopledirectory.CreatedEnrollmentStudent{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
		PersonID: value.PersonID, SchoolClass: value.SchoolClass, Status: value.Status,
		EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil,
	}, nil
}

func (s *Service) RenewEnrollmentStudent(ctx context.Context, id int64, input peopledirectory.EnrollmentStudent) error {
	if input.InitialProfile != nil {
		return &peopledirectory.InvalidStudentError{Reason: "initial profile is only valid for creation"}
	}
	if id <= 0 {
		return &peopledirectory.InvalidStudentError{Reason: "student ID is required"}
	}
	if err := normalizeEnrollmentStudent(&input); err != nil {
		return err
	}

	return MapError(s.students.RenewEnrollment(ctx, id, enrollmentInput(input)))
}

func normalizeEnrollmentStudent(input *peopledirectory.EnrollmentStudent) error {
	if input.SchoolClass == "" {
		return &peopledirectory.InvalidStudentError{Reason: "school class is required"}
	}
	input.SchoolClass = strings.TrimSpace(input.SchoolClass)
	if err := peopledirectory.ValidateBirthday(input.EnrolledFrom); err != nil {
		return &peopledirectory.InvalidStudentError{Reason: "invalid enrolled_from calendar date"}
	}
	if err := peopledirectory.ValidateBirthday(input.EnrolledUntil); err != nil {
		return &peopledirectory.InvalidStudentError{Reason: "invalid enrolled_until calendar date"}
	}

	return nil
}
