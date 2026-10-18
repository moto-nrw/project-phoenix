package api

import (
	"context"
	"strings"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The bindings below compose Enrollment's decision flow (#3564) over the
// Enrollment owner, the People Directory rows of the retained repositories,
// the Audit Platform trails, the Settings Platform, Care Plan's booking
// materialization and the realtime hub.

// NewEnrollmentDecisions composes Enrollment's decision flow over deps. The
// caller binds the Enrollment owner's ports and Care Plan's, Timetable's and
// Identity & Access's values into deps directly and the People Directory
// rows, the Audit Platform trails, the realtime hub and the settings through
// the EnrollmentDecision* adapters below; an absent adapter source leaves its
// port unbound, the way the decision flow always treated an unwired
// repository. The People Directory's rules and refusals are bound here, and
// the Enrollment owner behind deps.Requests also keeps the per-account read
// state of the queue (#3778).
func NewEnrollmentDecisions(deps enrollmentCompose.DecisionDependencies) *enrollmentCompose.Decisions {
	deps.People.Rules = enrollmentPeopleRules{}
	deps.People.ErrGuardianLinkNotFound = peopleValues.ErrStudentGuardianNotFound
	deps.People.ErrCompanionWouldLoseDeparture = peopleValues.TranslateStudentWriteError(peopleValues.ErrCompanionWouldLoseDeparture)
	deps.People.ErrCompanionLockBusy = peopleValues.TranslateStudentWriteError(peopleValues.ErrCompanionLockBusy)
	deps.People.IsLockNotAvailable = isPostgresLockNotAvailable
	if reads, ok := deps.Requests.(enrollmentOwner.RequestReads); ok {
		deps.Reads = reads
	}
	return enrollmentCompose.NewDecisions(deps)
}

// EnrollmentDecisionParentsURL is the base of the status links in the parent
// mails: parentsURL, falling back to frontendURL when empty.
func EnrollmentDecisionParentsURL(parentsURL, frontendURL string) string {
	base := strings.TrimRight(strings.TrimSpace(parentsURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	}
	return base
}

// EnrollmentDecisionPeopleDirectory binds the decision flow's People
// Directory ports to the retained person, staff, student, guardian link,
// guardian profile and phone rows; people resolves the accounts and links
// behind them. A nil row source leaves its port unbound.
func EnrollmentDecisionPeopleDirectory(
	persons enrollmentPersonRows, staff enrollmentStaffRows, students enrollmentStudentRows,
	studentGuardians enrollmentStudentGuardianRows, guardianProfiles enrollmentGuardianProfileRows,
	guardianPhones enrollmentGuardianPhoneRows, people EnrollmentDecisionPeople,
) enrollmentCompose.PeopleDirectory {
	var directory enrollmentCompose.PeopleDirectory
	if persons != nil {
		directory.Persons = enrollmentPersons{repo: persons, people: people}
	}
	if staff != nil {
		directory.Staff = enrollmentReviewerStaff{repo: staff}
	}
	if students != nil {
		directory.Students = enrollmentStudentRecords{repo: students}
	}
	if guardianProfiles != nil {
		directory.GuardianProfiles = enrollmentGuardianProfiles{repo: guardianProfiles, accounts: people}
	}
	if studentGuardians != nil {
		directory.StudentGuardians = enrollmentStudentGuardians{repo: studentGuardians, people: people}
	}
	if guardianPhones != nil {
		directory.GuardianPhones = enrollmentGuardianPhones{repo: guardianPhones}
	}
	return directory
}

// EnrollmentDecisionCompanions binds Care Plan's "läuft mit" edges and their
// deletion; without either the port stays unbound.
func EnrollmentDecisionCompanions(edges EnrollmentDepartureCompanions, deleteEdges func(context.Context, []int64) error) enrollmentCompose.DepartureCompanions {
	if edges == nil || deleteEdges == nil {
		return nil
	}
	return enrollmentDepartureCompanions{edges: edges, delete: deleteEdges}
}

// EnrollmentDecisionBroadcasts announces student_updated and
// student_companions_changed on the realtime hub after an approved
// enrollment sync trimmed a child's "läuft mit" links; nil without a hub.
func EnrollmentDecisionBroadcasts(hub realtime.Broadcaster) enrollmentCompose.StudentPlanBroadcasts {
	if hub == nil {
		return nil
	}
	return enrollmentPlanBroadcasts{hub: hub}
}

// EnrollmentDecisionSettingValues resolves the decision flow's tenant
// settings; nil without settings.
func EnrollmentDecisionSettingValues(settings EnrollmentDecisionSettings) enrollmentCompose.DecisionSettings {
	if settings == nil {
		return nil
	}
	return enrollmentDecisionSettings{settings: settings}
}

// The Audit Platform trails of the decision flow; an absent source leaves
// the port unbound.

// EnrollmentDecisionAccessLog records the exports of the decision flow.
func EnrollmentDecisionAccessLog(repo enrollmentAccessLogAppender) enrollmentCompose.ExportAccessLog {
	if repo == nil {
		return nil
	}
	return enrollmentExportAccessLog{repo: repo}
}

// EnrollmentDecisionAdjustments lists a child's audited offering adjustments.
func EnrollmentDecisionAdjustments(repo enrollmentAdjustmentRows) enrollmentCompose.OfferingAdjustmentTrail {
	if repo == nil {
		return nil
	}
	return enrollmentOfferingAdjustmentTrail{repo: repo}
}

// EnrollmentDecisionRestorations appends the trail of an admin restore.
func EnrollmentDecisionRestorations(repo enrollmentRestorationAppender) enrollmentCompose.RestorationAudit {
	if repo == nil {
		return nil
	}
	return enrollmentRestorationAudit{repo: repo}
}

// EnrollmentDecisionPayers appends payer changes to the guardian financial
// change ledger.
func EnrollmentDecisionPayers(ledger auditModels.GuardianFinancialChangeCreator) enrollmentCompose.PayerAudit {
	if ledger == nil {
		return nil
	}
	return enrollmentPayerAudit{ledger: ledger}
}

// EnrollmentDecisionStudentAudit records the tracked profile changes of an
// approval.
func EnrollmentDecisionStudentAudit(auditor EnrollmentStudentAuditor) enrollmentCompose.StudentAudit {
	if auditor == nil {
		return nil
	}
	return enrollmentStudentAudit{auditor: auditor}
}

// EnrollmentDecisionStudentConsents records the consent transitions of an
// approval.
func EnrollmentDecisionStudentConsents(auditor EnrollmentConsentAuditor) enrollmentCompose.StudentConsents {
	if auditor == nil {
		return nil
	}
	return enrollmentStudentConsents{auditor: auditor}
}

// enrollmentDecisionSettings resolves the decision flow's tenant settings.
type enrollmentDecisionSettings struct {
	settings EnrollmentDecisionSettings
}

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
	repo enrollmentAdjustmentRows
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
	repo enrollmentRestorationAppender
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
type enrollmentStudentConsents struct {
	auditor EnrollmentConsentAuditor
}

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

// EnrollmentDecisionSettings resolves the tenant settings of the decision
// flow.
type EnrollmentDecisionSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// EnrollmentStudentAuditor records tracked profile changes made while
// approving or synchronizing enrollment data for an existing student.
type EnrollmentStudentAuditor interface {
	RecordChangesForActor(ctx context.Context, before, after *peopleValues.StudentState, editedBy int64) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
}

// EnrollmentConsentAuditor records effective consent transitions made while an
// enrollment submission is applied to a student.
type EnrollmentConsentAuditor interface {
	RecordTransitions(ctx context.Context, before, after *peopleValues.StudentState, source string, actorAccountID *int64, changedAt time.Time) error
}

// EnrollmentDepartureCompanions reads Care Plan's "läuft mit" edges of a
// student.
type EnrollmentDepartureCompanions interface {
	ListForStudent(context.Context, int64) ([]*careplan.CompanionEdge, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
}
