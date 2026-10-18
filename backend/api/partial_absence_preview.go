package api

import (
	"context"

	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// newPartialAbsencePreview binds the Timetable partial-absence preview to the
// People Directory (is the child still enrolled) and to the Care Plan pickup
// exceptions.
func newPartialAbsencePreview(reads timetableCompose.PresenceReads, students peopledirectory.StudentQuery, exceptions carePlanPorts.ExceptionQueries) timetableCompose.PartialAbsencePreview {
	enrolled := func(ctx context.Context, studentID int64) (bool, error) {
		return peopledirectory.StudentStillEnrolled(ctx, students, studentID)
	}
	return timetableCompose.NewPartialAbsencePreview(reads, enrolled, pickupExceptionDirectory{query: exceptions})
}
