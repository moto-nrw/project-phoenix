package compose

import (
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

// The instance lifecycle, its automatic start and end, the status slots and
// the staff deviations live in internal/application (#4084). The roots build
// them through the constructors and dependency types below.

type (
	CareDayLocks                  = application.CareDayLocks
	CareDays                      = application.CareDays
	DeviationEventRecord          = application.DeviationEventRecord
	DeviationLifecycle            = application.DeviationLifecycle
	DeviationProtocol             = application.DeviationProtocol
	DeviationStaff                = application.DeviationStaff
	GuardianNoticeAudience        = application.GuardianNoticeAudience
	GuardianNoticePublication     = application.GuardianNoticePublication
	GuardianNoticePublished       = application.GuardianNoticePublished
	GuardianNotices               = application.GuardianNotices
	InstanceLifecycleDependencies = application.InstanceLifecycleDependencies
	InstanceLifecycleService      = application.InstanceLifecycleService
	InstanceStatusSlots           = application.InstanceStatusSlots
	LifecycleBroadcaster          = application.LifecycleBroadcaster
	LifecycleEvent                = application.LifecycleEvent
	LifecyclePresence             = application.LifecyclePresence
	LifecycleRooms                = application.LifecycleRooms
	LifecycleSettings             = application.LifecycleSettings
	LifecycleStaff                = application.LifecycleStaff
	LifecycleStudents             = application.LifecycleStudents
	SessionEnder                  = application.SessionEnder
	StaffDeviationDependencies    = application.StaffDeviationDependencies
	StatusSlotRows                = application.StatusSlotRows
)

// The deviation event types of the Änderungsprotokoll.
const (
	DeviationEventAbsence           = application.DeviationEventAbsence
	DeviationEventDroppedByReplan   = application.DeviationEventDroppedByReplan
	DeviationEventReturnToPresence  = application.DeviationEventReturnToPresence
	DeviationEventSickCleared       = application.DeviationEventSickCleared
	DeviationEventSickReported      = application.DeviationEventSickReported
	DeviationEventStaffMoved        = application.DeviationEventStaffMoved
	DeviationEventSubstituteRemoved = application.DeviationEventSubstituteRemoved
	DeviationEventSubstitution      = application.DeviationEventSubstitution
)

func NewInstanceLifecycle(deps InstanceLifecycleDependencies) (*InstanceLifecycleService, error) {
	return application.NewInstanceLifecycle(deps)
}

func NewInstanceAutoEnd(instances scheduleModel.ActivityInstanceRepository, completion timetable.InstanceLifecycle) (timetable.InstanceAutoEnd, error) {
	return application.NewInstanceAutoEnd(instances, completion)
}

func NewInstanceStatusSlots(rows StatusSlotRows) InstanceStatusSlots {
	return application.NewInstanceStatusSlots(rows)
}

func NewStaffDeviations(deps StaffDeviationDependencies) (timetable.StaffDeviations, error) {
	return application.NewStaffDeviations(deps)
}
