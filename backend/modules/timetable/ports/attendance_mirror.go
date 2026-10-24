package ports

import (
	"context"
	"log/slog"

	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AttendanceMirrorDependencies wires the mirror; both repositories are
// required, the logger falls back to slog.Default.
type AttendanceMirrorDependencies struct {
	Instances    *MirrorInstances
	Participants *MirrorParticipants
	Logger       *slog.Logger
	// ReportPanic reports a recovered panic to error tracking; the
	// composition binds Sentry.
	ReportPanic func(ctx context.Context, recovered any)
}

// MirrorInstances resolves the planned block bridged to a live session.
type MirrorInstances struct {
	FindByActiveGroups  func(context.Context, []int64) ([]*timetable.ScheduledInstance, error)
	FindByActiveGroupID func(context.Context, int64) (*timetable.ScheduledInstance, error)
}

// MirrorParticipants reads and mutates the attendance slice used by the mirror.
type MirrorParticipants struct {
	FindByInstanceAndStudent          func(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error)
	FindCurrentCandidates             func(ctx context.Context, studentID int64, date calendar.Date, at time.Time) ([]*timetable.ScheduledParticipant, error)
	FindCurrentCandidatesByStudentIDs func(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time) ([]*timetable.ScheduledParticipant, error)
	FindByStudentAndDateRange         func(ctx context.Context, studentID int64, from, to calendar.Date) ([]*timetable.ScheduledParticipant, error)
	FindByStudentIDsAndDate           func(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*timetable.ScheduledParticipant, error)
	CreateUnplannedPresentIfAbsent    func(ctx context.Context, instanceID, studentID int64, at time.Time) (*timetable.ScheduledParticipant, error)
	UpdateAttendanceFromCheckin       func(ctx context.Context, instanceID, studentID int64, at time.Time) (bool, error)
	UpdateAttendanceFromCheckinBatch  func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error
	UpdateAttendanceCheckout          func(ctx context.Context, instanceID, studentID int64, at time.Time) error
	UpdateAttendanceCheckoutBatch     func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error
	ReconcileAttendanceInterval       func(ctx context.Context, instanceID, studentID int64, previousIn time.Time, previousOut *time.Time, updatedIn time.Time, updatedOut *time.Time) (bool, error)
}
