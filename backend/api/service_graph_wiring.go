package api

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/realtime"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
)

// WorkforceWiring is what the root binds of Workforce into the service graph
// (#4008, ADR 0053 point 8). The adapters over the owners the root composes
// come built. The bindings over the staff rows, audit trails, settings and mail
// only this graph builds come as functions the graph calls with them.
type WorkforceWiring struct {
	// ShiftRows and ShiftTypeRows serve the retained Dienstplan rows of the
	// Timetable composition from Workforce.
	ShiftRows     timetablePorts.WorkforceShiftRows
	ShiftTypeRows timetablePorts.WorkforceShiftTypeRows
	// StaffAdmin composes the personnel-record administration.
	StaffAdmin func(WorkforceStaffAdminSources) (*workforceModule.StaffAdmin, error)
	// TimeTracking composes the time-tracking side of Workforce.
	TimeTracking func(WorkforceTimeTrackingSources) (*workforceCompose.TimeTracking, error)
	// PlanExportStaff serves the plan export's staff names.
	PlanExportStaff planexport.StaffNameReader
}

// WorkforceStaffAdminSources are the retained staff and person rows and the
// audit trails the personnel-record administration is bound over.
type WorkforceStaffAdminSources struct {
	Staff   staffAdminStaffRows
	Persons staffAdminPersonRows
	Audits  *auditCompose.Persistence
	Logger  *slog.Logger
	Now     func() time.Time
}

// WorkforceTimeTrackingSources are the retained rows, audit trails,
// settings, broadcaster and mail the time-tracking ports are bound over.
type WorkforceTimeTrackingSources struct {
	Audits         *auditCompose.Persistence
	Supervisions   studentpresence.SupervisionRecords
	SessionRecords *presenceCompose.SessionRecords
	Staff          timeTrackingStaffRows
	People         peoplePorts.PersonDirectory
	Holidays       workforceCompose.HolidayDatesReader
	Settings       settingsCompose.SettingsService
	Payroll        settingsCompose.PayrollStatusGetter
	Broadcaster    realtime.Broadcaster
	Mail           WorkforceMail
	ShiftPlanSync  workforceCompose.ShiftPlanSyncer
	Logger         *slog.Logger
}

// WorkforceMail is the tenant mail the absence requests go out through.
type WorkforceMail struct {
	Dispatcher interface {
		Dispatch(context.Context, email.DeliveryRequest)
	}
	From        email.Email
	ReplyTo     email.ReplyToResolver
	FrontendURL string
}

func (w WorkforceWiring) complete() bool {
	return w.ShiftRows != nil && w.ShiftTypeRows != nil &&
		w.StaffAdmin != nil && w.TimeTracking != nil && w.PlanExportStaff != nil
}

// PeopleWiring is what the root builds for the People Directory, School
// Membership and School Structure and hands to this graph (#4007, ADR 0053
// point 8): the retained repositories over those owners and the adapters that
// bind the owners to the consumers this graph composes. An adapter over a value
// this graph composes itself (an Audit trail, the Identity & Access module, the
// session records, the person service, the realtime hub) comes as a constructor
// the graph calls with that value.
type PeopleWiring struct {
	// Identity is the Identity & Access module the retained repositories read
	// accounts and memberships through.
	Identity *identityaccess.Module
	// The retained person, student, guardian, staff and teacher rows.
	peopleCompose.Repositories
	Groups        EducationGroupRepository
	GroupTeachers GroupTeacherRepository
	ClassTeachers ClassTeacherRepository
	// CaregiverBindingLock serializes the caregiver capability re-check over
	// the owners whose tables can retain the capability.
	CaregiverBindingLock peoplePorts.CaregiverBindingLocker

	// The directories of the School Structure group and substitution
	// services.
	GroupRooms            education.RoomDirectory
	GroupTeacherDirectory education.TeacherDirectory
	StaffChecks           interface {
		education.StaffDirectory
		education.StaffLockStore
	}
	SubstitutionPersons education.PersonQuery
	Caregivers          education.CaregiverDirectory
	ExternalCaregivers  education.ExternalCaregiverDirectory

	// Student Presence's reads of the students, the visitors and the schools.
	StudentDisplay      presenceservice.StudentDisplayReader
	CrossTenantStudents presenceservice.CrossTenantRepo
	PresenceSchools     presenceservice.SchoolQuery
	PresenceStudents    presenceservice.PresenceStudents

	// The Organisation & Tenancy school reads of the mail identity, the
	// enrollment flows, the device displays and the parent-message branding.
	SchoolContacts    deliveryCompose.MailSchoolContacts
	EnrollmentSchools enrollmentOwner.SchoolDirectory
	DisplaySchools    DisplaySchoolDirectory
	SchoolNames       interface {
		FindSchoolName(ctx context.Context, id int64) (string, bool, error)
	}

	// StudentAudit records the per-child change history through the People
	// Directory.
	StudentAudit peoplePorts.StudentAuditRecorder

	// Care Plan's schedule rows in Timetable's retained vocabulary.
	PickupSchedules   timetablePorts.CarePickupScheduleRows
	ArrivalSchedules  timetablePorts.CareArrivalScheduleRows
	PickupExceptions  timetablePorts.CarePickupExceptionRows
	ArrivalExceptions timetablePorts.CareArrivalExceptionRows

	// Constructors over the values this graph composes.
	SupervisionRecords       func(studentpresence.SupervisionRecords) studentpresence.SupervisionRecords
	SupervisionBlockers      func(studentpresence.SupervisionRecords) peoplePorts.SupervisionBlockers
	CaregiverIdentity        func(*identityaccess.Module) peoplePorts.CaregiverIdentity
	CaregiverCapabilityAudit func(AuthEventAppender) peoplePorts.CaregiverCapabilityAudit
	GuardianFinancialAudit   func(auditModels.GuardianFinancialChangeCreator) peoplePorts.GuardianFinancialAudit
	GuardianDataAccessLog    func(DataAccessAppender) peoplePorts.GuardianDataAccessLog
	ClassAssignmentAudit     func(auditModels.StaffMasterDataChangeCreator) education.ClassAssignmentAudit
	SubstitutionAudit        func(auditModels.SubstitutionChangeCreator) education.SubstitutionAudit
	StudentConsents          func(StudentConsentAppender) StudentConsentRecords
	StudentChangeLogCleanup  func(edits peoplePorts.StudentChangeLogStore, deletions DataDeletionAppender, settings RetentionSettings, logger *slog.Logger) peoplePorts.StudentChangeLogCleanupService
	StatusDayPeople          func(peoplePorts.PersonDirectory) presenceservice.StatusDayOverviewPeople
	GroupRosterPeople        func(peoplePorts.PersonDirectory) GroupRoster
	MasterDataDecisions      func(MasterDataDecisionWiring) (*careplanCompose.MasterDataDecisions, error)
	ParentRequestCoordinator func(*careplanCompose.MasterDataDecisions, careplan.ExcusedAbsenceRequests, any, careplanCompose.ParentRequestConflicts, peopledirectory.ParentRequestEvents, func(context.Context) []string) (parentrequests.Coordinator, error)
	ParentRequestWriteScope  func(policy ParentRequestScopePolicy, permissions func(context.Context) []string) careplanCompose.ReviewScopeResolver
}

