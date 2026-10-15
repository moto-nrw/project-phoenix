package api

import (
	"context"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// newPartialAbsencePreview binds the Timetable partial-absence preview to the
// People Directory (is the child still enrolled) and to the Care Plan pickup
// exceptions.
func newPartialAbsencePreview(reads timetableCompose.PresenceReads, students peopledirectory.StudentQuery, exceptions carePlanCompose.ExceptionQueries) timetableCompose.PartialAbsencePreview {
	enrolled := func(ctx context.Context, studentID int64) (bool, error) {
		values, err := students.ListStudentsByID(ctx, []int64{studentID})
		if err != nil {
			return false, err
		}
		for _, student := range values {
			if student.ID == studentID && !student.IsAlumnus() {
				return true, nil
			}
		}
		return false, nil
	}
	return timetableCompose.NewPartialAbsencePreview(reads, enrolled, pickupExceptionDirectory{query: exceptions})
}
