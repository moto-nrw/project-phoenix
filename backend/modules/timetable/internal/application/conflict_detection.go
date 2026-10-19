package application

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Conflict detection and staffing (#3424 slice S4, #3550): the start and
// planning conflict checks (WP-B9, #2139), the exception-conflict warnings
// (WP-B13), the staff availability pool (#1884) and the shift-coverage probe
// (#1873) behind the public timetable.ConflictDetectionCapability. The
// detection still reads the retained repository rows, whose status carries
// the Student Presence session state the owner's own rows do not, so the
// readers below are the retained repository shapes the composition root
// binds.

type conflictDetection struct {
	deps   ports.ConflictDetectionDependencies
	logger *slog.Logger
}

// NewConflictDetection composes the conflict detection and staffing
// capability. Every reader except ArrivalBaselines is required: the checks
// cannot decide whether to warn without them.
func NewConflictDetection(deps ports.ConflictDetectionDependencies) (timetable.ConflictDetectionCapability, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil || deps.InstanceStudents == nil || deps.Exceptions == nil ||
		deps.Schedules == nil || deps.Shifts == nil || deps.Staff == nil || deps.CalendarPeriods == nil ||
		deps.ArrivalExceptions == nil || deps.Presence == nil || deps.Sessions == nil || deps.ContentHash == nil {
		return nil, errors.New("timetable conflict detection: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &conflictCommands{&conflictDetection{deps: deps, logger: logger}}, nil
}

// germanDateLayout renders calendar dates as dd.mm.yyyy in user-facing copy.
const germanDateLayout = "02.01.2006"

// isoWeekday returns the ISO weekday (Monday=1 … Sunday=7) of the date.
func isoWeekday(date calendar.Date) int {
	weekday := date.Weekday()
	if weekday == time.Sunday {
		return 7
	}
	return int(weekday)
}

// clockWindowsOverlap reports whether two wall-clock windows intersect;
// touching boundaries do not overlap (same rule as StaffShift.Overlaps).
func clockWindowsOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	as, ae := calendar.NormalizeWallClock(aStart), calendar.NormalizeWallClock(aEnd)
	bs, be := calendar.NormalizeWallClock(bStart), calendar.NormalizeWallClock(bEnd)
	return as.Before(be) && bs.Before(ae)
}

// IsPlannableInstance keeps the blocks that still occupy their people:
// planned or running ones.
func IsPlannableInstance(instance *scheduleModels.ActivityInstance) bool {
	return instance.Status == scheduleModels.InstanceStatusPlanned ||
		instance.Status == scheduleModels.InstanceStatusActive
}

// shiftWindowsOf maps Dienstplan rows onto the public interval vocabulary.
func shiftWindowsOf(shifts []*scheduleModels.StaffShift) []timetable.ShiftWindow {
	windows := make([]timetable.ShiftWindow, 0, len(shifts))
	for _, shift := range shifts {
		if shift != nil {
			windows = append(windows, timetable.ShiftWindow{StartTime: shift.StartTime, EndTime: shift.EndTime, Cancelled: shift.Cancelled})
		}
	}
	return windows
}

type conflictCommands struct{ *conflictDetection }
