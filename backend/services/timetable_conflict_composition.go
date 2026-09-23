package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// TimetableConflictReaders are the retained readers the Timetable conflict
// detection and staffing capability reads (#3550). The production factory
// and the test compositions fill them from their own repository sets.
// ArrivalBaselines is optional (the exception-conflict read fails without
// it); Shifts, Staff, Exceptions, Schedules, CalendarPeriods and
// ArrivalExceptions are needed by NewTimetableConflictDetection only.
type TimetableConflictReaders struct {
	Instances         timetableCompose.ConflictInstances
	InstanceStaff     timetableCompose.ConflictInstanceStaff
	InstanceStudents  timetableCompose.ConflictInstanceStudents
	Exceptions        timetableCompose.ConflictExceptions
	Schedules         timetableCompose.ConflictSchedules
	Staff             timetableCompose.ConflictStaffDirectory
	CalendarPeriods   timetableCompose.ConflictCalendarPeriods
	ArrivalExceptions timetableCompose.ConflictArrivalExceptions
	Sessions          timetableCompose.ConflictSessions
	Shifts            timetableCompose.ConflictShifts
	Presence          timetableCompose.ConflictPresence
	ArrivalBaselines  careplan.ArrivalBaselineReader
	Logger            *slog.Logger
}

// NewTimetableConflictDetection composes the Timetable owner's conflict
// detection and staffing capability over the retained readers. It binds the
// two collaborators the owner may not name itself: Security Runtime's content
// fingerprint for the persisted conflict fingerprints and Care Plan's
// arrival baseline for the exception conflicts.
func NewTimetableConflictDetection(readers TimetableConflictReaders) (timetable.ConflictDetectionCapability, error) {
	return timetableCompose.NewConflictDetection(readers.dependencies())
}

// NewTimetableStartConflicts composes only the owner's start check, for the
// compositions that start blocks but serve no planning reads.
func NewTimetableStartConflicts(readers TimetableConflictReaders) (timetable.StartConflictQuery, error) {
	return timetableCompose.NewStartConflicts(readers.dependencies())
}

func (r TimetableConflictReaders) dependencies() timetableCompose.ConflictDetectionDependencies {
	deps := timetableCompose.ConflictDetectionDependencies{
		Instances:         r.Instances,
		InstanceStaff:     r.InstanceStaff,
		InstanceStudents:  r.InstanceStudents,
		Exceptions:        r.Exceptions,
		Schedules:         r.Schedules,
		Shifts:            r.Shifts,
		Staff:             r.Staff,
		CalendarPeriods:   r.CalendarPeriods,
		ArrivalExceptions: r.ArrivalExceptions,
		Presence:          r.Presence,
		Sessions:          r.Sessions,
		ContentHash:       securityruntime.Fingerprint,
		Logger:            r.Logger,
	}
	if r.ArrivalBaselines != nil {
		deps.ArrivalBaselines = arrivalBaselineProjector{reader: r.ArrivalBaselines}
	}
	return deps
}

// arrivalBaselineProjector serves the Timetable conflict detection's arrival
// port from Care Plan's baseline projection.
type arrivalBaselineProjector struct {
	reader careplan.ArrivalBaselineReader
}

func (p arrivalBaselineProjector) ProjectArrivals(ctx context.Context, studentIDs []int64, from, to timezone.Date) (timetableCompose.ArrivalBaselines, error) {
	projection, err := p.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return arrivalBaselineProjection{projection: projection}, nil
}

type arrivalBaselineProjection struct {
	projection *careplan.ArrivalBaselineProjection
}

func (p arrivalBaselineProjection) ExpectedArrival(studentID int64, date timezone.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.ExpectedArrival, true
}
