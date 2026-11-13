package api

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	careStudentsHTTP "github.com/moto-ogs/project-phoenix/modules/careplan/http"
	importPorts "github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
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

// importCohorts is the student import's Klasse/Gruppe follow-up (#6951):
// the gates an update attempt takes before its first row and the follow-up
// after its last. The zero value skips it.
type importCohorts struct {
	Lock   func(context.Context) error
	Follow func(context.Context, []importPorts.CohortChange, calendar.Date) error
}

// newImportCohorts binds the import's follow-up to Care Plan's gates for
// offering-derived writes and its offering resync, and to Timetable's cohort
// rosters.
func newImportCohorts(bookings careplan.BookingMaterializationCapability, rosters timetable.CohortRosters) importCohorts {
	follow := importCohortChanges(rosters)
	return importCohorts{
		Lock: bookings.LockOfferingDerivedWrites,
		Follow: func(ctx context.Context, changes []importPorts.CohortChange, from calendar.Date) error {
			for _, change := range changes {
				if change.SchoolClassChanged {
					if err := bookings.ResyncOfferingSourcedTemplates(ctx, from); err != nil {
						return err
					}
					break
				}
			}
			return follow(ctx, changes, from)
		},
	}
}

// importCohortChanges binds the import's change records to the owner's
// contract.
func importCohortChanges(rosters timetable.CohortRosters) func(context.Context, []importPorts.CohortChange, calendar.Date) error {
	return func(ctx context.Context, changes []importPorts.CohortChange, from calendar.Date) error {
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
