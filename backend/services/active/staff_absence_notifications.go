package active

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/email"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// absenceEmailSettings is the subset of config.SettingsService the absence
// notifications need. Narrow local interface so services/active does not
// depend on services/config.
type absenceEmailSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// AbsenceEmailDeps carries everything the absence email notifications need.
// Wired via setter injection in the factory (mirrors SetShiftPlanSyncer);
// bare-constructed services (unit tests) simply never send.
type AbsenceEmailDeps struct {
	Settings    absenceEmailSettings
	Dispatcher  *email.Dispatcher
	StaffRepo   usersModels.StaffRepository
	DefaultFrom email.Email
	FrontendURL string
	Logger      *slog.Logger
}

// SetAbsenceEmailDeps wires the absence email notifications (#1419 4d).
func (s *staffAbsenceService) SetAbsenceEmailDeps(deps AbsenceEmailDeps) {
	s.emailDeps = &deps
}

func (s *staffAbsenceService) emailLogger() *slog.Logger {
	if s.emailDeps != nil && s.emailDeps.Logger != nil {
		return s.emailDeps.Logger
	}
	return slog.Default()
}

// absenceEmailsEnabled reports whether emails should be sent at all: deps
// wired AND the tenant setting switched on. Errors resolve to "off".
func (s *staffAbsenceService) absenceEmailsEnabled(ctx context.Context) bool {
	if s.emailDeps == nil || s.emailDeps.Dispatcher == nil || s.emailDeps.Settings == nil {
		return false
	}
	enabled, err := s.emailDeps.Settings.ResolveBool(ctx, configModels.KeyNotificationsAbsenceApprovalEmail)
	if err != nil {
		s.emailLogger().Warn("failed to resolve absence email setting, skipping notification",
			"setting_key", configModels.KeyNotificationsAbsenceApprovalEmail,
			"error", err.Error(),
		)
		return false
	}
	return enabled
}

// absenceTypeLabelGerman maps the absence type to its German UI label.
func absenceTypeLabelGerman(absenceType string) string {
	switch absenceType {
	case activeModels.AbsenceTypeSick:
		return "Krankmeldung"
	case activeModels.AbsenceTypeVacation:
		return "Urlaub"
	case activeModels.AbsenceTypeTraining:
		return "Fortbildung"
	default:
		return "Sonstige Abwesenheit"
	}
}

func formatAbsenceDateRange(a *activeModels.StaffAbsence) string {
	if a.DateStart == a.DateEnd {
		return a.DateStart.Format("02.01.2006")
	}
	return a.DateStart.Format("02.01.2006") + " bis " + a.DateEnd.Format("02.01.2006")
}

// notifyAbsenceRequested emails every staff member with vacation:approve that
// a new request arrived (#1419 4d). Called after the request row is created;
// failures only log — email must never block the workflow.
func (s *staffAbsenceService) notifyAbsenceRequested(ctx context.Context, absence *activeModels.StaffAbsence) {
	if !s.absenceEmailsEnabled(ctx) {
		return
	}
	requester, err := s.emailDeps.StaffRepo.GetStaffContactInfo(ctx, absence.StaffID)
	if err != nil {
		s.emailLogger().Warn("failed to load requester for absence email",
			"absence_id", absence.ID,
			"staff_id", absence.StaffID,
			"error", err.Error(),
		)
		return
	}
	approvers, err := s.emailDeps.StaffRepo.ListStaffWithPermission(ctx, permissions.VacationApprove)
	if err != nil {
		s.emailLogger().Warn("failed to load approvers for absence email",
			"absence_id", absence.ID,
			"error", err.Error(),
		)
		return
	}
	if len(approvers) == 0 {
		s.emailLogger().Warn("no staff with vacation:approve found, absence request email not sent",
			"absence_id", absence.ID,
		)
		return
	}
	requesterName := requester.FirstName + " " + requester.LastName
	for _, approver := range approvers {
		if approver.Email == "" || approver.StaffID == absence.StaffID {
			continue
		}
		s.dispatchAbsenceEmail(ctx, "absence_request_received", absence, email.Message{
			From:     s.emailDeps.DefaultFrom,
			To:       email.NewEmail(approver.FirstName+" "+approver.LastName, approver.Email),
			Subject:  "Neuer Abwesenheitsantrag von " + requesterName,
			Template: "absence-request-received.html",
			Content: map[string]any{
				"FirstName":        approver.FirstName,
				"LastName":         approver.LastName,
				"RequesterName":    requesterName,
				"AbsenceTypeLabel": absenceTypeLabelGerman(absence.AbsenceType),
				"DateRange":        formatAbsenceDateRange(absence),
				"Note":             absence.Note,
				"LinkURL":          s.emailDeps.FrontendURL + "/staff",
				"LogoURL":          s.logoURL(),
			},
		}, approver.Email)
	}
}

// notifyAbsenceDecision emails the requesting staff member about an approve /
// decline / Rückfrage on their request (#1419 4d).
func (s *staffAbsenceService) notifyAbsenceDecision(ctx context.Context, absence *activeModels.StaffAbsence) {
	if !s.absenceEmailsEnabled(ctx) {
		return
	}
	var subject, template, metaType string
	switch absence.Status {
	case activeModels.AbsenceStatusApproved:
		subject, template, metaType = "Dein Abwesenheitsantrag wurde genehmigt", "absence-request-approved.html", "absence_request_approved"
	case activeModels.AbsenceStatusDeclined:
		subject, template, metaType = "Dein Abwesenheitsantrag wurde abgelehnt", "absence-request-declined.html", "absence_request_declined"
	case activeModels.AbsenceStatusQuestion:
		subject, template, metaType = "Rückfrage zu deinem Abwesenheitsantrag", "absence-request-question.html", "absence_request_question"
	default:
		return
	}
	requester, err := s.emailDeps.StaffRepo.GetStaffContactInfo(ctx, absence.StaffID)
	if err != nil {
		s.emailLogger().Warn("failed to load requester for absence decision email",
			"absence_id", absence.ID,
			"staff_id", absence.StaffID,
			"error", err.Error(),
		)
		return
	}
	if requester.Email == "" {
		return
	}
	s.dispatchAbsenceEmail(ctx, metaType, absence, email.Message{
		From:     s.emailDeps.DefaultFrom,
		To:       email.NewEmail(requester.FirstName+" "+requester.LastName, requester.Email),
		Subject:  subject,
		Template: template,
		Content: map[string]any{
			"FirstName":        requester.FirstName,
			"LastName":         requester.LastName,
			"AbsenceTypeLabel": absenceTypeLabelGerman(absence.AbsenceType),
			"DateRange":        formatAbsenceDateRange(absence),
			"DecisionNote":     absence.DecisionNote,
			"LinkURL":          s.emailDeps.FrontendURL + "/time-tracking",
			"LogoURL":          s.logoURL(),
		},
	}, requester.Email)
}

func (s *staffAbsenceService) logoURL() string {
	return fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.emailDeps.FrontendURL)
}

func (s *staffAbsenceService) dispatchAbsenceEmail(ctx context.Context, metaType string, absence *activeModels.StaffAbsence, message email.Message, recipient string) {
	s.emailDeps.Dispatcher.Dispatch(ctx, email.DeliveryRequest{
		Message: message,
		Metadata: email.DeliveryMetadata{
			Type:        metaType,
			ReferenceID: absence.ID,
			Recipient:   recipient,
		},
	})
}
