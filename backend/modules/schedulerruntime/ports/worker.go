// Package ports is the contract the typed roots fill to build the Worker
// (ADR 0056 point 4): its complete dependencies, the consumer-owned ports
// the Worker declares onto the owners and the results they report. The
// root binds every port; the Worker implementation in services/scheduler
// names these types and nothing of another owner (ADR 0058 point 2).
package ports

import (
	"context"
	"log/slog"
	"time"
)

// WorkerDependencies is the complete typed input to the embedded Worker root.
// Construction replaces the former post-construction Set* graph.
type WorkerDependencies struct {
	Logger *slog.Logger
	// Now supplies the worker clock; nil uses time.Now.
	Now                       func() time.Time
	Getenv                    func(string) string
	SchoolRepo                TenantDirectory
	TenantRuntime             TenantRuntime
	TenantRuntimeObserver     func(entryPoint, outcome string)
	UnitOfWorkObserver        func(entryPoint, kind, result string, duration time.Duration, retries int)
	Tracer                    WorkerTracer
	Settings                  SettingsResolver
	Active                    SessionMaintenance
	ActiveCleanup             PresenceCleanup
	AuthCleanup               AuthCleanup
	InvitationCleanup         InvitationCleaner
	EmailChangeCleanup        EmailChangeTokenCleaner
	OperatorInvitationCleanup OperatorInvitationCleaner
	// OperatorRetention deletes the operator ledger and the deactivated
	// operator accounts after their window (#4047).
	OperatorRetention OperatorRetention
	// AccountRetention tombstones the accounts no school needs (#4043).
	AccountRetention AccountRetention
	// HolidayRegistrationRetention deletes ended holiday registrations of
	// every Träger (#4661).
	HolidayRegistrationRetention HolidayRegistrationRetention
	WorkSessionCleanup           WorkSessionCleaner
	BreakAutoEnder               BreakAutoEnder
	AutoCheckouter               AutoCheckouter
	FeedbackCleaner              FeedbackCleaner
	// RetentionSteps are the owners' deletions of a school's nightly cleanup
	// (#3950), RetentionLog records each deletion; NewWorker builds the run
	// over them. CleanupRuns keeps each school's last successful nightly run.
	RetentionSteps            []RetentionStep
	RetentionLog              RetentionLog
	CleanupRuns               CleanupRunLedger
	StaffDocumentCleaner      StaffDocumentFileCleaner
	StudentDocumentCleaner    StudentDocumentFileCleaner
	FileStoreCleaner          FileStoreCleaner
	Materializer              TimetableMaterializer
	TimetableCleanup          TimetableCleanup
	CalendarFeedCleanup       CalendarFeedCleaner
	TimeTrackingCleanup       TimeTrackingCleanupService
	StudentChangeLogCleanup   StudentChangeLogCleanup
	PWAUsageCleanup           PWAUsageCleanup
	StaffMessageCleanup       StaffMessageCleanup
	BookingConsistency        BookingConsistencyAudit
	HolidayGuests             HolidayGuestReconciliation
	PlannedStaffExits         PlannedStaffExits
	WeekProtocols             WeekProtocolEvenings
	PayrollDispatch           PayrollDispatch
	MealOrders                MealOrderSettlement
	EnrollmentRejectedCleanup RejectedEnrollmentCleaner
	AutoStart                 InstanceAutoStart
	AutoEnd                   InstanceAutoEnd
	InstanceRepo              DayInstanceReader
	InstanceRoomRepo          ExistingRoomReader
	InstanceStudentRepo       InstanceCheckoutCloser
	TimetableBridge           TimetableBridgeCompleter
	StudentStatusDayRepo      StudentStatusFlagArchiver
	OverdueBroadcaster        InstanceOverdueBroadcaster
	StudentLifecycleRepo      StudentLifecycleRepository
	StudentLifecycleAudit     StudentLifecycleAuditor
	CareExitEffector          CareExitEffector
	OutboxWorker              OutboxWorkerRunner
	// SettleExpiredTransfers terminalizes ambiguous manual uploads, never retries them.
	SettleExpiredTransfers func(context.Context, int) (int, error)
	RolloverDeadlineRunner RolloverDeadlineRunner
	ReminderNotifications  ReminderNotificationTick
	// Lease elects the single Worker that runs jobs (#2726). Every Worker
	// process, embedded in Serve or standalone, runs under it.
	Lease WorkerLease
	// AppointmentReminders is the established reminder capability consumed by
	// the scheduler. It also exposes scheduled parent-announcement delivery,
	// avoiding a second dependency in this shrink-only worker composition.
	AppointmentReminders ReminderCommands
}

// Worker is the built Worker as a root runs it: Start schedules the jobs
// under the lease, Stop drains them, Ready reports whether this process
// holds the lease.
type Worker interface {
	Start()
	Stop()
	Ready() bool
}
