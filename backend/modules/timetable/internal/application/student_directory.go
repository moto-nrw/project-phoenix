package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type studentDirectory struct{ query ports.StudentDirectory }

func (d studentDirectory) ListEnrolledStudents(ctx context.Context) ([]domain.TargetStudent, domain.OperationStats, error) {
	started := time.Now()
	students, err := d.query.ListEnrolledStudents(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.TargetStudent, 0, len(students))
	for _, student := range students {
		result = append(result, domain.TargetStudent{
			ID: student.ID, SchoolClass: student.SchoolClass, EducationGroupID: student.EducationGroupID,
			EnrolledUntil: student.EnrolledUntil,
		})
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func NewStudentDirectory(query ports.StudentDirectory) studentDirectory {
	return studentDirectory{query: query}
}
