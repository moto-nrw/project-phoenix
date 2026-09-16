package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"

	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const absenceEmailDateLayout = "02.01.2006"

// absenceEmailSettingResolver is the subset of config.SettingsService the absence
// notifications need. Narrow local interface so this package does not
// depend on services/config.
// absenceEmailSettingResolver answers the one tenant setting the absence
// notifications consult. It names the question rather than taking a registry
// key, so this package does not carry the settings vocabulary of another owner.
type absenceEmailSettingResolver interface {
	AbsenceApprovalEmailEnabled(ctx context.Context) (bool, error)
}

type absenceEmailSchoolFinder interface {
	FindSchoolSubdomain(ctx context.Context, id int64) (subdomain string, found bool, err error)
}

type absenceEmailStaffDirectory interface {
	GetStaffContactInfo(context.Context, int64) (*AbsenceEmailContact, error)
	ListAbsenceApprovers(context.Context) ([]*AbsenceEmailContact, error)
}

// AbsenceEmailContact contains only notification addressing and identity.
type AbsenceEmailContact struct {
	StaffID   int64
	FirstName string
	LastName  string
	Email     string
}

// AbsenceEmailDeps carries everything the absence email notifications need.
// Bound by the factory through WithAbsenceEmail;
// bare-constructed services (unit tests) simply never send.
type AbsenceEmailDeps struct {
	Settings    absenceEmailSettingResolver
	Dispatcher  absenceEmailDispatcher
	StaffRepo   absenceEmailStaffDirectory
	SchoolRepo  absenceEmailSchoolFinder
	FrontendURL string
	Logger      *slog.Logger
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
	if s.emailDeps == nil ||
		s.emailDeps.Dispatcher == nil ||
		s.emailDeps.Settings == nil ||
		s.emailDeps.StaffRepo == nil ||
		s.emailDeps.SchoolRepo == nil {
		return false
	}
	enabled, err := s.emailDeps.Settings.AbsenceApprovalEmailEnabled(ctx)
	if err != nil {
		s.emailLogger().Warn("failed to resolve absence email setting, skipping notification",
			"setting", "absence_approval_email",
			"error", err.Error(),
		)
		return false
	}
	return enabled
}

// absenceEmailTypeLabel is the wording an absence email uses. The school's own
// Abwesenheitsart wins when the row carries one (#2403) — the mail should read
// "Regenerationstag", not the generic "Sonstige Abwesenheit" the base type
// resolves to.
func absenceEmailTypeLabel(a *activeModels.StaffAbsence) string {
	if a.AbsenceTypeLabel != "" {
		return a.AbsenceTypeLabel
	}
	return absenceTypeLabelGerman(a.AbsenceType)
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
	case activeModels.AbsenceTypeCompTime:
		return "Freizeitausgleich"
	default:
		return "Sonstige Abwesenheit"
	}
}

func formatAbsenceDateRange(a *activeModels.StaffAbsence) string {
	if a.DateStart == a.DateEnd {
		return a.DateStart.Format(absenceEmailDateLayout)
	}
	return a.DateStart.Format(absenceEmailDateLayout) + " bis " + a.DateEnd.Format(absenceEmailDateLayout)
}

