package ports

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// GroupRows are the owner's activity group and target reads and writes.
type GroupRows interface {
	timetable.ActivityGroupQuery
	timetable.ActivityGroupCommand
}
