package api

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// NewCleanupTimetable composes the unobserved Timetable owner for CLI roots.
// The serving root uses composeModuleServices so it can attach metrics.
func NewCleanupTimetable(db *bun.DB) (timetable.Capability, error) {
	students, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	if err != nil {
		return nil, err
	}
	return timetableCompose.New(timetableCompose.Dependencies{
		DB: db, Students: timetableStudents(students), Observe: func(timetableCompose.Observation) {},
	})
}