// The Audit Platform trails the root's adapters append to, as the narrow
// ports they write through.
type (
	AuthEventAppender interface {
		Create(ctx context.Context, event *auditModels.AuthEvent) error
	}
	DataAccessAppender interface {
		Create(ctx context.Context, entry *auditModels.DataAccessLog) error
	}
	DataDeletionAppender interface {
		Create(ctx context.Context, deletion *auditModels.DataDeletion) error
	}
	StudentConsentAppender interface {
		Create(ctx context.Context, entry *auditModels.StudentConsentChange) error
	}
	FileEventAppender interface {
		Create(ctx context.Context, event *auditModels.FileEvent) error
	}
)

// RetentionSettings resolves a tenant's retention period setting.
type RetentionSettings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// validate refuses a wiring with a missing value or constructor: the graph
// binds every one of them while it composes.
func (w PeopleWiring) validate() error {
	required := []any{
		w.Identity, w.Persons, w.Students, w.StudentGuardians, w.GuardianProfiles, w.Staff, w.Teachers,
		w.Groups, w.GroupTeachers, w.ClassTeachers, w.CaregiverBindingLock,
		w.GroupRooms, w.GroupTeacherDirectory, w.StaffChecks, w.SubstitutionPersons, w.Caregivers, w.ExternalCaregivers,
		w.StudentDisplay, w.CrossTenantStudents, w.PresenceSchools, w.PresenceStudents,
		w.SchoolContacts, w.EnrollmentSchools, w.DisplaySchools, w.SchoolNames, w.StudentAudit,
		w.PickupSchedules, w.ArrivalSchedules, w.PickupExceptions, w.ArrivalExceptions,
		w.SupervisionRecords, w.SupervisionBlockers, w.CaregiverIdentity, w.CaregiverCapabilityAudit,
		w.GuardianFinancialAudit, w.GuardianDataAccessLog, w.ClassAssignmentAudit, w.SubstitutionAudit,
		w.StudentConsents, w.StudentChangeLogCleanup, w.StatusDayPeople, w.GroupRosterPeople,
		w.MasterDataDecisions, w.ParentRequestCoordinator, w.ParentRequestWriteScope,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the People Directory, School Membership and School Structure wiring is incomplete")
		}
	}
	return nil
}

// EnrollmentWiring is what the root binds of Enrollment into this graph (#4012,
// ADR 0053 point 8): the booking projection over Care Plan's bookings and the
// adapters over the owners the root composes come built. The flows over the
// settings, audit trails, outbox, realtime hub and the other values only this
// graph builds come as functions the graph calls with them.
type EnrollmentWiring struct {
	// Bookings is the Enrollment owner with Care Plan's bookings bound in.
	Bookings *enrollmentOwner.BookingProjection
	// ApprovedOfferings resolves approved bookings to the People Directory's
	// students.
	ApprovedOfferings *enrollmentCompose.ApprovedOfferingProjection
	// CareBookings records and replaces a child's bookings in Care Plan.
	CareBookings EnrollmentCareBookings
	// Guardians is the People Directory guardian read of the deletion preview.
	Guardians enrollmentCompose.GuardianDirectory
	// The parents portal's reads of a child's care periods and bookings.
	ParentCarePeriods     EnrollmentCarePeriods
	ParentOfferingHistory EnrollmentOfferingHistory
	// OfferingPlanning serves the offering-change review's manual planning
	// and course groups from Enrollment and Timetable.
	OfferingPlanning careplanCompose.OfferingChangePlanning

	// BindSettingsGuards registers Enrollment's active phase restrictions as
	// the Settings Platform's collection guards (#1663).
	BindSettingsGuards func(settingsCompose.BatchSettingsService)
	// MailRenderers render Enrollment's outbox kinds from the default sender.
	MailRenderers       func(defaultFrom email.Email) map[string]deliveryCompose.MailRenderer
	Notifications       func(EnrollmentSettings, MailOutbox) enrollmentOwner.Notifications
	Deletions           func(EnrollmentDeletionSources) enrollmentCompose.DeletionModule
	Phases              func(EnrollmentPhaseSources) *enrollmentCompose.Phases
	Decisions           func(EnrollmentDecisionSources) *enrollmentCompose.Decisions
	Capacity            func(EnrollmentSettings) enrollmentOwner.OfferingCapacity
	AdminSubscribers    func(deliveryCompose.EmailSubscribers, deliveryCompose.EmailAccounts) enrollmentCompose.AdminMailSubscribers
	Intake              func(EnrollmentIntakeSources) *enrollmentCompose.Intake
	Reports             func(EnrollmentReportSources) enrollmentOwner.Reports
	GuardianInvitations func(EnrollmentGuardianInvitations) enrollmentCompose.GuardianInvitationAvailability
	ChangeRequests      func(EnrollmentChangeRequestSources) enrollmentOwner.ChangeRequests
	Rollovers           func(EnrollmentRolloverSources) enrollmentOwner.Rollovers
	OfferingChanges     func(OfferingChangeInputs) (careplan.OfferingChangeCapability, error)
	OfferingConflicts   func(careplan.OfferingChangeConflicts) careplanCompose.ParentRequestConflicts
}

// validate refuses a wiring with a missing value or constructor.
func (w EnrollmentWiring) validate() error {
	required := []any{
		w.Bookings, w.ApprovedOfferings, w.CareBookings, w.Guardians, w.ParentCarePeriods, w.ParentOfferingHistory,
		w.OfferingPlanning, w.BindSettingsGuards, w.MailRenderers, w.Notifications, w.Deletions, w.Phases,
		w.Decisions, w.Capacity, w.AdminSubscribers, w.Intake, w.Reports, w.GuardianInvitations,
		w.ChangeRequests, w.Rollovers, w.OfferingChanges, w.OfferingConflicts,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the Enrollment wiring is incomplete")
		}
	}
	return nil
}

// EnrollmentCareBookings records and replaces the bookings of an approved
// child in Care Plan.
type EnrollmentCareBookings interface {
	RecordCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error
	ReplaceCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error
}

// EnrollmentCarePeriods answers the parents portal's care periods.
type EnrollmentCarePeriods interface {
	CarePeriods(ctx context.Context, studentID int64) ([]*parentportal.CarePeriod, error)
}

// EnrollmentOfferingHistory answers the parents portal's booked offerings.
type EnrollmentOfferingHistory interface {
	OfferingHistory(ctx context.Context, requestChildID int64) ([]*parentportal.OfferingBooking, error)
}

// EnrollmentSettings is the slice of the settings service the Enrollment
// bindings resolve through.
type EnrollmentSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	LockClassCollectionPair(ctx context.Context) error
}

