// Package presenceservice composes the Student Presence presence services
// (sessions, visits, attendance, status days, history and retention cleanup)
// behind the public facades of modules/studentpresence.
package presenceservice

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The presence services read and lock facts owned by other capabilities; the
// composition root implements these ports over their owners.
type (
	PresenceDependencies               = presence.ServiceDependencies
	PresenceOption                     = presence.ServiceOption
	RequestPrincipal                   = presence.RequestPrincipal
	PresenceSettingsResolver           = presence.SettingsResolver
	GuardianWaker                      = presence.GuardianWaker
	School                             = presence.School
	SchoolQuery                        = presence.SchoolQuery
	PresenceStudents                   = presence.PresenceStudents
	AttendanceStaff                    = presence.AttendanceStaff
	AttendanceStaffNames               = presence.AttendanceStaffNames
	AttendanceRooms                    = presence.AttendanceRooms
	SessionRoom                        = activeModels.SessionRoom
	AttendanceActivityCategories       = presence.AttendanceActivityCategories
	AttendanceEducationGroups          = presence.AttendanceEducationGroups
	EducationGroupRoom                 = presence.EducationGroupRoom
	SessionDeviceDirectory             = presence.SessionDeviceDirectory
	StudentDisplayFacts                = presence.StudentDisplayFacts
	PresenceRetention                  = presence.PresenceRetention
	DeletionAudit                      = presence.DeletionAudit
	DeletionEvent                      = presence.DeletionEvent
	DataAccessAudit                    = presence.DataAccessAudit
	DataAccessEvent                    = presence.DataAccessEvent
	AttendanceHistoryReader            = presence.AttendanceHistoryReader
	HistoryRoomReader                  = presence.HistoryRoomReader
	HistorySlotReader                  = presence.HistorySlotReader
	ManualPartialAbsenceReader         = presence.ManualPartialAbsenceReader
	StudentStatusDayRepository         = presence.StudentStatusDayRepository
	StudentStatusDayOverviewRepository = presence.StudentStatusDayOverviewRepository
	StatusDayOverviewPeople            = presence.StatusDayOverviewPeople
)

// WithPresenceSettings supplies the tenant-scoped settings resolver. Without
// it the presence mode and the sick/excused clear modes cannot be resolved and
// the affected commands fail loudly.
func WithPresenceSettings(resolver PresenceSettingsResolver) PresenceOption {
	return presence.WithSettings(resolver)
}

// WithPresenceTenantRuntime supplies the transaction runtime commands open
// their own tenant transaction through when no request transaction exists.
func WithPresenceTenantRuntime(runtime tenant.UnitOfWork) PresenceOption {
	return presence.WithTenantRuntime(runtime)
}

// WithGuardianWaker wakes the guardians after attendance changes (#2252).
func WithGuardianWaker(waker GuardianWaker) PresenceOption {
	return presence.WithGuardianWaker(waker)
}

// NewPresence builds the presence capability from explicit dependencies.
func NewPresence(deps PresenceDependencies, options ...PresenceOption) studentpresence.Presence {
	return newPresenceFacade(presence.NewService(deps, options...))
}

// PresenceEngine returns the application service a composed presence value
// runs on. Only Student Presence's own behaviour tests use it; other callers
// depend on the public facades.
func PresenceEngine(value studentpresence.Presence) (presence.Service, bool) {
	facade, ok := value.(*presenceFacade)
	if !ok {
		return nil, false
	}
	return facade.engine, true
}

// NewPresenceCleanup builds the data-retention cleanup; the optional clock
// fixes the calendar day stale records are compared against.
func NewPresenceCleanup(presenceRetention PresenceRetention, supervisors activeModels.GroupSupervisorRepository, deletions DeletionAudit, today ...func() timezone.Date) studentpresence.PresenceCleanup {
	return presence.NewCleanupService(presenceRetention, supervisors, deletions, today...)
}

// NewStudentHistory composes owner attendance reads with visit history,
// data-access logging and planned slot attendance. slots may be nil.
func NewStudentHistory(attendance AttendanceHistoryReader, rooms HistoryRoomReader, accessLog DataAccessAudit, slots HistorySlotReader) studentpresence.StudentHistory {
	return presence.NewStudentHistoryService(attendance, rooms, accessLog, slots)
}

// NewStatusDays builds the status-day reads and writes over Care Plan's
// status-day adapter; a full-day status never silently overwrites a
// time-specific excusal on the same date.
func NewStatusDays(repo StudentStatusDayRepository, pickupExceptions ManualPartialAbsenceReader, db studentpresence.DatabaseHandle, lockExceptionDay LockExceptionDay, clocks ...func() time.Time) studentpresence.StatusDays {
	return statusDays{presence.NewStudentStatusDayServiceWithPartialAbsences(repo, pickupExceptions, db, lockExceptionDay, clocks...)}
}

// statusDays names the status-day delete after the capability.
type statusDays struct {
	*presence.StudentStatusDayService
}

func (s statusDays) DeleteStatusDay(ctx context.Context, wc studentpresence.StatusDayWriteContext, statusDayID, studentID int64) error {
	return s.DeleteByID(ctx, wc, statusDayID, studentID)
}

// NewStatusDayOverviews builds the tenant-wide absence overview.
func NewStatusDayOverviews(repo StudentStatusDayOverviewRepository, people StatusDayOverviewPeople) studentpresence.StatusDayOverviews {
	return presence.NewStudentStatusDayOverviewService(repo, people)
}

// GroupSupervisorRowWriter is the supervision write services/education still
// performs with retained supervisor rows.
type GroupSupervisorRowWriter interface {
	CreateGroupSupervisor(context.Context, *activeModels.GroupSupervisor) error
}

// GroupSupervisorRows bridges the retained supervisor-row writer of a
// composed presence value until the supervision rows move behind the owner
// (#3422). It returns nil for any other value.
func GroupSupervisorRows(value studentpresence.Presence) GroupSupervisorRowWriter {
	engine, ok := PresenceEngine(value)
	if !ok {
		return nil
	}
	return engine
}
