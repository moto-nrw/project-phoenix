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

// NewSickCascade binds the cascade Workforce's absence lifecycle calls. Every
// dependency but the optional ones below is required: the cascade is
// fail-closed, so a half-wired composition must fail here instead of letting a
// sick report commit without its plan effects.
func NewSickCascade(deps SickCascadeDependencies) (shiftplansync.SickCascade, error) {
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

// SubstitutionDependencies are the Betreuungsplan surfaces a Terminvertretung
// reads and writes.
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
}

// NewSubstitution binds the schedule half of the substitution module
// (services/education stays the port owner until #2742).
func NewSubstitution(deps SubstitutionDependencies) (shiftplansync.ScheduleSubstitution, error) {
	if missing := application.MissingNames(map[string]bool{
		"Deviations":        deps.Deviations != nil,
		"ActivityInstances": deps.ActivityInstances != nil,
		"InstanceStaff":     deps.InstanceStaff != nil,
		"Staff":             deps.Staff != nil,
	}); len(missing) > 0 {
		return nil, fmt.Errorf("shift plan sync compose: schedule substitution needs %s", strings.Join(missing, ", "))
	}
	return application.NewSubstitutionAdapter(application.SubstitutionAdapterDependencies{
		Instances:     deps.ActivityInstances,
		InstanceStaff: deps.InstanceStaff,
		Staff:         deps.Staff,
		Engine:        deps.Deviations,
		Broadcaster:   deps.Broadcaster,
		Logger:        deps.Logger,
	}), nil
}

var (
	_ ports.Shifts                       = application.Shifts{}
	_ shiftplansync.ScheduleSubstitution = (*application.SubstitutionAdapter)(nil)
)
