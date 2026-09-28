package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// Resource defines the students API resource
type Resource struct {
	ResourceConfig
}

// ClassListEntry is one class-list-only child (#2382) as this resource reads
// it: a name and a free-text class, nothing else exists.
type ClassListEntry struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// PrivacyConsentCapability is the Student Presence owner surface this
// resource needs for the per-child GDPR retention consent. Student Presence
// owns users.privacy_consents because the recorded window bounds how long
// presence data is kept (#3349).
type PrivacyConsentCapability interface {
	ListPrivacyConsents(context.Context, int64) ([]studentpresence.PrivacyConsent, error)
	RecordPrivacyConsent(context.Context, studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error)
	RevisePrivacyConsent(context.Context, studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error)
}

// FamilyProtectionCapability is the People Directory owner surface behind the
// per-child privacy ledger: the current flag and the append-only change.
type FamilyProtectionCapability interface {
	peopleModule.FamilyProtectionQuery
	peopleModule.FamilyProtectionCommand
}

// ClassListEntryReader hands over the entries in the class-then-name display
// order the "Klassenliste" export and the class dropdown both rely on. The
// root binds it to the School Membership capability that owns them.
type ClassListEntryReader interface {
	ListClassListEntriesInDisplayOrder(context.Context) ([]ClassListEntry, error)
}

// ChildQuotaUsage is the school's Kinderkontingent (Booked) next to its
// Kontingentzahl (Occupied), as the Datenverwaltung shows it (#3569).
type ChildQuotaUsage struct {
	Booked   int
	Occupied int
}

// ChildQuotaReader reads the Kinderkontingent of the caller's school. The
// root binds it to the School Membership capability that counts the
// Kontingentzahl; limited is false when the school has no Kinderkontingent.
type ChildQuotaReader interface {
	ChildQuotaUsage(context.Context) (usage ChildQuotaUsage, limited bool, err error)
}

// WeekdayPickupNoteReplacer owns the one atomic write that replaces the
// recurring day notes for a child. It deliberately excludes dated notes.
type WeekdayPickupNoteReplacer interface {
	ReplaceWeekdayPickupNotes(context.Context, int64, int64, map[int]string) error
}

// PlannedStudents is the consumer-owned port to the Timetable owner's
// planned-block lookup the day planning reads (#584): which of the children
// have a planned block on a date.
type PlannedStudents interface {
	GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error)
}

// StudentPresence is the consumer-owned presence port of the students inbound
// (#3352). It names exactly what the student list, day planning, visit and
// school check-in handlers read and write on the Student Presence capability:
// the shared location snapshot (embedded, which also brings the presence mode
// and the bulk attendance read), today's attendance status, the current and
// past room visits, the present/in-transit/in-room list filters, the session a
// visit belongs to, and the explicit web check-in, check-out and batch
// commands. The composition root binds the public capability to it; handlers
// never see kiosk sessions, student moves or maintenance.
type StudentPresence interface {
	common.StudentLocationReader
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*studentpresence.DailyAttendanceStatus, error)
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error)
	FindVisitsByStudentID(ctx context.Context, studentID int64) ([]studentpresence.Visit, error)
	ListStudentsPresentToday(ctx context.Context) ([]int64, error)
	ListStudentsInTransit(ctx context.Context) ([]int64, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
	GetActiveGroup(ctx context.Context, id int64) (*studentpresence.SessionDetail, error)
	EndVisit(ctx context.Context, id int64) error
	CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*studentpresence.AttendanceResult, error)
	CheckOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*studentpresence.AttendanceResult, error)
	ProcessSchoolCheckinBatch(ctx context.Context, studentIDs []int64, staffID int64, action string) (*studentpresence.SchoolCheckinBatchResult, error)
}

// The public capability satisfies the port; a contract change surfaces here.
var _ StudentPresence = studentpresence.Presence(nil)

