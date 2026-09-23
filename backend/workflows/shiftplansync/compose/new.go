// Package compose binds the shift-plan-sync workflow to the owner
// capabilities and the retained timetable planning services.
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync/ports"
)

// SickCascadeDependencies are the surfaces the #1843 cascade writes across:
// Workforce's Dienstplan on one side, the retained Betreuungsplan planning
// services on the other.
type SickCascadeDependencies struct {
	// Planning is the shift write that rebuilds a cancelled shift's cover set
	// atomically; Workforce serves the rows and the provenance stamp.
	Planning        workforce.StaffShiftPlanning
	Workforce       workforce.Capability
	LockStaffShifts func(ctx context.Context, staffID int64) error

	Instances     timetableplanning.InstanceService
	TimetableData *timetableplanning.TimetableDataService
	InstanceStaff scheduleModels.InstanceStaffRepository

	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	// Today is the calendar day the cascade's past-day guards compare
	// against; nil means timezone.TodayDate.
	Today func() timezone.Date
}

// NewSickCascade binds the cascade Workforce's absence lifecycle calls. Every
// dependency but the optional ones below is required: the cascade is
// fail-closed, so a half-wired composition must fail here instead of letting a
// sick report commit without its plan effects.
func NewSickCascade(deps SickCascadeDependencies) (shiftplansync.SickCascade, error) {
	if missing := missingNames(map[string]bool{
		"Planning":        deps.Planning != nil,
		"Workforce":       deps.Workforce != nil,
		"LockStaffShifts": deps.LockStaffShifts != nil,
		"Instances":       deps.Instances != nil,
		"TimetableData":   deps.TimetableData != nil,
		"InstanceStaff":   deps.InstanceStaff != nil,
	}); len(missing) > 0 {
		return nil, fmt.Errorf("shift plan sync compose: sick cascade needs %s", strings.Join(missing, ", "))
	}
	return application.NewSickCascade(application.SickCascadeDependencies{
		Shifts: shifts{
			planning: deps.Planning,
			rows:     deps.Workforce,
			lock:     deps.LockStaffShifts,
		},
		Instances:     deps.Instances,
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
	// Instances applies the deviations; ActivityInstances, InstanceStaff and
	// Staff serve the overview the planner reads.
	Instances         timetableplanning.InstanceService
	ActivityInstances scheduleModels.ActivityInstanceRepository
	InstanceStaff     scheduleModels.InstanceStaffRepository
	Staff             usersModels.StaffRepository

	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

// NewSubstitution binds the schedule half of the substitution module
// (services/education stays the port owner until #2742).
func NewSubstitution(deps SubstitutionDependencies) (shiftplansync.ScheduleSubstitution, error) {
	if missing := missingNames(map[string]bool{
		"Instances":         deps.Instances != nil,
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
		Engine:        deps.Instances,
		Broadcaster:   deps.Broadcaster,
		Logger:        deps.Logger,
	}), nil
}

// errSickCascadeUnbound keeps the cascade fail-closed when the composition
// never bound the workflow: a sick report must not commit half-applied.
var errSickCascadeUnbound = errors.New("shift plan sync: schedule cascade is not bound")

// DeferredSickCascade resolves the cascade on every call, because the root
// builds the absence service long before the schedule side exists. A nil
// result is fail-closed: every method returns an error, never a silent no-op.
func DeferredSickCascade(resolve func() shiftplansync.SickCascade) shiftplansync.SickCascade {
	return deferredSickCascade{resolve: resolve}
}

type deferredSickCascade struct {
	resolve func() shiftplansync.SickCascade
}

func (d deferredSickCascade) cascade() (shiftplansync.SickCascade, error) {
	if d.resolve == nil {
		return nil, errSickCascadeUnbound
	}
	cascade := d.resolve()
	if cascade == nil {
		return nil, errSickCascadeUnbound
	}
	return cascade, nil
}

func (d deferredSickCascade) MarkSickForRange(ctx context.Context, in workforce.SickCascadeInput) error {
	cascade, err := d.cascade()
	if err != nil {
		return err
	}
	return cascade.MarkSickForRange(ctx, in)
}

func (d deferredSickCascade) ClearSickForRange(ctx context.Context, in workforce.SickCascadeInput) error {
	cascade, err := d.cascade()
	if err != nil {
		return err
	}
	return cascade.ClearSickForRange(ctx, in)
}

func (d deferredSickCascade) ReconcileSickRange(ctx context.Context, before, after workforce.SickCascadeInput) error {
	cascade, err := d.cascade()
	if err != nil {
		return err
	}
	return cascade.ReconcileSickRange(ctx, before, after)
}

func (d deferredSickCascade) ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error {
	cascade, err := d.cascade()
	if err != nil {
		return err
	}
	return cascade.ReassignSickStamps(ctx, fromAbsenceID, toAbsenceID)
}

// missingNames lists the absent dependencies in a stable order.
func missingNames(present map[string]bool) []string {
	missing := make([]string, 0, len(present))
	for name, ok := range present {
		if !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

// shifts binds the workflow's Dienstplan port to the Workforce owner: the
// cancellation to its planning capability, the listings and the sick stamp to
// its row capability, and the write lock to the per-staff advisory lock the
// module's own writes take.
type shifts struct {
	planning workforce.StaffShiftPlanning
	rows     workforce.Capability
	lock     func(context.Context, int64) error
}

var (
	_ ports.Shifts                       = shifts{}
	_ shiftplansync.ScheduleSubstitution = (*application.SubstitutionAdapter)(nil)
)

func (s shifts) ListStaffShifts(ctx context.Context, filter workforce.StaffShiftFilter) ([]workforce.StaffShift, error) {
	return s.rows.ListStaffShifts(ctx, filter)
}

func (s shifts) ApplyCancellation(ctx context.Context, input workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error) {
	return s.planning.ApplyCancellation(ctx, input)
}

func (s shifts) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error) {
	return s.rows.SetStaffShiftSickAbsence(ctx, shiftID, absenceID)
}

func (s shifts) LockStaffShifts(ctx context.Context, staffID int64) error {
	return s.lock(ctx, staffID)
}
