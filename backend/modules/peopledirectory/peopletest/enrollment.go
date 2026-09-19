// Package peopletest composes the People Directory owner for behavior tests.
package peopletest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

type Student struct {
	ID            int64
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

type StudentQuery interface {
	ListEnrolledStudents(context.Context) ([]Student, error)
}

type studentQuery func(context.Context) ([]Student, error)

func (q studentQuery) ListEnrolledStudents(ctx context.Context) ([]Student, error) { return q(ctx) }

func NewStudentQuery(db *bun.DB) (StudentQuery, error) {
	owner, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	if err != nil {
		return nil, err
	}
	return studentQuery(func(ctx context.Context) ([]Student, error) {
		rows, err := owner.ListEnrolledStudents(ctx)
		if err != nil {
			return nil, err
		}
		students := make([]Student, 0, len(rows))
		for _, row := range rows {
			students = append(students, Student{ID: row.ID, Status: row.Status, EnrolledFrom: row.EnrolledFrom, EnrolledUntil: row.EnrolledUntil})
		}
		return students, nil
	}), nil
}