// EnrollmentGuardianInvitations lists the open invitations of guardian
// profiles.
type EnrollmentGuardianInvitations interface {
	ListOpenGuardianInvitations(context.Context, []int64) ([]identityaccess.GuardianInvitation, error)
}

// EnrollmentPhaseSources are the values of this graph the phase
// administration is bound over.
type EnrollmentPhaseSources struct {
	LockTemplateRecurrence          func(context.Context) error
	ValidateCareOfferingPhaseChange func(context.Context, int64, *enrollmentOwner.Phase) error
	// SourcedTemplates resolves the booking materialization on every edit.
	SourcedTemplates func() enrollmentOwner.SourcedTemplateResyncer
	Settings         EnrollmentSettings
	Logger           *slog.Logger
}

// EnrollmentDecisionSettings resolves the tenant settings of the decision
// flow.
type EnrollmentDecisionSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// EnrollmentStudentAuditor records tracked profile changes made while
// approving or synchronizing enrollment data for an existing student.
type EnrollmentStudentAuditor interface {
	RecordChangesForActor(ctx context.Context, before, after *peopledirectory.StudentState, editedBy int64) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
}

// EnrollmentConsentAuditor records effective consent transitions made while an
// enrollment submission is applied to a student.
type EnrollmentConsentAuditor interface {
	RecordTransitions(ctx context.Context, before, after *peopledirectory.StudentState, source string, actorAccountID *int64, changedAt time.Time) error
}

// EnrollmentDepartureCompanions reads Care Plan's "läuft mit" edges of a
// student.
type EnrollmentDepartureCompanions interface {
	ListForStudent(context.Context, int64) ([]*careplan.CompanionEdge, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
}

// EnrollmentDecisionSources are the owners and retained repositories the root
// binds the decision flow to. A nil source skips the work that needs it, the
// way the decision flow always treated an unwired repository.
type EnrollmentDecisionSources struct {
	Requests               enrollmentCompose.DecisionRequests
	Children               enrollmentCompose.DecisionChildren
	Guardians              enrollmentCompose.DecisionGuardians
	LateInvites            enrollmentCompose.DecisionLateInvites
	Phases                 enrollmentCompose.DecisionPhases
	Schemas                enrollmentCompose.DecisionSchemas
	CareOfferings          enrollmentCompose.CareOfferingCatalog
	DataAccessLog          enrollmentAccessLogAppender
	OfferingAdjustments    enrollmentAdjustmentRows
	Restorations           enrollmentRestorationAppender
	Notifications          enrollmentOwner.Notifications
	Persons                enrollmentPersonRows
	Staff                  enrollmentStaffRows
	Students               enrollmentStudentRows
	StudentEnrollment      EnrollmentDecisionPeople
	Companions             EnrollmentDepartureCompanions
	DeleteCompanions       func(context.Context, []int64) error
	StudentGuardians       enrollmentStudentGuardianRows
	GuardianFinancialAudit auditModels.GuardianFinancialChangeCreator
	GuardianProfiles       enrollmentGuardianProfileRows
	GuardianPhones         enrollmentGuardianPhoneRows
	PickupSchedules        enrollmentCompose.WeeklyPickupSchedules
	ArrivalSchedules       enrollmentCompose.WeeklyArrivalSchedules
	// CareBookings is Care Plan's booking materialization (#3560). Nil skips
	// the roster materialization of an approval (focused tests).
	CareBookings enrollmentCompose.DecisionBookings
	// GuardianAccess is the Identity & Access capability an approval uses to
	// recognise a parent's existing portal account. Required: an approval
	// without it fails instead of silently skipping the account attach.
	GuardianAccess  enrollmentCompose.DecisionGuardianAccess
	StudentAudit    EnrollmentStudentAuditor
	StudentConsents EnrollmentConsentAuditor
	CareWithdrawal  enrollmentCompose.CareWithdrawalReconciler
	// Broadcaster announces student_updated + student_companions_changed
	// after an approved enrollment sync trimmed a child's "läuft mit" links.
	Broadcaster realtime.Broadcaster
	FrontendURL string
	// ParentsURL is the base of the status links in the parent mails. It
	// falls back to FrontendURL when empty.
	ParentsURL             string
	Settings               EnrollmentDecisionSettings
	LockTemplateRecurrence func(context.Context) error
	Pickups                enrollmentCompose.WeeklyPickupHooks
	Logger                 *slog.Logger
	Today                  func() timezone.Date
}

// EnrollmentIntakeSettings resolves the tenant settings of the intake by key.
// A resolver that also answers ResolveMany is read through one snapshot.
type EnrollmentIntakeSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentIntakeSources are the owners and retained repositories the root
// binds the intake to. A nil source skips the work that needs it, the way
// the intake always treated an unwired repository.
type EnrollmentIntakeSources struct {
	Requests           enrollmentCompose.IntakeRequests
	Children           enrollmentCompose.IntakeChildren
	Bookings           enrollmentOwner.CareBookingCommands
	Guardians          enrollmentCompose.IntakeGuardians
	LateInviteRepo     enrollmentCompose.IntakeLateInvites
	CareOfferingRepo   enrollmentCompose.IntakeOfferings
	Capacity           enrollmentOwner.OfferingCapacity
	Catalog            enrollmentCompose.IntakeCatalog
	SchoolRepo         enrollmentOwner.SchoolDirectory
	Notifications      enrollmentOwner.Notifications
	StudentRepo        enrollmentCompose.StudentMatches
	GuardianAuthorizer enrollmentCompose.GuardianStudentAuthorizer
	RateLimitRepo      enrollmentCompose.SubmissionRateLimiter
	OutboxEnqueuer     enrollmentCompose.MailOutbox
	Settings           EnrollmentIntakeSettings
	// AdminSubscribers adds the staff who switched on the "Neue Anmeldung"
	// mail (#3780); nil keeps the configured address list only.
	AdminSubscribers enrollmentCompose.AdminMailSubscribers
	ManualDecider    enrollmentCompose.ManualEnrollmentDecider
	// ParentChanges makes a request unread again after a parent edit (#3778).
	ParentChanges enrollmentCompose.RequestParentChanges
	// FrontendURL is the base of the staff links, ParentsURL of the
	// parent-facing ones; it falls back to FrontendURL.
	FrontendURL string
	ParentsURL  string
	Logger      *slog.Logger
}

// EnrollmentChangeRequestSources are the owners and retained repositories the
// root binds the change requests to.
type EnrollmentChangeRequestSources struct {
	Bookings            enrollmentCompose.CareBookingChanges
	Requests            enrollmentCompose.ChangeRequestRecords
	Children            enrollmentCompose.IntakeChildren
	Guardians           enrollmentCompose.IntakeGuardians
	LateInviteRepo      enrollmentCompose.DecisionLateInvites
	CareOfferingRepo    enrollmentCompose.IntakeOfferings
	Capacity            enrollmentOwner.OfferingCapacity
	Catalog             enrollmentCompose.IntakeCatalog
	Notifications       enrollmentOwner.Notifications
	GuardianProfileRepo enrollmentGuardianProfileRows
	GuardianPhoneRepo   enrollmentGuardianPhoneRows
	GuardianInvitations enrollmentCompose.GuardianInvitationAvailability
	PersonRepo          EnrollmentReviewerPersons
	StudentRepo         enrollmentCompose.StudentMatches
	GuardianAuthorizer  enrollmentCompose.GuardianStudentAuthorizer
	// Decisions applies an approved change to a child the decision flow
	// already approved; BookingGates wrap those writes. A nil Decisions
	// leaves every child to the booking replacement.
	Decisions            enrollmentOwner.ApprovedChildChanges
	BookingGates         enrollmentCompose.CareBookingGates
	CompanionGraphLocker enrollmentCompose.CompanionGraphCoordinator
	Settings             EnrollmentIntakeSettings
	OutboxEnqueuer       enrollmentCompose.MailOutbox
	FrontendURL          string
	ParentsURL           string
	Logger               *slog.Logger
}

// EnrollmentReviewerPersons resolves the persons behind reviewer accounts.
type EnrollmentReviewerPersons interface {
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*peopledirectory.DirectoryPersonSnapshot, error)
}