// notifyAbsenceRequested emails every staff member with vacation:approve that
// a new request arrived (#1419 4d). Called after the request row is created;
// failures only log — email must never block the workflow.
func (s *staffAbsenceService) notifyAbsenceRequested(ctx context.Context, absence *activeModels.StaffAbsence) {
	StampAbsenceTypeLabels(ctx, s.absenceTypes, []*activeModels.StaffAbsence{absence}, s.getLogger())
	if !s.absenceEmailsEnabled(ctx) {
		return
	}
	linkURL, ok := s.absenceEmailLink(ctx, absence, "/staff")
	if !ok {
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
	approvers, err := s.emailDeps.StaffRepo.ListAbsenceApprovers(ctx)
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
	previousQuestion := strings.TrimSpace(absence.DecisionNote)
	subject := "Neuer Abwesenheitsantrag von " + requesterName
	if previousQuestion != "" {
		subject = "Abwesenheitsantrag erneut eingereicht von " + requesterName
	}
	for _, approver := range approvers {
		if approver.Email == "" || approver.StaffID == absence.StaffID {
			continue
		}
		s.dispatchAbsenceEmail(ctx, "absence_request_received", absence, AbsenceEmailMessage{
			To:       AbsenceEmailAddress{Name: approver.FirstName + " " + approver.LastName, Address: approver.Email},
			Subject:  subject,
			Template: "absence-request-received.html",
			Content: AbsenceEmailContent{
				FirstName:        approver.FirstName,
				LastName:         approver.LastName,
				RequesterName:    requesterName,
				AbsenceTypeLabel: absenceEmailTypeLabel(absence),
				DateRange:        formatAbsenceDateRange(absence),
				Note:             absence.Note,
				PreviousQuestion: previousQuestion,
				LinkURL:          linkURL,
				LogoURL:          s.logoURL(),
			},
		}, approver.Email)
	}
}

// notifyAbsenceDecision emails the requesting staff member about an approve /
// decline / Rückfrage on their request (#1419 4d).
func (s *staffAbsenceService) notifyAbsenceDecision(ctx context.Context, absence *activeModels.StaffAbsence) {
	StampAbsenceTypeLabels(ctx, s.absenceTypes, []*activeModels.StaffAbsence{absence}, s.getLogger())
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
	linkURL, ok := s.absenceEmailLink(ctx, absence, "/time-tracking")
	if !ok {
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
	s.dispatchAbsenceEmail(ctx, metaType, absence, AbsenceEmailMessage{
		To:       AbsenceEmailAddress{Name: requester.FirstName + " " + requester.LastName, Address: requester.Email},
		Subject:  subject,
		Template: template,
		Content: AbsenceEmailContent{
			FirstName:        requester.FirstName,
			LastName:         requester.LastName,
			AbsenceTypeLabel: absenceEmailTypeLabel(absence),
			DateRange:        formatAbsenceDateRange(absence),
			DecisionNote:     absence.DecisionNote,
			LinkURL:          linkURL,
			LogoURL:          s.logoURL(),
		},
	}, requester.Email)
}

func (s *staffAbsenceService) absenceEmailLink(ctx context.Context, absence *activeModels.StaffAbsence, targetPath string) (string, bool) {
	tenantID := absence.GetTenantID()
	if tenantID == 0 {
		tenantID = tenant.FromContext(ctx)
	}
	if tenantID == 0 {
		s.emailLogger().Warn("cannot build absence email link without tenant",
			"absence_id", absence.ID,
		)
		return "", false
	}

	subdomain, found, err := s.emailDeps.SchoolRepo.FindSchoolSubdomain(ctx, tenantID)
	if err != nil {
		s.emailLogger().Warn("failed to load school for absence email link",
			"absence_id", absence.ID,
			"tenant_id", tenantID,
			"error", err.Error(),
		)
		return "", false
	}
	if !found {
		s.emailLogger().Warn("school lookup returned no row for absence email link",
			"absence_id", absence.ID,
			"tenant_id", tenantID,
		)
		return "", false
	}
	link, err := buildTenantFrontendURL(s.emailDeps.FrontendURL, subdomain, targetPath)
	if err != nil {
		s.emailLogger().Warn("failed to build tenant-aware absence email link",
			"absence_id", absence.ID,
			"tenant_id", tenantID,
			"error", err.Error(),
		)
		return "", false
	}
	return link, true
}

func buildTenantFrontendURL(frontendURL, subdomain, targetPath string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(frontendURL))
	if err != nil {
		return "", fmt.Errorf("parse frontend URL: %w", err)
	}
	if base.Scheme == "" || base.Host == "" || base.Hostname() == "" {
		return "", fmt.Errorf("frontend URL must include scheme and host")
	}
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return "", fmt.Errorf("school subdomain is required")
	}
	if !strings.HasPrefix(targetPath, "/") {
		return "", fmt.Errorf("target path must start with '/'")
	}

	host := subdomain + "." + base.Hostname()
	if port := base.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	base.Host = host
	base.Path = targetPath
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func (s *staffAbsenceService) logoURL() string {
	return fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.emailDeps.FrontendURL)
}

func (s *staffAbsenceService) dispatchAbsenceEmail(ctx context.Context, metaType string, absence *activeModels.StaffAbsence, message AbsenceEmailMessage, recipient string) {
	dispatcher := s.emailDeps.Dispatcher
	message.Type, message.ReferenceID, message.Recipient = metaType, absence.ID, recipient
	// The reply address is resolved after commit, on a fresh context that only
	// carries the tenant: resolving it here would open a settings transaction
	// inside the caller's still-open one.
	tenantID := tenant.FromContext(ctx)
	message.TenantID = tenantID
	tenant.RegisterAfterCommit(ctx, func() {
		sendCtx := context.Background()
		// Without a tenant there is no OGS to answer to, and stamping the
		// context would panic. Send exactly as before in that case.
		if tenantID > 0 {
			sendCtx = tenant.WithTenantID(sendCtx, tenantID)
		}
		dispatcher.Dispatch(sendCtx, message)
	})
}
