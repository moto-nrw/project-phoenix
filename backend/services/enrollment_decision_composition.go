package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below compose Enrollment's decision flow (#3564) over the
// Enrollment owner, the People Directory rows of the retained repositories,
// the Audit Platform trails, the Settings Platform, Care Plan's booking
// materialization and the realtime hub.

// EnrollmentDecisionSettings resolves the tenant settings of the decision
// flow.
type EnrollmentDecisionSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// EnrollmentStudentAuditor records tracked profile changes made while
// approving or synchronizing enrollment data for an existing student.
type EnrollmentStudentAuditor interface {
	RecordChangesForActor(ctx context.Context, before, after *userModels.Student, editedBy int64) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after userModels.StudentStatus) error
}

// EnrollmentConsentAuditor records effective consent transitions made while an
// enrollment submission is applied to a student.
type EnrollmentConsentAuditor interface {
	RecordTransitions(ctx context.Context, before, after *userModels.Student, source string, actorAccountID *int64, changedAt time.Time) error
}

// EnrollmentDepartureCompanions reads Care Plan's "läuft mit" edges of a
// student.
type EnrollmentDepartureCompanions interface {
	ListForStudent(context.Context, int64) ([]*userModels.StudentCompanion, error)
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
	DataAccessLog          auditModels.DataAccessLogRepository
	OfferingAdjustments    auditModels.EnrollmentOfferingAdjustmentRepository
	Restorations           auditModels.EnrollmentRestorationRepository
	Notifications          enrollmentOwner.Notifications
	Persons                userModels.PersonRepository
	Staff                  userModels.StaffRepository
	Students               userModels.StudentRepository
	StudentEnrollment      enrollmentCompose.DecisionStudentEnrollment
	Companions             EnrollmentDepartureCompanions
	DeleteCompanions       func(context.Context, []int64) error
	StudentGuardians       userModels.StudentGuardianRepository
	GuardianFinancialAudit auditModels.GuardianFinancialChangeCreator
	GuardianProfiles       userModels.GuardianProfileRepository
	GuardianPhones         userModels.GuardianPhoneNumberRepository
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
	Today                  func() calendar.Date
}

// NewEnrollmentDecisions composes Enrollment's decision flow over the sources.
func NewEnrollmentDecisions(src EnrollmentDecisionSources) *enrollmentCompose.Decisions {
	parentsURL := strings.TrimRight(strings.TrimSpace(src.ParentsURL), "/")
	if parentsURL == "" {
		parentsURL = strings.TrimRight(strings.TrimSpace(src.FrontendURL), "/")
	}
	deps := enrollmentCompose.DecisionDependencies{
		Requests: src.Requests, Children: src.Children, Guardians: src.Guardians,
		LateInvites: src.LateInvites, Phases: src.Phases, Schemas: src.Schemas, Offerings: src.CareOfferings,
		Notifications: src.Notifications, Bookings: src.CareBookings, GuardianAccess: src.GuardianAccess,
		StudentEnrollment: src.StudentEnrollment, People: enrollmentPeopleDirectory(src),
		PickupSchedules: src.PickupSchedules, ArrivalSchedules: src.ArrivalSchedules,
		CareWithdrawal: src.CareWithdrawal, LockTemplateRecurrence: src.LockTemplateRecurrence,
		Pickups: src.Pickups, ParentsURL: parentsURL, Logger: src.Logger, Today: src.Today,
		AccessLog: decisionAccessLog(src), Adjustments: decisionAdjustmentTrail(src),
		Restorations: decisionRestorationAudit(src), Payers: decisionPayerAudit(src),
		StudentAudit: decisionStudentAudit(src), StudentConsents: decisionStudentConsents(src),
	}
	if src.Companions != nil && src.DeleteCompanions != nil {
		deps.Companions = enrollmentDepartureCompanions{edges: src.Companions, delete: src.DeleteCompanions}
	}
	if src.Broadcaster != nil {
		deps.Broadcasts = enrollmentPlanBroadcasts{hub: src.Broadcaster}
	}
	if src.Settings != nil {
		deps.Settings = enrollmentDecisionSettings{settings: src.Settings}
	}
	return enrollmentCompose.NewDecisions(deps)
}