// EnrollmentReportOwnerReads are the Enrollment owner reads of the reports.
type EnrollmentReportOwnerReads interface {
	enrollmentCompose.ReportRequests
	enrollmentCompose.ReportChildren
	enrollmentCompose.ReportGuardians
	enrollmentCompose.ReportSchemas
	enrollmentCompose.ReportPhases
}

// EnrollmentReportSources are the reads the root binds the reports to.
type EnrollmentReportSources struct {
	Owner             EnrollmentReportOwnerReads
	Offerings         enrollmentCompose.ReportOfferings
	AccessLog         enrollmentAccessLogAppender
	Students          enrollmentRosterStudentRows
	Persons           enrollmentRosterPersonRows
	Groups            enrollmentCompose.RosterGroups
	StudentGuardians  enrollmentGuardianContactRows
	Companions        enrollmentCompose.RosterCompanions
	ClassListEntries  enrollmentCompose.ClassListEntries
	PickupSchedules   enrollmentCompose.PickupSchedules
	CareParticipation enrollmentCompose.CareParticipation
	Settings          interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
	}
}

// EnrollmentRolloverSettings resolves the grade collection settings the
// rollover reads.
type EnrollmentRolloverSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentRolloverSources are the owners the root binds the rollover to.
type EnrollmentRolloverSources struct {
	Bookings enrollmentOwner.CareBookingCommands
	Phases   enrollmentCompose.RolloverPhases
	Requests enrollmentCompose.RolloverRequests
	Children enrollmentCompose.RolloverChildren
	// Catalog clones the source phase's care-offering catalog (#2249).
	Catalog          enrollmentCompose.RolloverCatalogCloner
	Notifications    enrollmentOwner.Notifications
	PhaseEligibility enrollmentCompose.PhaseEligibilityGuard
	Outbox           enrollmentCompose.MailOutbox
	Settings         EnrollmentRolloverSettings
	// Decisions approves auto-renewed rows on an auto-approve phase.
	Decisions  enrollmentCompose.RolloverDecider
	ParentsURL string
	Logger     *slog.Logger
}

// EnrollmentDeletionOwner is the Enrollment owner slice the admin deletion
// and the retention cleanup read, lock and delete through.
type EnrollmentDeletionOwner = enrollmentCompose.DeletionOwner

// EnrollmentDeletionSources are the owners the root binds the admin deletion
// and the retention cleanup to.
type EnrollmentDeletionSources struct {
	Owner     EnrollmentDeletionOwner
	Guardians enrollmentCompose.GuardianDirectory
	// CountAuditAdjustments counts the audited offering adjustments of a
	// request or one of its children.
	CountAuditAdjustments func(context.Context, int64, *int64) (int, error)
	// CountBookings counts Care Plan's bookings of the request children.
	CountBookings func(context.Context, []int64) (int, error)
	Audit         enrollmentDeletionAppender
	Delivery      enrollmentCompose.EnrollmentDeletionDelivery
	Settings      interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
	Logger *slog.Logger
}

// OfferingChangeInputs are the owners the offering-change review is composed
// over.
type OfferingChangeInputs struct {
	CarePlan careplan.Capability
	// Catalog reads the care offerings; nil reads them from CarePlan.
	Catalog     careplanCompose.OfferingChangeCatalog
	Enrollment  OfferingChangeEnrollmentReads
	Students    offeringChangeStudentRows
	Withdrawals offeringChangeWithdrawalRows
	Settings    OfferingChangeSettingsReads
	Planning    careplanCompose.OfferingChangePlanning
	Bookings    careplan.BookingMaterializationCapability
	Reviews     OfferingChangeReviewPolicy
	// Permissions reads the permissions of the request claims.
	Permissions func(context.Context) []string
	Emitter     *communicationCompose.ParentEventEmitter
	Events      peopledirectory.ParentRequestEvents
	Shares      careplanCompose.ShareVisibility
	Today       func() timezone.Date
	Logger      *slog.Logger
}

// OfferingChangeEnrollmentReads is the part of the Enrollment owner an
// offering change reads.
type OfferingChangeEnrollmentReads interface {
	StudentCarePeriods(context.Context, int64) ([]*enrollmentOwner.StudentCarePeriod, error)
	RequestByID(context.Context, int64, bool) (*enrollmentOwner.Request, error)
	ChildByID(context.Context, int64) (*enrollmentOwner.RequestChild, error)
	ChildrenByID(context.Context, []int64) ([]*enrollmentOwner.RequestChild, error)
	Phase(context.Context, int64) (*enrollmentOwner.Phase, error)
	RequestChildOfferingsAtDate(context.Context, int64, enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	EffectiveOfferingSelectionsAtDates(context.Context, map[int64]enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	OfferingCatalogState(context.Context, int64, int64, enrollmentOwner.Date, enrollmentOwner.Date) (*enrollmentOwner.OfferingCatalogState, error)
	OfferingCapacityPeak(context.Context, int64, []int64, enrollmentOwner.Date, enrollmentOwner.Date) (int, error)
}

// OfferingChangeReviewPolicy is the parent-request review policy's scope of
// the caller.
type OfferingChangeReviewPolicy interface {
	Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
}

// EducationGroupRepository is the retained group store this graph and its
// consumers read: School Structure's contract plus the group supervisors the
// staff notification recipients resolve.
type EducationGroupRepository interface {
	education.LegacyGroupRepository
	// ListGroupSupervisors answers who supervises the groups on the day, as
	// the notification recipients' own pair type (#3556).
	ListGroupSupervisors(ctx context.Context, groupIDs []int64, on timezone.Date) ([]deliveryCompose.StaffGroupPair, error)
}

// GroupTeacherRepository is the retained contract of education.group_teacher,
// served over School Membership.
type GroupTeacherRepository interface {
	Create(ctx context.Context, relation *schoolmembership.GroupAssignment) error
	FindByID(ctx context.Context, id any) (*schoolmembership.GroupAssignment, error)
	Update(ctx context.Context, relation *schoolmembership.GroupAssignment) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*schoolmembership.GroupAssignment, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*schoolmembership.GroupAssignment, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*schoolmembership.GroupAssignment, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*schoolmembership.GroupAssignment, error)
	// The group service's teacher assignment store (#3556).
	education.GroupTeacherStore
	// ListGroupTeacherBlockers returns group assignments as
	// caregiver-capability blocker rows.
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]peopledirectory.BlockerGroup, error)
}

