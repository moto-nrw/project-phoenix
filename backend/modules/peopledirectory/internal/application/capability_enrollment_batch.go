package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The data import binds the enrollment batch slice outside Owner (#5552).
var _ peopledirectory.EnrollmentBatches = (*Service)(nil)

// normalizedEnrollment validates and normalizes one input as
// CreateEnrollmentStudent does.
func normalizedEnrollment(input peopledirectory.EnrollmentStudent) (peopledirectory.EnrollmentStudent, error) {
	if input.PersonID <= 0 {
		return input, &peopledirectory.InvalidStudentError{Reason: "person ID is required"}
	}
	if err := normalizeEnrollmentStudent(&input); err != nil {
		return input, err
	}
	if input.InitialProfile != nil {
		profile := *input.InitialProfile
		if err := normalizeEnrollmentDeparture(&profile); err != nil {
			return input, err
		}
		input.InitialProfile = &profile
	}
	return input, nil
}

func createdEnrollmentStudent(value peopledirectory.Student) peopledirectory.CreatedEnrollmentStudent {
	return peopledirectory.CreatedEnrollmentStudent{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
		PersonID: value.PersonID, SchoolClass: value.SchoolClass, Status: value.Status,
		EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil,
	}
}

func studentOfCreated(created peopledirectory.CreatedEnrollmentStudent) peopledirectory.Student {
	return peopledirectory.Student{
		ID: created.ID, CreatedAt: created.CreatedAt, UpdatedAt: created.UpdatedAt, TenantID: created.TenantID,
		PersonID: created.PersonID, SchoolClass: created.SchoolClass, Status: created.Status,
		EnrolledFrom: created.EnrolledFrom, EnrolledUntil: created.EnrolledUntil,
	}
}

func (s *Service) CreateEnrollmentProfiles(ctx context.Context, inputs []peopledirectory.EnrollmentStudent) ([]peopledirectory.CreatedEnrollmentStudent, error) {
	if len(inputs) == 0 {
		return []peopledirectory.CreatedEnrollmentStudent{}, nil
	}
	normalized := make([]domain.EnrollmentStudent, len(inputs))
	for i, input := range inputs {
		value, err := normalizedEnrollment(input)
		if err != nil {
			return nil, err
		}
		normalized[i] = enrollmentInput(value)
	}
	values, err := s.students.CreateEnrollments(ctx, normalized)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.CreatedEnrollmentStudent, len(values))
	for i, value := range values {
		result[i] = createdEnrollmentStudent(value)
	}
	return result, nil
}

func (s *Service) EnrollCreatedStudent(ctx context.Context, created peopledirectory.CreatedEnrollmentStudent, input peopledirectory.EnrollmentStudent) (int64, error) {
	if created.ID <= 0 {
		return 0, &peopledirectory.InvalidStudentError{Reason: "student ID is required"}
	}
	value, err := normalizedEnrollment(input)
	if err != nil {
		return 0, err
	}
	membershipID, err := s.students.EnrollCreated(ctx, studentOfCreated(created), enrollmentInput(value))
	return membershipID, MapError(err)
}

func (s *Service) SaveEnrollmentCares(ctx context.Context, cares []peopledirectory.EnrollmentCare) error {
	if len(cares) == 0 {
		return nil
	}
	entries := make([]EnrollmentCareEntry, len(cares))
	for i, care := range cares {
		if care.MembershipID <= 0 || care.Student.ID <= 0 {
			return &peopledirectory.InvalidStudentError{Reason: "membership and student ID are required"}
		}
		value, err := normalizedEnrollment(care.Input)
		if err != nil {
			return err
		}
		entries[i] = EnrollmentCareEntry{MembershipID: care.MembershipID, Student: studentOfCreated(care.Student), Input: enrollmentInput(value)}
	}
	return MapError(s.students.SaveEnrollmentCares(ctx, entries))
}
