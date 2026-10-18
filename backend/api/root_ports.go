package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/observability"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

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

// pickupStudentLocks takes the People Directory student lock a weekly pickup
// approval holds. A student that no longer exists reports not found.
type pickupStudentLocks struct {
	students interface {
		LockStudent(ctx context.Context, studentID int64) error
	}
}

func (l pickupStudentLocks) LockStudent(ctx context.Context, studentID int64) (bool, error) {
	err := l.students.LockStudent(ctx, studentID)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return false, nil
	}
	return err == nil, err
}

// observeCarePlan records the excused-absence workflow as Care Plan
// operations.
func observeCarePlan(observation carePlanCompose.Observation) {
	observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, careplan.ErrorCode(observation.Err), observation.Err)
}

// enrollmentPhaseRestrictions are Enrollment's reads of the active phases
// that restrict eligibility.
type enrollmentPhaseRestrictions interface {
	HasActiveClassRestrictedPhase(ctx context.Context) (bool, error)
	HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error)
	MaxActivePhaseGrade(ctx context.Context) (int, error)
}

// newEnrollmentFormSchemas builds the form schema administration where the
// root mounts it, over the collection settings (#4012).
func newEnrollmentFormSchemas(settings enrollmentSettingsReads, logger *slog.Logger) enrollmentOwner.FormSchemaAdministration {
	return enrollmentCompose.NewFormSchemas(NewEnrollmentBookings(), enrollmentCollectionSettings{settings: settings}, logger.With("service", "enrollment-form-schema"))
}

// newEnrollmentCaptcha builds the captcha verifier of the public enrollment
// form where the root mounts it.
func newEnrollmentCaptcha(settings enrollmentSettingsReads, logger *slog.Logger) enrollmentOwner.CaptchaVerifier {
	return enrollmentCompose.NewCaptcha(enrollmentCaptchaSettings{settings: settings}, "", logger.With("service", "enrollment-captcha"))
}

// newEnrollmentPhaseExpiry builds the phase expiry warnings over the People
// Directory's enrolled students and Care Plan's offerings.
func newEnrollmentPhaseExpiry(students peopledirectory.StudentQuery, carePlan careplan.Query) enrollmentOwner.PhaseExpiryWarnings {
	bookings := NewEnrollmentBookings()
	return enrollmentCompose.NewPhaseExpiryWarnings(enrollmentCompose.NewPhaseExpirySnapshots(
		bookings, phaseExpiryStudents{query: students}, phaseExpiryCarePlanDirectory{query: carePlan}, bookings,
	))
}

// operatorFrontendURLOf is the operator portal host the invitation mails
// link to. The operator subdomain is separate from FRONTEND_URL, so the link
// goes straight to the operator host: a cross-origin redirect hop is what
// mail content scanners treat as a phishing signal.
func operatorFrontendURLOf(operatorHostname, frontendURL string) (string, error) {
	if operatorHostname == "" {
		return "", fmt.Errorf("NEXT_PUBLIC_OPERATOR_HOSTNAME is required")
	}
	protocol := "http"
	if strings.HasPrefix(frontendURL, "https://") {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s", protocol, strings.TrimRight(operatorHostname, "/")), nil
}

// emailChangeExpiry is the owner's lifetime of an operator e-mail change
// link; the root reports when the owner raised the configured value.
func emailChangeExpiry(passwordResetExpiry time.Duration, logger *slog.Logger) time.Duration {
	expiry := identityaccess.OperatorEmailChangeExpiry(passwordResetExpiry)
	if expiry != passwordResetExpiry {
		logger.Warn("email change token expiry bumped to minimum 15 minutes",
			slog.Int("configured_minutes", int(passwordResetExpiry.Minutes())),
			slog.Int("effective_minutes", int(expiry.Minutes())),
		)
	}
	return expiry
}

// payloadRenderer renders a queued mail from its payload alone.
func payloadRenderer(render func(context.Context, map[string]any) (*email.Message, error)) deliveryCompose.MailRendererFunc {
	return func(ctx context.Context, intent *deliveryCompose.MailIntent) (*email.Message, error) {
		return render(ctx, intent.Payload)
	}
}

// precedingMails reports Delivery's state of a queued mail in Identity's
// terms (#4006).
type precedingMails struct {
	status func(ctx context.Context, tenantID, id int64) (deliveryModule.State, bool, error)
}

func (p precedingMails) PrecedingMailState(ctx context.Context, tenantID, outboxID int64) (identityaccessCompose.PrecedingMailState, bool, error) {
	state, found, err := p.status(ctx, tenantID, outboxID)
	if err != nil || !found {
		return identityaccessCompose.PrecedingMailQueued, found, err
	}
	switch state {
	case deliveryModule.StateSent:
		return identityaccessCompose.PrecedingMailSent, true, nil
	case deliveryModule.StateCancelled:
		return identityaccessCompose.PrecedingMailCancelled, true, nil
	case deliveryModule.StateDeadLetter:
		return identityaccessCompose.PrecedingMailUndeliverable, true, nil
	default:
		return identityaccessCompose.PrecedingMailQueued, true, nil
	}
}

// requiredLink trims a configured base URL and refuses an empty one, and in
// production one without https.
func requiredLink(name, raw string, production bool) (string, error) {
	value := strings.TrimRight(raw, "/")
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if production && !strings.HasPrefix(value, "https://") {
		return "", fmt.Errorf("%s must use https:// in production (received %q)", name, raw)
	}
	return value, nil
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
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error)
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
	CurrentStates(ctx context.Context, student *userModels.Student, canManagePhoto bool) ([]userModels.StudentConsentState, error)
	RecordTransitions(ctx context.Context, before, after *userModels.Student, source string, actorAccountID *int64, changedAt time.Time) error
	CurrentStudentConsents(ctx context.Context, snapshot peopledirectory.StudentConsentSnapshot, canManagePhoto bool) ([]peopledirectory.StudentConsentState, error)
	RecordStudentConsentTransitions(ctx context.Context, before, after peopledirectory.StudentConsentSnapshot, source string, actorAccountID *int64, changedAt time.Time) error
}

// GuardianDisplayQuery names the guardians of the Delivery recipients.
type GuardianDisplayQuery interface {
	GuardianDisplays(context.Context, []int64) ([]peoplePorts.GuardianDisplay, error)
}

// TimetableDeviationTrail is the Audit Platform's Änderungsprotokoll as the
// Timetable owner rows read and append it.
type TimetableDeviationTrail interface {
	Create(ctx context.Context, event *auditModels.DeviationEvent) error
	ListByRange(ctx context.Context, from, to auditModels.Date, activityGroupID *int64, startTime *string) ([]*auditModels.DeviationEvent, error)
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

type CalendarOutbox interface {
	Enqueue(context.Context, calendarCompose.EnqueueRequest) (*calendarCompose.EmailOutbox, error)
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}

type CalendarCalDAVSettings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

// ParentChildLinkSource lists the children a guardian account reaches
// (careplanCompose.ParentChildLinks).
type ParentChildLinkSource interface {
	ListByAccount(context.Context, int64) ([]careplanCompose.ParentChildLink, error)
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
