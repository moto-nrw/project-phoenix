package timetableplanning

import (
	"log/slog"
	"time"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication"
	activeModel "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

// InstanceServiceDependencies aggregates wiring. All repo fields are required;
// Broadcaster is optional (nil → no SSE).
type InstanceServiceDependencies struct {
	InstanceRepo       scheduleModel.ActivityInstanceRepository
	IdempotencyRepo    scheduleModel.InstanceIdempotencyRepository
	InstanceStaffRepo  scheduleModel.InstanceStaffRepository
	InstanceStudents   scheduleModel.InstanceStudentRepository
	ExceptionRepo      scheduleModel.ActivityExceptionRepository
	ActiveGroupRepo    activeModel.GroupRepository
	SupervisorRepo     activeModel.GroupSupervisorRepository
	Presence           InstancePresence
	RoomRepo           facilitiesModel.RoomRepository
	ActivityGroupRepo  activitiesModel.GroupRepository
	StaffRepo          usersModel.StaffRepository
	StudentRepo        usersModel.StudentRepository
	CalendarPeriodRepo scheduleModel.CalendarPeriodRepository
	ActiveService      ActiveSessionEnder
	Materialization    MaterializationService
	// CareDayService decides which still-expected children may be stamped
	// absent when an instance ends (#1747) — required.
	CareDayService InstanceCareDays
	// DeviationEventRepo appends the Änderungsprotokoll (#1886) — required.
	DeviationEventRepo auditModel.DeviationEventRepository
	Broadcaster        realtime.Broadcaster
	DB                 *bun.DB
	Logger             *slog.Logger
	Settings           LifecycleSettings
	RecoveryRepo       scheduleModel.ActivityRecoveryRepository
	Now                func() time.Time
	EnforceTimePolicy  bool
	// GuardianNotices publishes the cancellation notice to families (#2601).
	// Optional: nil means a cancellation can never carry a notice. The
	// composition root passes a late-bound publisher because the announcement
	// service is built after this one.
	GuardianNotices announcement.CareCancellationPublisher
}
