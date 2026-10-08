package services

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// EducationTeachers serves the teacher directory of the retained School
// Structure group service (#2742) from the retained teacher repository: the
// teachers come back as the service's own view, with the name and login of
// the staff member's person.
type EducationTeachers struct{ teachers userModels.TeacherRepository }

// NewEducationTeachers binds the teacher directory to the teacher repository.
func NewEducationTeachers(teachers userModels.TeacherRepository) EducationTeachers {
	return EducationTeachers{teachers: teachers}
}

// FindTeacher fails when the teacher does not exist for the caller.
func (t EducationTeachers) FindTeacher(ctx context.Context, id int64) error {
	_, err := t.teachers.FindByID(ctx, id)
	return err
}

// ListTeachers returns the teachers with their staff member's person.
func (t EducationTeachers) ListTeachers(ctx context.Context, ids []int64) ([]*education.Teacher, error) {
	teachers, err := t.teachers.FindWithStaffAndPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*education.Teacher, 0, len(teachers))
	for _, teacher := range teachers {
		if teacher != nil {
			result = append(result, educationTeacher(teacher))
		}
	}
	return result, nil
}

func educationTeacher(teacher *userModels.Teacher) *education.Teacher {
	result := &education.Teacher{
		ID: teacher.ID, StaffID: teacher.StaffID,
		Specialization: teacher.Specialization, Role: teacher.Role, Qualifications: teacher.Qualifications,
		CreatedAt: teacher.CreatedAt, UpdatedAt: teacher.UpdatedAt,
	}
	if teacher.Staff == nil || teacher.Staff.Person == nil {
		return result
	}
	person := teacher.Staff.Person
	result.Person = &education.TeacherPerson{FirstName: person.FirstName, LastName: person.LastName}
	if person.Account != nil {
		result.Person.Email = person.Account.Email
	}
	return result
}