// enrollmentPeopleDirectory binds the People Directory ports the sources
// carry.
func enrollmentPeopleDirectory(src EnrollmentDecisionSources) enrollmentCompose.PeopleDirectory {
	people := enrollmentCompose.PeopleDirectory{
		Rules:                          enrollmentPeopleRules{},
		ErrGuardianLinkNotFound:        userModels.ErrStudentGuardianNotFound,
		ErrCompanionWouldLoseDeparture: userModels.ErrCompanionWouldLoseDeparture,
		ErrCompanionLockBusy:           userModels.ErrCompanionLockBusy,
		IsLockNotAvailable:             isPostgresLockNotAvailable,
	}
	if src.Persons != nil {
		people.Persons = enrollmentPersons{repo: src.Persons}
	}
	if src.Staff != nil {
		people.Staff = enrollmentReviewerStaff{repo: src.Staff}
	}
	if src.Students != nil {
		people.Students = enrollmentStudentRecords{repo: src.Students}
	}
	if src.GuardianProfiles != nil {
		people.GuardianProfiles = enrollmentGuardianProfiles{repo: src.GuardianProfiles}
	}
	if src.StudentGuardians != nil {
		people.StudentGuardians = enrollmentStudentGuardians{repo: src.StudentGuardians}
	}
	if src.GuardianPhones != nil {
		people.GuardianPhones = enrollmentGuardianPhones{repo: src.GuardianPhones}
	}
	return people
}

// The Audit Platform trails the sources carry; an absent source leaves the
// port unbound.

func decisionAccessLog(src EnrollmentDecisionSources) enrollmentCompose.ExportAccessLog {
	if src.DataAccessLog == nil {
		return nil
	}
	return enrollmentExportAccessLog{repo: src.DataAccessLog}
}

func decisionAdjustmentTrail(src EnrollmentDecisionSources) enrollmentCompose.OfferingAdjustmentTrail {
	if src.OfferingAdjustments == nil {
		return nil
	}
	return enrollmentOfferingAdjustmentTrail{repo: src.OfferingAdjustments}
}

func decisionRestorationAudit(src EnrollmentDecisionSources) enrollmentCompose.RestorationAudit {
	if src.Restorations == nil {
		return nil
	}
	return enrollmentRestorationAudit{repo: src.Restorations}
}

func decisionPayerAudit(src EnrollmentDecisionSources) enrollmentCompose.PayerAudit {
	if src.GuardianFinancialAudit == nil {
		return nil
	}
	return enrollmentPayerAudit{ledger: src.GuardianFinancialAudit}
}

func decisionStudentAudit(src EnrollmentDecisionSources) enrollmentCompose.StudentAudit {
	if src.StudentAudit == nil {
		return nil
	}
	return enrollmentStudentAudit{auditor: src.StudentAudit}
}

func decisionStudentConsents(src EnrollmentDecisionSources) enrollmentCompose.StudentConsents {
	if src.StudentConsents == nil {
		return nil
	}
	return enrollmentStudentConsents{auditor: src.StudentConsents}
}

// enrollmentDecisionSettings resolves the decision flow's tenant settings.
type enrollmentDecisionSettings struct{ settings EnrollmentDecisionSettings }

func (s enrollmentDecisionSettings) WaitlistEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentWaitlistEnabled)
}

func (s enrollmentDecisionSettings) AutoInviteGuardianOnApprove(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentAutoInviteGuardianOnApprove)
}

func (s enrollmentDecisionSettings) CareOfferingsEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCareOfferingsEnabled)
}

