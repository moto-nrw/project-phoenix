package repositories

import (
	"context"
	"fmt"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// NewPartialAbsencePreview binds the Timetable partial-absence preview to the
// People Directory (is the child still enrolled) and to the Care Plan pickup
// exceptions of the database.
func NewPartialAbsencePreview(db *bun.DB) timetableCompose.PartialAbsencePreview {
	exceptions, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("partial absence preview: compose care plan exception queries: %v", err))
	}
	students := peopleDirectoryReads(db)
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
	return timetableCompose.NewPartialAbsencePreview(mustPresenceReads(db), enrolled, pickupExceptionDirectory{query: exceptions})
}