// ClassTeacherRepository is the retained contract of the staff-to-school-class
// assignments (#1772), served over School Membership.
type ClassTeacherRepository interface {
	Create(ctx context.Context, assignment *schoolmembership.ClassAssignment) error
	FindByID(ctx context.Context, id any) (*schoolmembership.ClassAssignment, error)
	Update(ctx context.Context, assignment *schoolmembership.ClassAssignment) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*schoolmembership.ClassAssignment, error)
	// The class assignment service's store (#3556).
	education.ClassTeacherStore
	// FindByStaff returns the class assignments of one staff member.
	FindByStaff(ctx context.Context, staffID int64) ([]*schoolmembership.ClassAssignment, error)
}

// GroupRoster is the live group roster's People port: the participation
// candidates of a group with their names.
type GroupRoster interface {
	GroupMembers(ctx context.Context, groupID int64) ([]grouplive.RosterStudent, error)
}

// StudentConsentRecords resolves and records a child's consent states over the
// People Directory and the Audit Platform trail.
type StudentConsentRecords interface {
	CurrentStates(ctx context.Context, student *peopledirectory.StudentState, canManagePhoto bool) ([]peopledirectory.StudentConsentState, error)
	RecordTransitions(ctx context.Context, before, after *peopledirectory.StudentState, source string, actorAccountID *int64, changedAt time.Time) error
	CurrentStudentConsents(ctx context.Context, snapshot peopledirectory.StudentConsentSnapshot, canManagePhoto bool) ([]peopledirectory.StudentConsentState, error)
	RecordStudentConsentTransitions(ctx context.Context, before, after peopledirectory.StudentConsentSnapshot, source string, actorAccountID *int64, changedAt time.Time) error
}

// ParentRequestScopePolicy is the identity-owned review reach the Stammdaten
// decision writes with.
type ParentRequestScopePolicy interface {
	Scope(ctx context.Context, permissions []string) (bool, []int64, error)
}

// MasterDataDecisionWiring is the graph's view of the Stammdaten decision's
// dependencies. Nil effects are skipped.
type MasterDataDecisionWiring struct {
	CarePlan    careplan.Capability
	Fields      MasterDataPeople
	Students    masterDataStudentRows
	Persons     masterDataPersonRows
	Audit       peoplePorts.StudentChangeRecorder
	Scope       careplanCompose.ReviewScopeResolver
	Emitter     *communicationCompose.ParentEventEmitter
	Broadcaster realtime.Broadcaster
	Events      peopledirectory.ParentRequestEvents
	Shares      careplanCompose.ShareVisibility
	Logger      *slog.Logger
	Today       func() timezone.Date
}

// PresenceWiring is what the root builds for Student Presence, Facilities,
// Device Fleet and the class-day views and hands to this graph (#4010, ADR 0053
// point 8). A binding over values only this graph composes (the person service,
// the Audit trails, the settings, the Audit command) comes as a constructor the
// graph calls with them.
type PresenceWiring struct {
	// Presence is the Student Presence owner the retained services read and
	// write through, its operations logged at debug level. UnloggedPresence
	// logs only its failed operations.
	Presence         *studentpresence.Module
	UnloggedPresence *studentpresence.Module
	// SessionRecords are the session and supervision records over the room,
	// activity, device and staff owners.
	SessionRecords *presenceCompose.SessionRecords
	// PartialAbsencePreview previews the blocks an earlier pickup excuses.
	PartialAbsencePreview arrivalTimetable.PartialAbsencePreview
	// The attendance reads of the active service over the People Directory,
	// School Structure and Timetable.
	AttendanceStaff           presenceservice.AttendanceStaff
	AttendanceEducationGroups presenceservice.AttendanceEducationGroups
	AttendanceActivities      presenceservice.AttendanceActivityGroups
	// RoomService is the retained room service with its occupancy and
	// history over Student Presence; OpenGroups are the running sessions the
	// yard reads.
	RoomService facilitiesCompose.RoomService
	OpenGroups  facilitiesCompose.OpenGroupCatalog
	// The running sessions and released rooms of the supervision dashboard.
	OpenRoomSessions SupervisionOpenRoomSessions
	OpenRooms        supervisiondashboard.RoomDirectory
	// StatusDays are the scheduled day statuses over Care Plan.
	StatusDays studentpresence.StatusDays
	// ClassListEntries are School Membership's class-list entries in the
	// enrollment class roster's vocabulary.
	ClassListEntries enrollmentCompose.ClassListEntries

	// Constructors over the values this graph composes.
	BindClassListEntries func(auditModels.Command) error
	SessionDevices       func(DeviceOnlineWindowSettings) presenceservice.SessionDeviceDirectory
	AttendanceStaffNames func(peoplePorts.PersonDirectory) presenceservice.AttendanceStaffNames
	ClassDay             func(ClassDaySources) classday.ClassDay
	Statistics           func(StatisticsSources) studentpresence.StatisticsReports
}

// ClassDaySources are what this graph composes of the school portal's
// class-day capability: the caller, the reports, the schedules and care
// days, the student rows, the access log, the settings, the block starts and
// the broadcaster.
type ClassDaySources struct {
	Caller     classdayCompose.CallerReader
	Reports    enrollmentOwner.Reports
	StatusDays interface {
		FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date absencerecords.Date) ([]*absencerecords.StudentStatusDay, error)
	}
	PickupTimes            classdayCompose.ClassDayPickupReader
	ArrivalTimes           classdayCompose.ArrivalTimeReader
	CareDays               classdayCompose.CareDayResolver
	Companions             classdayCompose.Companions
	Students               classDayStudentRows
	Persons                classDayPersonRows
	StudentGuardians       classDayContactRows
	AccessLog              accessLogLedger
	ClassArrivalExceptions careplan.ClassArrivalExceptions
	Settings               classDayWriteScopeSettings
	BlockStarts            classdayCompose.BlockStarts
	// Broadcaster is optional: without it no arrival schedule change is
	// announced.
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

// StatisticsSources are what this graph composes of the Statistik report:
// the access log and the retention settings.
type StatisticsSources struct {
	AccessLog accessLogLedger
	Settings  RetentionSettings
}

