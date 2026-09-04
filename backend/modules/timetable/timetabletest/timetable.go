// Package timetabletest composes the Timetable & Activities owner for tests.
package timetabletest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type TB interface {
	Helper()
	Fatalf(string, ...any)
}

type TargetStudent struct {
	ID               int64
	SchoolClass      string
	EducationGroupID *int64
	EnrolledUntil    string
}

type StudentDirectoryFunc func(context.Context) ([]TargetStudent, error)

func New(tb TB, db *bun.DB) timetable.Capability {
	return newModule(tb, db, timetableCompose.StudentDirectoryFunc(func(context.Context) ([]timetableCompose.TargetStudent, error) {
		return []timetableCompose.TargetStudent{}, nil
	}))
}

func NewWithStudentDirectory(tb TB, db *bun.DB, students StudentDirectoryFunc) timetable.Capability {
	if students == nil {
		tb.Fatalf("compose test Timetable & Activities: student directory is required")
	}
	return newModule(tb, db, timetableCompose.StudentDirectoryFunc(func(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
		values, err := students(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetableCompose.TargetStudent, 0, len(values))
		for _, value := range values {
			result = append(result, timetableCompose.TargetStudent(value))
		}
		return result, nil
	}))
}

func newModule(tb TB, db *bun.DB, students timetableCompose.StudentDirectory) timetable.Capability {
	tb.Helper()
	capability, err := timetableCompose.New(timetableCompose.Dependencies{
		DB:       db,
		Students: students,
		Observe:  func(timetableCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test Timetable & Activities: %v", err)
	}
	return capability
}
