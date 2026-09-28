package application

import (
	"context"
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Enrollment owner ports of the decision flow; the root binds the owner.
type (
	// DecisionRequests reads and locks the requests a decision works on.
	DecisionRequests interface {
		RequestByID(context.Context, int64, bool) (*enrollment.Request, error)
		AdminRequests(context.Context, enrollment.RequestListFilters) ([]*enrollment.Request, error)
		SetRequestWithdrawal(context.Context, int64, *time.Time) error
		AcquireSubmissionDedupLock(context.Context, int64, uint64) error
		AcquireExistingStudentMatchLock(context.Context, int64) error
		ActiveDuplicateChildren(context.Context, int64, string, []enrollment.DuplicateChildKey, int64) ([]enrollment.DuplicateChildKey, error)
		HasActiveRequestForMatchedStudent(context.Context, int64, int64, int64) (bool, error)
	}
	// DecisionChildren reads the request children with their offering
	// selections and writes the decision state.
	DecisionChildren interface {
		OfferingCapacityPeak(context.Context, int64, []int64, enrollment.Date, enrollment.Date) (int, error)
		RequestChildOfferingsAtDate(context.Context, int64, enrollment.Date) ([]*enrollment.RequestChildOffering, error)
		RequestChildOfferingHistoryForChildren(context.Context, []int64) ([]*enrollment.RequestChildOffering, error)
		RequestChildOfferingsForChildrenAtDate(context.Context, []int64, enrollment.Date) ([]*enrollment.RequestChildOffering, error)
		ChildByID(context.Context, int64) (*enrollment.RequestChild, error)
		ChildrenForRequest(context.Context, int64, bool) ([]*enrollment.RequestChild, error)
		ChildrenForRequests(context.Context, []int64) ([]*enrollment.RequestChild, error)
		RestoreWithdrawnChildren(context.Context, int64, []int64) ([]int64, error)
		UpdateChildStatus(context.Context, int64, string, *string, int64) error
		UpdateChildActivationPlan(context.Context, int64, string, *enrollment.Date) error
		LinkCreatedStudent(context.Context, int64, int64) error
	}
	// DecisionGuardians reads the co-guardians of a request and stamps the
	// guardian profile an approval resolved for one of them.
	DecisionGuardians interface {
		RequestGuardians(context.Context, []int64) ([]*enrollment.RequestGuardian, error)
		StampRequestGuardianProfile(context.Context, int64, int64) error
	}
	// DecisionLateInvites reads the late invite a request was submitted
	// through.
	DecisionLateInvites interface {
		LateInviteByUsedRequestID(context.Context, int64) (*enrollment.LateInvite, error)
	}
	// DecisionPhases reads the phases of the requests.
	DecisionPhases interface {
		Phase(context.Context, int64) (*enrollment.Phase, error)
		PhasesByID(context.Context, []int64) ([]*enrollment.Phase, error)
	}
	// DecisionSchemas reads the immutable form-schema versions requests are
	// pinned to.
	DecisionSchemas interface {
		Schema(context.Context, int64) (*enrollment.FormSchema, error)
		Schemas(context.Context, []int64) ([]*enrollment.FormSchema, error)
	}
	// CareOfferingCatalog reads Care Plan's offering catalog in Enrollment's
	// offering rows.
	CareOfferingCatalog interface {
		ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
		ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
		ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	}
)

// DecisionBookings is the Care Plan booking materialization an Enrollment
// decision drives (#3560).
type DecisionBookings interface {
	careplan.BookingMaterializer
	careplan.OfferingAdjustments
	ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error
}

// DecisionGuardianAccess is the Identity & Access seam the approval workflow
// consumes: resolve the parent's existing platform account and make it a
// guardian of this school. The public Identity & Access capability satisfies
// it; the approval never touches the account, mapping or role tables itself
// (#2699).
type DecisionGuardianAccess interface {
	FindAccount(ctx context.Context, id int64) (identityaccess.Account, error)
	FindAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error)
	GrantGuardianTenantAccess(ctx context.Context, accountID int64) (identityaccess.GuardianTenantAccess, error)
}

// DecisionStudentEnrollment is the bounded People Directory student seam used
// by approval, renewal and accepted form changes. It cannot update
// attendance.
type DecisionStudentEnrollment = peopleEnrollment.Commands

// DecisionSettings resolves the tenant settings the decision flow reads. A
// nil DecisionSettings applies the registry defaults: waitlist, guardian
// auto-invite and care offerings on, scheduled activation.
type DecisionSettings interface {
	WaitlistEnabled(ctx context.Context) (bool, error)
	AutoInviteGuardianOnApprove(ctx context.Context) (bool, error)
	CareOfferingsEnabled(ctx context.Context) (bool, error)
	// DefaultActivationMode returns enrollment.default_activation_mode:
	// "immediate" or "scheduled".
	DefaultActivationMode(ctx context.Context) (string, error)
}