// validate refuses a wiring with a missing value or constructor.
func (w PresenceWiring) validate() error {
	required := []any{
		w.Presence, w.UnloggedPresence, w.SessionRecords, w.PartialAbsencePreview,
		w.AttendanceStaff, w.AttendanceEducationGroups, w.AttendanceActivities,
		w.RoomService, w.OpenGroups, w.OpenRoomSessions, w.OpenRooms, w.StatusDays, w.ClassListEntries,
		w.BindClassListEntries, w.SessionDevices, w.AttendanceStaffNames, w.ClassDay, w.Statistics,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the Student Presence, Facilities, Device Fleet and class-day wiring is incomplete")
		}
	}
	return nil
}

// FileStorageWiring carries what the File Storage module needs from the root
// and cannot compose itself: the uploads object store and the metrics sink.
// Test factories that leave Objects nil get no file storage and no attachment
// purger; production composition requires both.
type FileStorageWiring struct {
	Objects filestorageCompose.ObjectBackend
	Observe func(filestorageCompose.Observation)
	// Settings and Events bind File Storage to the settings and the Audit
	// file events this graph composes.
	Settings func(settingsCompose.SettingsService) filestorageCompose.Settings
	Events   func(FileEventAppender) filestorageCompose.Events
}

// PlatformWiring is what the root builds of the platform owners, Communication
// and Organisation & Tenancy and hands to this graph (#4013, ADR 0053 point 8):
// the values over the owners the root composes, and the adapters that bind
// those owners to the values this graph composes itself.
type PlatformWiring struct {
	// SchoolSettings keeps the tenant settings on the school row of
	// Organisation & Tenancy.
	SchoolSettings settingsCompose.SchoolSettingsStore
	// OperatorAuditLog is the operator ledger of the Audit Platform.
	OperatorAuditLog operatorAuditEntries
	// ParentPortal are the retained guardian-portal, parent-messaging and
	// announcement repositories over their owners.
	ParentPortal ParentPortalRepositories
	// DeviceFleet is the unobserved Device Fleet the retained session and
	// device contracts read.
	DeviceFleet devicefleetModule.Capability
	// ListExport renders the list documents.
	ListExport *lists.RendererService

	// Constructors over the values this graph composes.
	DataAccessAudit            func(DataAccessAppender) presenceservice.DataAccessAudit
	DeletionAudit              func(DataDeletionAppender) presenceservice.DeletionAudit
	TimeTrackingRetentionAudit func(DataDeletionAppender) workforceCompose.DeletionAudit
	GuardianDisplays           func(GuardianDisplayQuery) deliveryCompose.GuardianDisplayResolver
	AnnouncementDeliveries     func(*deliveryModule.Module) communicationCompose.ParentAnnouncementDeliveryRecorder
	EnrollmentDeliveries       func(*deliveryModule.Module) enrollmentCompose.EnrollmentDeletionDelivery
	OperatorProvisioning       func(OperatorProvisioningSources) (organizationtenancy.Provisioning, error)
	PWAUsageCounts             func(organizationtenancy.Provisioning) deliveryCompose.PWAUsageCounts
}

// GuardianDisplayQuery names the guardians of the Delivery recipients.
type GuardianDisplayQuery interface {
	GuardianDisplays(context.Context, []int64) ([]peoplePorts.GuardianDisplay, error)
}

// OperatorProvisioningSources are the values of this graph the root binds
// Organisation & Tenancy's operator provisioning over (#3253).
type OperatorProvisioningSources struct {
	Identity     *identityaccess.Module
	Accounts     OperatorAccountDirectory
	Devices      devicefleetModule.Capability
	Sessions     *presenceCompose.SessionRecords
	Supervisions studentpresence.SupervisionRecords
	// Categories seeds the activity categories of a new school.
	Categories interface {
		CreateCategory(ctx context.Context, name, description, color string) error
	}
	Settings settingsCompose.SettingsService
	Logger   *slog.Logger
}

// validate refuses a wiring with a missing value or constructor: the graph
// binds every one of them while it composes.
func (w PlatformWiring) validate() error {
	required := []any{
		w.SchoolSettings, w.OperatorAuditLog, w.ParentPortal.ParentAnnouncement, w.DeviceFleet, w.ListExport,
		w.DataAccessAudit, w.DeletionAudit, w.TimeTrackingRetentionAudit, w.GuardianDisplays,
		w.AnnouncementDeliveries, w.EnrollmentDeliveries, w.OperatorProvisioning, w.PWAUsageCounts,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the platform, Communication and Organisation & Tenancy wiring is incomplete")
		}
	}
	return nil
}

// TimetableWiring is what the root binds of Timetable & Activities, the School
// Calendar and Reminder Delivery into the service graph (#4009, ADR 0053 point
// 8). The values over the owners the root composes come built. The bindings
// over values only this graph builds come as functions the graph calls with
// them.
type TimetableWiring struct {
	// Repositories are the Timetable owner's retained activities and schedule
	// repositories over the capabilities the root composes (#2743).
	Repositories arrivalTimetable.Repositories
	// EducationGroupNames names School Structure's education groups for the
	// Timetable owner rows (#3551).
	EducationGroupNames timetablePorts.EducationGroupNames
	// PlanningTracks is the planning-track administration the template
	// writes and the plan export read.
	PlanningTracks timetable.PlanningTrackAdministration
	// RecurrenceLock is the tenant recurrence gate every recurrence writer of
	// the graph serializes on (#3424 slice S2).
	RecurrenceLock timetable.RecurrenceWriteLock
	// ClassDayTimetable serves the slot lists' block and roster reads.
	ClassDayTimetable classdayCompose.TimetableReader
	// HistoryRooms and HistorySlots serve the student history's room names
	// and planned slots.
	HistoryRooms presenceservice.HistoryRoomReader
	HistorySlots presenceservice.HistorySlotReader
	// PlanExportClosingDays and PlanExportHolidays label the plan export's
	// non-working days from the School Calendar.
	PlanExportClosingDays planexport.ClosingDayReader
	PlanExportHolidays    planexport.HolidayReader
	// CalendarGuardians is the calendar's guardian directory over the People
	// Directory.
	CalendarGuardians calendarCompose.GuardianDirectory

	// Constructors over the values this graph composes.
	AttendanceMirror       func(arrivalTimetable.OwnerRows, *slog.Logger) (studentpresence.AttendanceSyncer, error)
	EndedSessionCompletion func(arrivalTimetable.OwnerRows, careplan.CareDayQuery) (timetable.EndedSessionCompletion, error)
	DeviationPorts         func(TimetableDeviationTrail) (timetablePorts.DeviationEventReader, timetablePorts.DeviationProtocol)
	ConflictDetection      func(TimetableConflictReaders) (timetable.ConflictDetectionCapability, error)
	Lifecycle              func(TimetableLifecycleInputs) (timetablePorts.InstanceLifecycle, error)
	StaffDeviations        func(TimetableDeviationInputs) (timetablePorts.StaffDeviationDependencies, error)
	Cleanup                TimetableCleanupComposer
	Operations             func(TimetableOperationInputs) (timetable.OperationCapability, error)
	Materialization        func(TimetableMaterializationInputs) timetablePorts.MaterializationDependencies
	Data                   func(TimetableDataInputs) (timetable.TimetableDataCapability, error)
	Templates              func(TimetableTemplateInputs) (timetablePorts.TemplateAdministrationDependencies, error)
	AttendanceCorrections  func(TimetableCorrectionInputs) (timetable.AttendanceCorrections, error)
	Calendar               func(CalendarDependencies) calendarCompose.Application
	AppointmentRenderer    func(defaultFrom email.Email) deliveryCompose.MailRenderer
	ReminderQuery          func(ReminderSources) reminder.Query
	ReminderCommand        func(calendarCompose.Application) reminder.Command
}

