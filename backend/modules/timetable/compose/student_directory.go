package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

type TargetStudent struct {
	ID               int64
	SchoolClass      string
	EducationGroupID *int64
	EnrolledUntil    string
}

type StudentDirectory interface {
	ListEnrolledStudents(context.Context) ([]TargetStudent, error)
}

type StudentDirectoryFunc func(context.Context) ([]TargetStudent, error)

func (f StudentDirectoryFunc) ListEnrolledStudents(ctx context.Context) ([]TargetStudent, error) {
	return f(ctx)
}

type studentDirectory struct{ query StudentDirectory }

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
