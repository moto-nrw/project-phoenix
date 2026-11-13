package api

import (
	"context"

	careStudentsHTTP "github.com/moto-ogs/project-phoenix/modules/careplan/http"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/workflows/gradetransition"
)

// Timetable's cohort rosters (#6841) serve two consumer ports: the student
// edit's Klasse/Gruppe follow-up and the grade transition's class rewrite.
// These bindings translate their change records onto the owner's contract.

// studentCohortChanges binds the student edit's Klasse/Gruppe follow-up.
func studentCohortChanges(rosters timetable.CohortRosters) careStudentsHTTP.FollowCohortChanges {
	return func(ctx context.Context, changes []careStudentsHTTP.TargetCohortChange, from calendar.Date) error {
		values := make([]timetable.CohortChange, 0, len(changes))
		for _, change := range changes {
			values = append(values, timetable.CohortChange{
				StudentID: change.StudentID, SchoolClassChanged: change.SchoolClassChanged, PreviousSchoolClass: change.PreviousSchoolClass,
				GroupChanged: change.GroupChanged, PreviousGroupID: change.PreviousGroupID,
			})
		}
		return rosters.FollowCohortChanges(ctx, values, from)
	}
}

// gradeTransitionClassChanges binds the grade transition's class follow-up.
func gradeTransitionClassChanges(rosters timetable.CohortRosters) func(context.Context, []gradetransition.ClassChange, calendar.Date) error {
	return func(ctx context.Context, changes []gradetransition.ClassChange, from calendar.Date) error {
		values := make([]timetable.CohortChange, 0, len(changes))
		for _, change := range changes {
			values = append(values, timetable.CohortChange{
				StudentID: change.StudentID, SchoolClassChanged: true, PreviousSchoolClass: change.PreviousSchoolClass,
			})
		}
		return rosters.FollowCohortChanges(ctx, values, from)
	}
}
