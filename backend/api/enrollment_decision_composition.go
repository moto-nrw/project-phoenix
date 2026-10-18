package api

import (
	"context"
	"strings"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The bindings below compose Enrollment's decision flow (#3564) over the
// Enrollment owner, the People Directory rows of the retained repositories,
// the Audit Platform trails, the Settings Platform, Care Plan's booking
// materialization and the realtime hub.

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
	// The Enrollment owner behind Requests also keeps the per-account read
	// state of the queue (#3778).
	if reads, ok := src.Requests.(enrollmentOwner.RequestReads); ok {
		deps.Reads = reads
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
		ErrGuardianLinkNotFound:        peopleValues.ErrStudentGuardianNotFound,
		ErrCompanionWouldLoseDeparture: peopleValues.TranslateStudentWriteError(peopleValues.ErrCompanionWouldLoseDeparture),
		ErrCompanionLockBusy:           peopleValues.TranslateStudentWriteError(peopleValues.ErrCompanionLockBusy),
		IsLockNotAvailable:             isPostgresLockNotAvailable,
	}
	if src.Persons != nil {
		people.Persons = enrollmentPersons{repo: src.Persons, people: src.StudentEnrollment}
	}
	if src.Staff != nil {
		people.Staff = enrollmentReviewerStaff{repo: src.Staff}
	}
	if src.Students != nil {
		people.Students = enrollmentStudentRecords{repo: src.Students}
	}
	if src.GuardianProfiles != nil {
		people.GuardianProfiles = enrollmentGuardianProfiles{repo: src.GuardianProfiles, accounts: src.StudentEnrollment}
	}
	if src.StudentGuardians != nil {
		people.StudentGuardians = enrollmentStudentGuardians{repo: src.StudentGuardians, people: src.StudentEnrollment}
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
