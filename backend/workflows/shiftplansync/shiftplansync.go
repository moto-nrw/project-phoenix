// Package shiftplansync is the application workflow for the two writes that
// cross Workforce and Timetable & Activities inside one tenant transaction:
// the #1843 sick cascade, which fans a sick report out into shift
// cancellations (Dienstplan, schedule.staff_shifts) and per-day block
// absences (Betreuungsplan, schedule.instance_staff) and reverses exactly the
// rows it stamped, and the schedule substitution moves, which pull a person
// from their blocks and put a stand-in there.
//
// Neither write has a single owner: the sick cascade is triggered by
// Workforce's absence lifecycle but touches Timetable's staffing rows, and a
// substitution is planned against Timetable's blocks but reads Workforce's
// people. Both are therefore composed here, behind the owners' public
// contracts, and every method joins the caller's tenant transaction. The sick
// cascade is FAIL-CLOSED: the linkage is the feature, so an error must abort
// the surrounding absence write.
package shiftplansync

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// SickCascade is the #1843 cascade. Workforce declares the port its absence
// lifecycle calls; this workflow implements it.
type SickCascade = workforce.ShiftPlanSync

// ScheduleSubstitution is the schedule half of the substitution module.
// services/education stays the port owner until #2742.
type ScheduleSubstitution = education.ScheduleAdapter
