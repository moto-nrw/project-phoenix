// Package presenceservice composes the Student Presence presence services
// (sessions, visits, attendance, status days, history and retention cleanup)
// behind the public facades of modules/studentpresence.
package presenceservice

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
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
	SessionRoom                        = ports.SessionRoom
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
	return newPresenceFacade(presence.NewService(deps, options...), deps)
}

// presenceEngineOf returns the application service a composed presence value
// runs on.
func presenceEngineOf(value studentpresence.Presence) (presence.Service, bool) {
	facade, ok := value.(*presenceFacade)
	if !ok {
		return nil, false
	}
	return facade.engine, true
}

// NewPresenceCleanup builds the data-retention cleanup; the optional clock
// fixes the calendar day stale records are compared against.
func NewPresenceCleanup(presenceRetention PresenceRetention, supervisors ports.GroupSupervisorRepository, deletions DeletionAudit, today ...func() timezone.Date) studentpresence.PresenceCleanup {
	return presence.NewCleanupService(presenceRetention, supervisors, deletions, today...)
}
