// Package shiftplansync is the application workflow for the two writes that
// cross Workforce and Timetable & Activities inside one tenant transaction:
// the #1843 sick cascade, which fans a sick report out into shift
// cancellations (Dienstplan, schedule.staff_shifts) and per-day block
// absences (Betreuungsplan, schedule.instance_staff) and reverses exactly the
// rows it stamped, and the schedule substitution moves, which pull a person
// from their blocks and put a stand-in there.
//
// Neither write has a single owner: the sick cascade follows Workforce's
// absence writes but touches Timetable's staffing rows, and a substitution is
// planned against Timetable's blocks but reads Workforce's people. Both are
// therefore entries of this workflow, behind the owners' public contracts:
// the root routes the absence writes and the schedule substitution target
// here, and no owner calls back into the workflow (#4825). The sick cascade
// is FAIL-CLOSED: the linkage is the feature, so an error must abort the
// surrounding absence write.
package shiftplansync

import (
	"context"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// SickReports is the entry of the absence writes that may cascade a sick
// report: Workforce's write and its returned plan run in one tenant
// transaction. The Workforce HTTP routes reach it through their absence
// write ports.
type SickReports interface {
	CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error
	DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error
}

// ScheduleSubstitution is the Terminvertretung entry of /api/substitutions in
// School Structure's substitution vocabulary: the root routes the schedule
// target to it (#4825).
type ScheduleSubstitution = education.SubstitutionModule