func (s enrollmentDecisionSettings) DefaultActivationMode(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModels.KeyEnrollmentDefaultActivationMode)
}

// enrollmentOfferingAdjustmentTrail lists a child's audited offering
// adjustments.
type enrollmentOfferingAdjustmentTrail struct {
	repo auditModels.EnrollmentOfferingAdjustmentRepository
}

func (t enrollmentOfferingAdjustmentTrail) ListOfferingAdjustments(ctx context.Context, requestChildID int64) ([]*enrollmentOwner.OfferingAdjustmentRecord, error) {
	rows, err := t.repo.ListByRequestChildID(ctx, requestChildID)
	if err != nil {
		return nil, err
	}
	records := make([]*enrollmentOwner.OfferingAdjustmentRecord, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			records = append(records, nil)
			continue
		}
		records = append(records, &enrollmentOwner.OfferingAdjustmentRecord{
			ID: row.ID, TenantID: row.TenantID, RequestID: row.RequestID, RequestChildID: row.RequestChildID,
			StudentID: row.StudentID, ActorAccountID: row.ActorAccountID, ActorRole: row.ActorRole,
			ActorNameSnapshot: row.ActorNameSnapshot, ActorEmailSnapshot: row.ActorEmailSnapshot,
			Reason: row.Reason, Source: row.Source, Before: row.Before, After: row.After,
			CompleteWithdrawalConfirmed: row.CompleteWithdrawalConfirmed, ChangedAt: row.ChangedAt,
		})
	}
	return records, nil
}

// enrollmentRestorationAudit appends the trail of an admin restore.
type enrollmentRestorationAudit struct {
	repo auditModels.EnrollmentRestorationRepository
}

func (a enrollmentRestorationAudit) RecordRestoration(ctx context.Context, restoration enrollmentCompose.Restoration) error {
	return a.repo.Create(ctx, &auditModels.EnrollmentRestoration{
		RequestID:      restoration.RequestID,
		ChildIDs:       restoration.ChildIDs,
		ActorAccountID: restoration.ActorAccountID,
		RestoredAt:     restoration.RestoredAt,
	})
}

// enrollmentPayerAudit appends payer changes to the guardian financial change
// ledger.
type enrollmentPayerAudit struct {
	ledger auditModels.GuardianFinancialChangeCreator
}

func (a enrollmentPayerAudit) RecordPayerChange(ctx context.Context, change enrollmentCompose.PayerChange) error {
	studentID := change.StudentID
	return a.ledger.Create(ctx, &auditModels.GuardianFinancialChange{
		GuardianProfileID: change.GuardianProfileID,
		StudentID:         &studentID,
		ChangedBy:         change.ChangedBy,
		FieldName:         auditModels.GuardianPaymentFieldIsPayer,
		OldValue:          change.OldValue,
		NewValue:          change.NewValue,
		Note:              change.Note,
	})
}

// enrollmentStudentConsents records the consent transitions of an
// enrollment approval.
type enrollmentStudentConsents struct{ auditor EnrollmentConsentAuditor }

func (c enrollmentStudentConsents) RecordEnrollmentConsentTransitions(ctx context.Context, before, after *enrollmentCompose.Student, actorAccountID *int64, changedAt time.Time) error {
	return c.auditor.RecordTransitions(ctx, decisionStudentRow(before), decisionStudentRow(after), auditModels.StudentConsentSourceEnrollment, actorAccountID, changedAt)
}

// enrollmentPlanBroadcasts announces a synced departure plan on the realtime
// hub.
type enrollmentPlanBroadcasts struct{ hub realtime.Broadcaster }

func (b enrollmentPlanBroadcasts) BroadcastStudentUpdated(tenantID int64, source string) error {
	return b.hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source}))
}

func (b enrollmentPlanBroadcasts) BroadcastStudentCompanionsChanged(tenantID int64, source string) error {
	return b.hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source}))
}
