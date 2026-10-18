package api

import (
	"context"
	"errors"
	"log/slog"
	"reflect"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CarePlanWiring is what the root builds of Care Plan and hands to this graph
// (#4011, ADR 0053 point 8): the adapters that bind Care Plan to the owners the
// root composes, and the bindings over the values only this graph composes
// (settings, Audit trails, the realtime hub, the parent event emitter, the
// caller context and Care Plan's own services), which the graph calls with
// them.
type CarePlanWiring struct {
	// StatusDays serves Care Plan's status days to Student Presence, the
	// class day and the parents portal.
	StatusDays *StudentStatusDayRepository
	// PickupStudents, PickupStudentLocks and PickupExcusalTimetable bind the
	// pickup schedules and the pickup auto excusal to the People Directory,
	// Timetable and Student Presence.
	PickupStudents         careplanCompose.PickupBulkStudents
	PickupStudentLocks     careplanCompose.PickupStudentLocks
	PickupExcusalTimetable PickupExcusalTimetable
	// ArrivalRules and ArrivalDependencies bind the arrival schedules to the
	// People Directory and the Timetable class arrival plans.
	ArrivalRules        careplanCompose.ArrivalScheduleRules
	ArrivalDependencies careplanCompose.ArrivalScheduleDependencies
	// LifecycleOwners are the owners the care lifecycle drives around a care
	// exit.
	LifecycleOwners careplanCompose.CareLifecycleOwners
	// CompanionGraph is Care Plan's companion lock protocol with People
	// Directory's stranding verdict, for the enrollment approval.
	CompanionGraph enrollmentCompose.CompanionGraphCoordinator
	// The retained care schedule and excused-absence request rows the
	// parents portal reads.
	CareScheduleRequests timetablePorts.CareScheduleRequestRows
	ExcusedRequestRows   ExcusedRequestReads

	// Bindings over the values this graph composes.
	CareParticipation      func(current func() careplan.CareParticipation) peoplePorts.CareParticipationResolver
	ArrivalBaselines       func(approved careplan.ApprovedBookingReader, bookingsAuthoritative func(context.Context) (bool, error)) (careplan.ArrivalBaselineReader, error)
	OfferingCatalog        func(CareOfferingCatalogSources) (careplan.CareOfferingCatalogCapability, error)
	OfferingPhaseGuard     func(careplan.CareOfferingGuards) func(context.Context, int64, *enrollmentOwner.Phase) error
	BookingMaterialization func(BookingMaterializationSources) (careplan.BookingMaterializationCapability, error)
	OfferingRosterResync   func(careplan.SourcedRosters) func(context.Context, timetable.OfferingRosterResyncInput) error
	StudentDocuments       func(StudentDocumentSources) (careplan.StudentDocuments, error)
	ReviewPolicy           func(identityaccess.ParentRequestReviews) CarePlanReviewPolicy
	CareRequests           func(CareRequestSources) carerequests.Service
	PickupAdjustments      func(PickupAdjustmentSources) (careplan.PickupAdjustments, error)
	ExcusedRequests        func(ExcusedRequestSources) (*careplanCompose.ExcusedAbsenceRequests, error)
	SettingsSideEffects    func(*settingsCompose.SideEffectRegistry, careplan.BookingAuthority)
}

// PickupExcusalTimetable previews the blocks an earlier pickup excuses and
// records the later-pickup tasks.
type PickupExcusalTimetable interface {
	careplanCompose.PartialAbsencePreview
	careplanCompose.PickupExtensions
}

// ExcusedRequestReads reads one excused-absence request for the parents
// portal.
type ExcusedRequestReads interface {
	FindByID(ctx context.Context, id any) (*absencerecords.ExcusedAbsenceRequest, error)
}

// CarePlanReviewPolicy is the parent request review reach the request queues
// of this graph apply.
type CarePlanReviewPolicy interface {
	Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
	AbsenceScope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
	Allows(ctx context.Context, permissions []string, student *peopleValues.StudentState) (bool, error)
}

// CareOfferingCatalogSources are the values of this graph the care-offering
// catalog is composed over. Resync resolves the booking materialization,
// which is composed after the catalog; it returns nil until then.
type CareOfferingCatalogSources struct {
	Enrollment     *enrollmentOwner.BookingProjection
	Settings       settingsCompose.SettingsService
	Resync         func() careplan.BookingMaterializationCapability
	LockRecurrence func(context.Context) error
	Today          func() timezone.Date
	Logger         *slog.Logger
}

// BookingMaterializationSources are the values of this graph Care Plan's
// booking materialization is composed over.
type BookingMaterializationSources struct {
	Catalog                  careplan.CareOfferingCatalogCapability
	Rosters                  timetablePorts.RosterReconciler
	Enrollment               *enrollmentOwner.BookingProjection
	Approved                 *enrollmentCompose.ApprovedOfferingProjection
	Settings                 settingsCompose.SettingsService
	Withdrawals              careplanCompose.BookingWithdrawals
	Adjustments              bookingAdjustmentTrail
	Accounts                 *identityaccess.Module
	Pickup                   careplan.PickupBaselineReader
	LockRecurrence           func(context.Context) error
	ResyncPickupAutoExcusals func(ctx context.Context, studentIDs []int64) error
	Broadcaster              realtime.Broadcaster
	GuardianNotifier         *communicationCompose.ParentEventEmitter
	Today                    func() timezone.Date
	Logger                   *slog.Logger
}

// StudentDocumentSources are the caller context and the Audit trails the
// child documents are composed over.
type StudentDocumentSources struct {
	Caller    securityruntime.StudentAccessUserContext
	Edits     studentDocumentEdits
	AccessLog studentDocumentAccessLog
}

// CareRequestSources are the values of this graph the care-schedule change
// requests are composed over.
type CareRequestSources struct {
	Arrivals    careplan.ArrivalScheduleService
	Pickups     careplan.PickupScheduleService
	AutoExcusal careplan.PickupAutoExcusal
	Staff       CareRequestStaff
	Emitter     *communicationCompose.ParentEventEmitter
	Broadcaster realtime.Broadcaster
	Reviews     CarePlanReviewPolicy
	Permissions func(context.Context) []string
	Events      peopleValues.ParentRequestEvents
	Audit       peoplePorts.StudentChangeRecorder
	Shares      communicationCompose.ParentShareVisibilityResolver
	Today       func() timezone.Date
	Logger      *slog.Logger
}

// PickupAdjustmentSources are the values of this graph Care Plan's pickup
// adjustments are composed over.
type PickupAdjustmentSources struct {
	PickupSchedules  careplan.PickupScheduleService
	ArrivalSchedules careplan.ArrivalScheduleService
	Baselines        careplan.PickupBaselineReader
	Offerings        careplan.DirectOfferingAdjustments
	Settings         settingsCompose.SettingsService
	Audit            peoplePorts.StudentPickupPlanRecorder
	Today            func() timezone.Date
}

// ExcusedRequestSources are the values of this graph the excused-absence
// requests are composed over. Nil effects are skipped.
type ExcusedRequestSources struct {
	Reviews     CarePlanReviewPolicy
	Permissions func(context.Context) []string
	Emitter     *communicationCompose.ParentEventEmitter
	Broadcaster realtime.Broadcaster
	Events      peopleValues.ParentRequestEvents
	Notifier    deliveryCompose.AbsenceNotifier
	Shares      careplanCompose.ShareVisibility
	Logger      *slog.Logger
}

// validate refuses a wiring with a missing value or binding: the graph binds
// every one of them while it composes.
func (w CarePlanWiring) validate() error {
	required := []any{
		w.StatusDays, w.PickupStudents, w.PickupStudentLocks, w.PickupExcusalTimetable,
		w.ArrivalRules, w.ArrivalDependencies.Students, w.LifecycleOwners.Students, w.CompanionGraph,
		w.CareScheduleRequests, w.ExcusedRequestRows,
		w.CareParticipation, w.ArrivalBaselines, w.OfferingCatalog, w.OfferingPhaseGuard,
		w.BookingMaterialization, w.OfferingRosterResync, w.StudentDocuments, w.ReviewPolicy,
		w.CareRequests, w.PickupAdjustments, w.ExcusedRequests, w.SettingsSideEffects,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the Care Plan wiring is incomplete")
		}
	}
	return nil
}

// shareVisibilityFunc adapts a closure to the request queues' sharing port.
// The graph uses it to resolve the parents domain, which is composed after
// the request services it serves, at call time instead of at construction.
type shareVisibilityFunc func(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error)

func (f shareVisibilityFunc) SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error) {
	return f(ctx, studentID, requestType, requestID)
}
