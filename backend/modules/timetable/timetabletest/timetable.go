// Package timetabletest composes the Timetable & Activities owner for tests.
package timetabletest

import (
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
		DB: db, Observe: func(timetableCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test Timetable & Activities: %v", err)
	}
	return capability
}