// ResourceConfig holds all dependencies for creating a students Resource.
// Using a config struct instead of individual parameters improves maintainability.
type ResourceConfig struct {
	// PeopleDirectory is the owner capability the routes read and write a
	// child's own row, the persons they render under and the directory list
	// through (#3349, #2731).
	PeopleDirectory peopleModule.Capability
	// Persons is the person half the retained person service still decides
	// (see PersonRecords).
	Persons                PersonRecords
	SchoolGroups           SchoolGroups
	UserContextService     CallerContext
	ActiveService          StudentPresence
	PickupScheduleService  careplan.PickupScheduleService
	WeekdayPickupNotes     WeekdayPickupNoteReplacer
	PartialAbsenceService  careplan.PartialAbsenceService
	ArrivalScheduleService careplan.ArrivalScheduleService
	InstanceService        PlannedStudents
	// CareDayService gates the day-planning timetable signal on the child's
	// care plan (#1747) — without it a child assigned to a block counts as
	// "kommt heute" on every weekday, including the ones they are not booked
	// for. Optional: nil keeps the unfiltered pre-#1747 behaviour, which is
	// what bare test Resources rely on.
	CareDayService  careplan.CareDayQuery
	SchoolService   SchoolDirectory
	SettingsService TenantSettings
	// CompanionService is the Care Plan "läuft mit" graph (#3350): the links
	// themselves and the lock protocol every writer of them shares. It is a
	// second field rather than part of StudentService because the two halves
	// of the child record have different owners.
	CompanionService careplan.StudentCompanions
	// ClassListEntries supplies the class-list-only entries (#2382) the
	// "Klassenliste" export merges into the Klassenverband, read through
	// their School Membership owner in the display order the export needs.
	// Optional: nil exports without entries (bare test Resources).
	ClassListEntries ClassListEntryReader
	// ChildQuota backs the Kinderkontingent line of the Datenverwaltung
	// (#3569). Optional for bare test Resources; the route answers 500
	// without it.
	ChildQuota ChildQuotaReader
	// StudentDeletion is the owner workflow behind the permanent deletion
	// routes (#2710): delete-impact, DELETE /{id}, the graduate purge and the
	// withdrawal deletion. Optional so bare test Resources still compile; the
	// routes answer 500 rather than deleting through a second path when nil.
	StudentDeletion *studentdeletion.Workflow
	// CareLifecycleService backs "Betreuung beenden" (#2487) — the regular
	// exit, which is deliberately NOT a deletion.
	CareLifecycleService careplan.CareLifecycle
	// StudentAuditService is the owner's per-child change history. Optional:
	// nil skips recording (bare test Resources) and answers the history route
	// with 500.
	StudentAuditService     StudentChangeHistory
	MasterDataReviewService masterdatarequests.Decisions
	CareRequestService      carerequests.Decisions
	CareRequestReviews      careplan.CareScheduleReviewQuery
	// OfferingChangeService backs the post-enrollment offering-change queue
	// (#1665).
	OfferingChangeService    careplan.OfferingChangeRequests
	PickupAdjustmentService  careplan.PickupAdjustments
	ExcusedRequestService    excusedrequests.Service
	ParentRequestBulkService parentrequests.BulkApprover
	// ParentRequestConflictService resolves a whole conflict group at once
	// (#2267). Optional: a bare test Resource answers 500 rather than
	// silently deciding requests one by one, which is the bug the group
	// exists to prevent.
	ParentRequestConflictService parentrequests.ConflictResolver
	// FamilyProtection is the People Directory owner capability behind the
	// per-child privacy ledger (#3349). Optional: a bare test Resource answers
	// 500 rather than reaching the ledger through a second path.
	FamilyProtection FamilyProtectionCapability
	// RequestReviewAccess reports the caller's coarse reach over the parent
	// request queues so the empty list can explain itself. Optional: a nil
	// policy omits the field (bare test Resources).
	RequestReviewAccess ParentRequestReviewAccess
	// RequestReview is the shared request-review projection (#2705) behind
	// the aggregated list and the pending-count badge. Optional for bare
	// test Resources; the two routes answer 500 without it.
	RequestReview           requestreview.Query
	StudentStatusDayService studentpresence.StatusDays
	AbsenceOverview         studentpresence.StatusDayOverviews
	StudentHistoryService   studentpresence.StudentHistory
	OGSGroupLiveService     grouplive.Query
	// ActiveEnrollments backs the export's "angemeldet" column. Optional:
	// an export that asks for the column answers 500 without it.
	ActiveEnrollments    ActiveEnrollments
	EnrollmentDecision   enrollmentOwner.Decisions
	EnrollmentFormSchema enrollmentOwner.FormSchemaAdministration
	// OfferingPickupTimes is Care Plan's offering pickup projection (#3560):
	// the reset of a manual weekly Gehzeit onto the Angebots-Gehzeit.
	// Optional for bare test Resources; the reset route answers 500 without
	// it.
	OfferingPickupTimes careplan.OfferingPickupTimes
	// OfferingSourceResyncer re-reconciles Jahrgang-filtered offering-sourced
	// Regeltermine after a direct school_class edit, in the same transaction —
	// the same hook a grade transition uses (#2147 review round 10). Optional:
	// nil skips the resync (bare test Resources).
	OfferingSourceResyncer OfferingSourceResyncer
	// LockTemplateRecurrence takes the tenant-wide recurrence gate the resync
	// requires. It must be acquired BEFORE the student row locks (see
	// applyStudentUpdate for the ordering rationale). Required whenever
	// OfferingSourceResyncer is set.
	LockTemplateRecurrence func(ctx context.Context) error
	Broadcaster            realtime.Broadcaster
	// ParentEventEmitter wakes a child's guardians (message-independent
	// parent_child_updated SSE fan-out) after staff-side care writes, so an open
	// parents-app tab refetches the child's care state live (#1725). Optional —
	// nil is a no-op (the guardian helper guards on it), so tests that build a
	// bare Resource keep working.
	ParentEventEmitter GuardianWake
	AbsenceNotifier    notificationsService.AbsenceNotifier
	StudentPhotos      StudentPhotoLifecycle
	// StudentConsents serves the shared consent projection (People Directory)
	// and records every effective change (Audit Platform) for the retained
	// student rows this resource holds (#3349).
	StudentConsents StudentConsentCapability
	// PrivacyConsents is the Student Presence owner capability over
	// users.privacy_consents (#3349). Optional: a bare test Resource answers
	// 500 on the two consent routes rather than reaching the table through a
	// second path.
	PrivacyConsents PrivacyConsentCapability
	// StudentDocumentService backs the child's Dokumente tab (#777).
	StudentDocumentService careplan.StudentDocuments
	ListExportService      lists.Renderer
	Logger                 *slog.Logger
	Now                    func() time.Time
	// DeviceAuthenticator guards the RFID routes. The Device Fleet
	// composition builds it; this resource only mounts it. Without one the
	// routes reject every request rather than serve unauthenticated kiosks.
	DeviceAuthenticator common.Middleware
	// AuthenticatedDevice reads the kiosk the authenticator admitted on the
	// request (its device id). The root binds it to the Device Fleet
	// authenticator's request context; nil treats every request as
	// unauthenticated.
	AuthenticatedDevice DeviceIdentity
}

