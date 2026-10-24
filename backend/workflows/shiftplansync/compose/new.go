// Package compose binds the shift-plan-sync workflow to the owner
// capabilities.
package compose

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync/ports"
)

// SickCascadeDependencies are the surfaces the #1843 cascade writes across:
// Workforce's Dienstplan on one side, the Timetable owner's deviation writes
// on the other.
// TimetableRows are the Betreuungsplan rows and the day lock the sick
// cascade reads through; the composition root binds them.
type TimetableRows = application.TimetableRows

// SickStamps are the sick-report provenance stamps on block assignments the
// cascade lists and releases; the composition root binds them.
type SickStamps = application.SickStamps

// SickCascade is the #1843 cascade the sick-report entry applies.
type SickCascade = application.SickCascade

// ScheduleInstances and ScheduleInstanceStaff are the appointment and
// assignment reads of a Terminvertretung, in the Timetable owner's public
// vocabulary; the composition root binds them.
type (
	ScheduleInstances     = application.ScheduleInstances
	ScheduleInstanceStaff = application.ScheduleInstanceStaff
)

type SickCascadeDependencies struct {
	// Planning is the shift write that rebuilds a cancelled shift's cover set
	// atomically; Workforce serves the rows and the provenance stamp.
	Planning        workforce.StaffShiftPlanning
	Workforce       workforce.Capability
	LockStaffShifts func(ctx context.Context, staffID int64) error

	// Deviations are the Timetable owner's sick-report writes
	// (timetable.StaffDeviations satisfies them).
	Deviations    application.SickDeviations
	TimetableData TimetableRows
	InstanceStaff SickStamps

	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	// Today is the calendar day the cascade's past-day guards compare
	// against; nil means timezone.TodayDate.
	Today func() calendar.Date
}

// NewSickCascade binds the cascade the sick-report entry applies. Every
// dependency but the optional ones below is required: the cascade is
// fail-closed, so a half-wired composition must fail here instead of letting a
// sick report commit without its plan effects.
func NewSickCascade(deps SickCascadeDependencies) (SickCascade, error) {
	if missing := application.MissingNames(map[string]bool{
		"Planning":        deps.Planning != nil,
		"Workforce":       deps.Workforce != nil,
		"LockStaffShifts": deps.LockStaffShifts != nil,
		"Deviations":      deps.Deviations != nil,
		"TimetableData":   deps.TimetableData != nil,
		"InstanceStaff":   deps.InstanceStaff != nil,
	}); len(missing) > 0 {
		return nil, fmt.Errorf("shift plan sync compose: sick cascade needs %s", strings.Join(missing, ", "))
	}
	return application.NewSickCascade(application.SickCascadeDependencies{
		Shifts: application.Shifts{StaffShiftPlanning: deps.Planning, Lock: deps.LockStaffShifts,
			ShiftQuery: deps.Workforce, StaffShiftCommand: deps.Workforce},
		Deviations:    deps.Deviations,
		TimetableData: deps.TimetableData,
		InstanceStaff: deps.InstanceStaff,
		Broadcaster:   deps.Broadcaster,
		Logger:        deps.Logger,
		Today:         deps.Today,
	}), nil
}

// NewSickReports binds the entry of the absence writes that may cascade a
// sick report: Workforce's sick-report writes and the cascade, run in one
// tenant transaction (#4825).
func NewSickReports(absences workforce.SickReportWrites, cascade SickCascade) (shiftplansync.SickReports, error) {
	if absences == nil || cascade == nil {
		return nil, fmt.Errorf("shift plan sync compose: sick reports need the absence writes and the cascade")
	}
	return application.NewSickReports(absences, cascade), nil
}

// SubstitutionDependencies are the Betreuungsplan surfaces a Terminvertretung
// reads and writes, and the request's tenant transaction.
type SubstitutionDependencies struct {
	// Deviations applies the Timetable owner's deviation writes
	// (timetable.StaffDeviations); ActivityInstances, InstanceStaff and Staff
	// serve the overview the planner reads.
	Deviations        application.ScheduleDeviations
	ActivityInstances ScheduleInstances
	InstanceStaff     ScheduleInstanceStaff
	Staff             application.ScheduleStaff

	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	// Runtime is the request's tenant transaction the writes join.
	Runtime application.ScheduleRuntime
}

// NewSubstitution binds the Terminvertretung entry of /api/substitutions
// (#4825). It serves the schedule target in School Structure's substitution
// vocabulary; the root routes that target here, the module serves the group
// targets.
func NewSubstitution(deps SubstitutionDependencies) (shiftplansync.ScheduleSubstitution, error) {
	if missing := application.MissingNames(map[string]bool{
		"Deviations":        deps.Deviations != nil,
		"ActivityInstances": deps.ActivityInstances != nil,
		"InstanceStaff":     deps.InstanceStaff != nil,
		"Staff":             deps.Staff != nil,
		"Runtime":           deps.Runtime != nil,
	}); len(missing) > 0 {
		return nil, fmt.Errorf("shift plan sync compose: schedule substitution needs %s", strings.Join(missing, ", "))
	}
	return application.NewScheduleSubstitutions(application.NewSubstitutionAdapter(application.SubstitutionAdapterDependencies{
		Instances:     deps.ActivityInstances,
		InstanceStaff: deps.InstanceStaff,
		Staff:         deps.Staff,
		Engine:        deps.Deviations,
		Broadcaster:   deps.Broadcaster,
		Logger:        deps.Logger,
	}), deps.Runtime), nil
}

var (
	_ ports.Shifts                       = application.Shifts{}
	_ shiftplansync.ScheduleSubstitution = (*application.ScheduleSubstitutions)(nil)
)