// validate refuses a wiring with a missing value or constructor: the graph
// binds every one of them while it composes.
func (w TimetableWiring) validate() error {
	required := []any{
		w.EducationGroupNames, w.PlanningTracks, w.RecurrenceLock, w.ClassDayTimetable, w.HistoryRooms, w.HistorySlots,
		w.PlanExportClosingDays, w.PlanExportHolidays, w.CalendarGuardians,
		w.AttendanceMirror, w.EndedSessionCompletion, w.DeviationPorts, w.ConflictDetection, w.Lifecycle,
		w.StaffDeviations, w.Cleanup, w.Operations, w.Materialization, w.Data, w.Templates,
		w.AttendanceCorrections, w.Calendar, w.AppointmentRenderer, w.ReminderQuery, w.ReminderCommand,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the Timetable, School Calendar and reminder wiring is incomplete")
		}
	}
	if w.Repositories.ActivityInstance == nil {
		return errors.New("the Timetable repositories are required")
	}
	return nil
}

// TimetableDeviationTrail is the Audit Platform's Änderungsprotokoll as the
// Timetable owner rows read and append it.
type TimetableDeviationTrail interface {
	Create(ctx context.Context, event *auditModels.DeviationEvent) error
	ListByRange(ctx context.Context, from, to auditModels.Date, activityGroupID *int64, startTime *string) ([]*auditModels.DeviationEvent, error)
}

// TimetableConflictReaders are the retained readers the Timetable conflict
// detection and staffing capability reads (#3550). The production factory
// and the test compositions fill them from their own repository sets.
// ArrivalBaselines is optional (the exception-conflict read fails without
// it); every other reader is required.
type TimetableConflictReaders struct {
	Instances         timetablePorts.ConflictInstances
	InstanceStaff     timetablePorts.ConflictInstanceStaff
	InstanceStudents  timetablePorts.ConflictInstanceStudents
	Exceptions        timetablePorts.ConflictExceptions
	Schedules         timetablePorts.ConflictSchedules
	Staff             timetablePorts.ConflictStaffDirectory
	CalendarPeriods   timetablePorts.ConflictCalendarPeriods
	ArrivalExceptions timetablePorts.ConflictArrivalExceptions
	Sessions          timetablePorts.ConflictSessions
	Shifts            timetablePorts.ConflictShifts
	Presence          timetablePorts.ConflictPresence
	ArrivalBaselines  careplan.ArrivalBaselineReader
	Logger            *slog.Logger
}

// TimetableLifecycleInputs compose the lifecycle over the retained rows
// (Instances, InstanceStaff, Participants, Templates, Students, Rooms,
// DeviationEvents and the recovery Locks). Planning carries the retained
// planning repositories the rows do not (IdempotencyRepo, ExceptionRepo,
// CalendarPeriodRepo) and the composition root's DB; every other field of it
// is filled from these inputs. Broadcaster and GuardianNotices are optional.
type TimetableLifecycleInputs struct {
	Rows                arrivalTimetable.OwnerRows
	Planning            timetablePorts.InstanceLifecycleDependencies
	Staff               timetablePorts.LifecycleStaff
	Sessions            studentpresence.SessionRecords
	Supervisions        studentpresence.SupervisionRecords
	Presence            timetablePorts.LifecyclePresence
	SessionEnder        timetablePorts.SessionEnder
	CareDays            careplan.CareDayQuery
	CareDayLocks        timetablePorts.CareDayLocks
	GuardianNotices     communication.CareCancellationPublisher
	Settings            TimetableSettings
	Materialization     timetable.MaterializationCapability
	RecurrenceLock      timetable.RecurrenceWriteLock
	StartConflicts      timetable.StartConflictQuery
	SubstituteConflicts timetable.SubstituteConflictQuery
	Broadcaster         realtime.Broadcaster
	Logger              *slog.Logger
	Now                 func() time.Time
	EnforceTimePolicy   bool
}

// TimetableDeviationInputs compose the deviation writes over the retained
// rows (Instances, InstanceStaff and the Audit Platform's DeviationEvents).
// Lifecycle is the instance lifecycle the cancel branch and the
// "deliberately unstaffed" acknowledgement hand over to. The composition root
// adds its database handle to the dependencies they bind.
type TimetableDeviationInputs struct {
	Rows         arrivalTimetable.OwnerRows
	Staff        timetablePorts.DeviationStaff
	Supervisions studentpresence.SupervisionRecords
	Lifecycle    timetablePorts.DeviationLifecycle
	Broadcaster  realtime.Broadcaster
	Logger       *slog.Logger
	Now          func() time.Time
}

// TimetableCorrectionInputs compose the attendance correction over the
// retained rows (Participants, Instances and the optional completion Locks).
// Trail is optional: without it every correction fails closed.
type TimetableCorrectionInputs struct {
	Rows   arrivalTimetable.OwnerRows
	Trail  attendanceCorrectionLedger
	People timetablePorts.CorrectionPeople
	Logger *slog.Logger
}

// TimetableCleanupComposer composes the Timetable owner's GDPR retention
// (WP-B14) over the Settings and Audit collaborators.
type TimetableCleanupComposer func(TimetableRetentionInputs) (timetable.TimetableCleanup, error)

// TimetableRetentionInputs are the collaborators the Timetable owner may not
// name itself: the Settings Platform's retention window and the Audit
// Platform's deletion records. Deviations and Settings are optional.
type TimetableRetentionInputs struct {
	Owner      timetable.Capability
	Deletions  DataDeletionAppender
	Deviations timetableDeviationSweep
	Settings   TimetableRetentionSettings
	Logger     *slog.Logger
	Clock      func() time.Time
}

// TimetableRetentionSettings is the slice of the settings service the
// retention window resolves through.
type TimetableRetentionSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// TimetableSettings is the slice of the settings service the operational
// policies resolve through.
type TimetableSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// TimetableOperationPresence is the Student Presence slice the operational
// day writes visits through.
type TimetableOperationPresence interface {
	CreateVisit(ctx context.Context, visit *studentpresence.Visit) error
	EndVisit(ctx context.Context, id int64) error
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error)
}

// EffectiveArrivalTimes and EffectivePickupTimes are Care Plan's effective
// arrival and pickup times of a day.
type (
	EffectiveArrivalTimes interface {
		GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
	}
	EffectivePickupTimes interface {
		GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*careplan.EffectivePickupTime, error)
	}
)

