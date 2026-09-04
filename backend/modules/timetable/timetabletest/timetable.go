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

func New(tb TB, db *bun.DB) timetable.Capability {
	tb.Helper()
	capability, err := timetableCompose.New(timetableCompose.Dependencies{
		DB: db,
		Students: timetableCompose.StudentDirectoryFunc(func(context.Context) ([]timetableCompose.TargetStudent, error) {
			return []timetableCompose.TargetStudent{}, nil
		}),
		Observe: func(timetableCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test Timetable & Activities: %v", err)
	}
	return capability
}
