package repositories

import (
	"context"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type parentStudentDirectory struct{ students peopledirectory.StudentQuery }

func (d parentStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]parentRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]parentRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, parentRepo.DirectoryStudent{
			ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID,
			SchoolClass: student.SchoolClass, Status: student.Status,
			EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
		})
	}
	return result, nil
}