// TimetableOperationInputs are the retained rows and the owners the
// Timetable operational day reads through (#3551). Broadcaster is optional.
type TimetableOperationInputs struct {
	Rows           arrivalTimetable.OwnerRows
	PlanningTracks timetable.PlanningTrackQuery
	Sessions       studentpresence.SessionRecords
	Supervisions   studentpresence.SupervisionRecords
	Visits         timetablePorts.OperationVisits
	Presence       TimetableOperationPresence
	People         timetablePorts.OperationPeople
	Settings       TimetableSettings
	CareDays       careplan.CareDayQuery
	Arrivals       EffectiveArrivalTimes
	Pickups        EffectivePickupTimes
	Lifecycle      timetablePorts.OperationLifecycle
	Broadcaster    realtime.Broadcaster
	Logger         *slog.Logger
	Now            func() time.Time
}

// TimetableDataInputs are the retained rows and the owners the planner's
// reads of the Timetable owner read through (#3551). ArrivalBaselines and
// PickupBaselines are optional; BlockMetadata is the block display projection
// the composition root binds; Transactional binds the tenant runtime's
// advisory locks a spontaneous start takes.
type TimetableDataInputs struct {
	Rows             arrivalTimetable.OwnerRows
	ArrivalBaselines careplan.ArrivalBaselineReader
	PickupBaselines  careplan.PickupBaselineReader
	Visits           timetablePorts.OperationVisits
	Groups           timetablePorts.DataGroups
	Sessions         studentpresence.SessionRecords
	ConflictAcks     timetable.ConflictAckCapability
	BlockMetadata    timetablePorts.BlockDisplayMetadata
	Transactional    bool
	Logger           *slog.Logger
}

// TimetableMaterializationInputs compose the recurrence engine. CareBounds
// is the per-date care filter (#2487); NonWorkingDays skips holidays and
// closing days (#3594); Broadcaster announces created
// occurrences to the staffing caches (#1844). The composition root adds its
// database handle to the dependencies they bind.
type TimetableMaterializationInputs struct {
	Rows           arrivalTimetable.TemplateRows
	CareBounds     timetablePorts.CareBoundReader
	NonWorkingDays timetable.NonWorkingDayCalendar
	RecurrenceLock timetable.RecurrenceWriteLock
	Broadcaster    realtime.Broadcaster
	Logger         *slog.Logger
}

// TimetableTemplateInputs compose the template writes. Deviations is the
// instance lifecycle's deviation machinery the split preserves
// Vertretungsplan overrides with; CareOfferings and ResyncOfferingRoster are
// Enrollment's. The composition root adds its database handle to the
// dependencies they bind.
type TimetableTemplateInputs struct {
	Rows                 arrivalTimetable.TemplateRows
	PlanningTracks       timetablePorts.PlanningTrackAssignments
	Materialization      timetable.MaterializationCapability
	Deviations           timetablePorts.SeriesDeviations
	CareOfferings        careplan.CareOfferingGuards
	ResyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock       timetable.RecurrenceWriteLock
	Broadcaster          realtime.Broadcaster
	Logger               *slog.Logger
	Today                func() timezone.Date
}

// CalendarDependencies are the values the staff and parent calendars bind
// their narrow fact ports over.
type CalendarDependencies struct {
	CalendarFacts
	Appointments appointments.Capability
	UserContext  identityaccess.CallerIdentities
	// Runtime is the tenant runtime over the root database
	// (calendarCompose.NewTenantRuntime); nil runs without a database.
	Runtime          *calendarCompose.TenantRuntime
	CalendarRenderer calendarService.CalendarRenderer

	// Notification dependencies (all optional — nil disables e-mail; the in-app
	// calendar is unaffected).
	Outbox     CalendarOutbox
	PushOutbox CalendarPushOutbox
	Settings   CalendarLogoResolver
	// CalDAVPolicy decides whether CalDAV is enabled; without it the
	// policy resolves calendar.caldav_enabled through CalDAVSettings, and
	// without both CalDAV has no policy.
	CalDAVPolicy           calendarService.CalDAVPolicy
	CalDAVSettings         CalendarCalDAVSettings
	StaffFeedTombstoneRepo schoolcalendar.FeedHistory
	ParentsURL             string
	FrontendURL            string
	CalDAVURL              string
	// Today pins the feeds' calendar day; nil uses the real day.
	Today func() appointments.Date

	// Notifier and Preferences drive the guardian push/in-app notification that
	// accompanies the appointment e-mails (#1671). Both optional and both
	// required together: without consent there is nobody to address, so a nil
	// Preferences disables the push rather than broadcasting past consent.
	Notifier CalendarNotifier
	// ReminderNotifier waits for Web Push acceptance so the scheduler can retry
	// a transient failure without permanently claiming the reminder delivery.
	ReminderNotifier CalendarReminderNotifier
	Preferences      CalendarPreferences

	// Logger is nil-safe (see service.logger()); notification dispatch is
	// fire-and-forget and reports its failures here instead of to the caller.
	Logger *slog.Logger
}

type CalendarOutbox interface {
	Enqueue(context.Context, calendarCompose.EnqueueRequest) (*calendarCompose.EmailOutbox, error)
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}
type CalendarPushOutbox interface {
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}
type CalendarLogoResolver interface {
	GetLoginImageURL(context.Context, int64) (string, error)
}
type CalendarCalDAVSettings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

// CalendarFacts are the retained rows and owner reads the calendar's fact
// ports read. A nil source leaves its port unset.
type CalendarFacts struct {
	StaffRepo            calendarStaffRows
	StudentRepo          calendarStudentRows
	GuardianProfileRepo  calendarCompose.GuardianDirectory
	StudentGuardianRepo  calendarRelationshipRows
	ChildRepo            ParentChildLinkSource
	GroupRepo            EducationGroupRepository
	InstanceStaffRepo    timetablePorts.CalendarAssignmentRows
	ActivityInstanceRepo timetablePorts.CalendarInstanceRows
	RoomRepo             facilitiesCompose.RoomRows
	StaffShiftRepo       timetablePorts.CalendarShiftRows
	ShiftTypeRepo        timetablePorts.CalendarShiftTypeRows
	SchoolRepo           organizationtenancy.Query
	AccountRepo          identityaccess.ParentCalendarFeeds
	StaffFeedRepo        identityaccess.StaffCalendarFeeds
	PersonRepo           calendarPersonRows
}

// ParentChildLinkSource lists the children a guardian account reaches
// (careplanCompose.ParentChildLinks).
type ParentChildLinkSource interface {
	ListByAccount(context.Context, int64) ([]careplanCompose.ParentChildLink, error)
}

// ReminderSources are the values of this graph the reminder evaluation
// reads: the acting staff member, the tenant settings, the effective
// pickups, the live sessions and the active supervisions.
type ReminderSources struct {
	CurrentStaff CareRequestStaff
	Settings     settingsCompose.SettingsService
	Pickups      EffectivePickupTimes
	Sessions     studentpresence.Presence
	Supervisions studentpresence.SupervisionRecords
	Logger       *slog.Logger
}