// Audit Platform ports of the decision flow.
type (
	// OfferingAdjustmentTrail lists the audited offering adjustments of one
	// request child.
	OfferingAdjustmentTrail interface {
		ListOfferingAdjustments(ctx context.Context, requestChildID int64) ([]*enrollment.OfferingAdjustmentRecord, error)
	}
	// RestorationAudit appends the trail of an admin restore (#2157).
	RestorationAudit interface {
		RecordRestoration(ctx context.Context, restoration Restoration) error
	}
	// PayerAudit appends one change of the payer flag to the guardian
	// financial change ledger.
	PayerAudit interface {
		RecordPayerChange(ctx context.Context, change PayerChange) error
	}
	// StudentAudit records tracked profile changes made while approving or
	// synchronizing enrollment data for an existing student.
	StudentAudit interface {
		RecordChangesForActor(ctx context.Context, before, after *Student, editedBy int64) error
		RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
	}
	// StudentConsents records the effective consent transitions an
	// enrollment submission applied to a student.
	StudentConsents interface {
		RecordEnrollmentConsentTransitions(ctx context.Context, before, after *Student, actorAccountID *int64, changedAt time.Time) error
	}
)

// Restoration is the append-only trail row of an admin restore.
type Restoration struct {
	RequestID      int64
	ChildIDs       []int64
	ActorAccountID *int64
	RestoredAt     time.Time
}

// PayerChange is one before/after pair of a relationship's payer flag.
type PayerChange struct {
	GuardianProfileID int64
	StudentID         int64
	ChangedBy         int64
	OldValue          string
	NewValue          string
	Note              string
}

// CareWithdrawalReconciler persists or obsoletes the durable follow-up in the
// same tenant transaction as the authoritative booking change.
type CareWithdrawalReconciler interface {
	ReconcileAuthoritativeBookingChange(ctx context.Context, change careplan.CareWithdrawalBookingChange) error
}

// StudentPlanBroadcasts announces a synced departure plan to open student and
// companion views.
type StudentPlanBroadcasts interface {
	BroadcastStudentUpdated(tenantID int64, source string) error
	BroadcastStudentCompanionsChanged(tenantID int64, source string) error
}

// WeeklyPickupHooks keep an approved weekly Gehzeit plan in step with the
// staff weekly editors (#2360): the student lock first, the comparison of the
// plan before and after, and the re-derived auto excusals, all in the
// caller's transaction. Every hook is optional.
type WeeklyPickupHooks struct {
	LockStudents  func(ctx context.Context, studentIDs []int64) error
	ResyncExcusal func(ctx context.Context, studentIDs []int64) error
	Snapshot      func(ctx context.Context, studentID int64, date calendar.Date) (map[int]string, error)
	RecordChanges func(ctx context.Context, studentID int64, date calendar.Date, before map[int]string) error
}

// DecisionDependencies bind the decision flow to its owners. Guardians,
// LateInvites, Bookings, the audit ports, the schedule ports, Settings,
// Broadcasts and the pickup hooks are optional where the flow degrades
// without them; the flow reports every other missing dependency when it
// needs it.
type DecisionDependencies struct {
	Requests          DecisionRequests
	Children          DecisionChildren
	Guardians         DecisionGuardians
	LateInvites       DecisionLateInvites
	Phases            DecisionPhases
	Schemas           DecisionSchemas
	Offerings         CareOfferingCatalog
	Notifications     enrollment.Notifications
	Bookings          DecisionBookings
	GuardianAccess    DecisionGuardianAccess
	StudentEnrollment DecisionStudentEnrollment
	People            PeopleDirectory
	Companions        DepartureCompanions
	PickupSchedules   WeeklyPickupSchedules
	ArrivalSchedules  WeeklyArrivalSchedules
	AccessLog         ExportAccessLog
	Adjustments       OfferingAdjustmentTrail
	Restorations      RestorationAudit
	Payers            PayerAudit
	StudentAudit      StudentAudit
	StudentConsents   StudentConsents
	CareWithdrawal    CareWithdrawalReconciler
	Broadcasts        StudentPlanBroadcasts
	Settings          DecisionSettings
	// LockTemplateRecurrence takes the tenant recurrence gate before an
	// approval rewrites an existing student's class, the same gate Care
	// Plan's booking materialization serializes its roster writes with.
	LockTemplateRecurrence func(context.Context) error
	Pickups                WeeklyPickupHooks
	Runtime                Runtime
	ParentsURL             string
	Logger                 *slog.Logger
	Today                  func() calendar.Date
}