// DeviceIdentity reports the device id of the kiosk the device authenticator
// admitted on this request, or false when none did.
type DeviceIdentity func(context.Context) (deviceID string, ok bool)

// NewResource creates a new students resource from the provided configuration.
func NewResource(cfg ResourceConfig) *Resource {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Resource{ResourceConfig: cfg}
}

func (rs *Resource) todayDate() timezone.Date {
	return timezone.DateFromTime(rs.Now())
}

// Router returns a configured router for student endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		rs.mountStudentReadRoutes(r, withTx)
		rs.mountRequestDecisionRoutes(r, withTx)
		rs.mountRequestQueueRoutes(r, withTx)
		rs.mountStudentWriteRoutes(r, withTx)
		rs.mountPickupRoutes(r, withTx)
		rs.mountArrivalRoutes(r, withTx)
		rs.mountCheckinAndFileRoutes(r, withTx)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates API key + PIN and sets tenant context,
	// then TenantTxMiddleware wraps each handler in a tenant-scoped transaction
	// (SET LOCAL ROLE phoenix_tenant + set_config) so RLS is enforced.
	r.Group(rs.mountDeviceRoutes)

	return r
}

// requiredDeviceAuthenticator returns the configured device authenticator, or
// a fail-closed one when the composition supplied none: a bare handler test or
// a broken graph logs the misconfiguration and never serves the RFID routes
// unauthenticated.
func (rs *Resource) requiredDeviceAuthenticator() common.Middleware {
	if rs.DeviceAuthenticator != nil {
		return rs.DeviceAuthenticator
	}
	rs.getLogger().Error("device route group mounted without an authenticator; every request is rejected",
		slog.String("middleware", "DeviceAuthenticator"),
	)
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			renderError(w, r, common.ErrorUnauthorized(errors.New("device API key is required")))
		})
	}
}

// withinTenant runs fn in the transaction of tenantID. Inside a route that
// already runs under the tenant transaction middleware it joins that
// transaction, so an error returned from fn rolls the whole request back.
func withinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		tenant.ObserveMissingTenant(ctx, err)
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}
