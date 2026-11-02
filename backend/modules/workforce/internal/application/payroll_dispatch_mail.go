package application

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The two mails of the payroll dispatch (#4805): the link to the payroll
// office and the notice to the people who manage time tracking. Workforce
// writes them; Delivery only carries them.

const (
	PayrollLinkMailType    = "payroll_export_link"
	PayrollProblemMailType = "payroll_dispatch_problem"
	payrollLinkTemplate    = "payroll-export-link.html"
	payrollProblemTemplate = "payroll-dispatch-problem.html"
	// PayrollDownloadPath is the public page the link opens; the frontend
	// serves it outside every portal.
	PayrollDownloadPath = "/lohnbuero/"
)

// payrollLink is the link mail of one dispatch.
type payrollLink struct {
	TenantID   int64
	DispatchID int64
	Recipient  string
	Year       int
	Month      int
	Correction bool
	Token      string
	ExpiresAt  time.Time
}

// payrollProblem is the notice of one blocked or failed dispatch.
type payrollProblem struct {
	TenantID   int64
	DispatchID int64
	Year       int
	Month      int
	Correction bool
	Reason     string
}

func (s *PayrollDispatch) enqueueLink(ctx context.Context, link payrollLink) (int64, error) {
	school, err := s.deps.Schools.School(ctx, link.TenantID)
	if err != nil {
		return 0, fmt.Errorf("read the school for the link mail: %w", err)
	}
	month := payrollMonthLabel(link.Year, link.Month)
	subject := "Monatsdatei " + month
	if link.Correction {
		subject = "Korrektur der Monatsdatei " + month
	}
	return s.deps.Mail.Enqueue(ctx, workforce.PayrollMailMessage{
		TenantID: link.TenantID, ToAddress: link.Recipient, Subject: subject, Template: payrollLinkTemplate,
		Content: workforce.PayrollMailContent{
			LogoURL: s.logoURL(), SchoolName: school.Name, MonthLabel: month, Correction: link.Correction,
			LinkURL:   fmt.Sprintf("%s%s%d.%s", s.frontendURL(), PayrollDownloadPath, link.TenantID, link.Token),
			ExpiresAt: link.ExpiresAt.In(calendar.Berlin).Format("02.01.2006, 15:04 Uhr"),
		},
		Type: PayrollLinkMailType, ReferenceID: link.DispatchID,
		IdempotencyKey: fmt.Sprintf("payroll-export-link-%d", link.DispatchID),
		ExpiresAt:      link.ExpiresAt, MaxAttempts: workforce.PayrollDispatchMaxAttempts,
		Backoff: []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute},
	})
}

// notifyManagers mails everybody who manages time tracking. Nobody with an
// address is no error: the reason stays visible on /payroll.
func (s *PayrollDispatch) notifyManagers(ctx context.Context, problem payrollProblem) error {
	managers, err := s.deps.Managers.TimeTrackingManagers(ctx)
	if err != nil {
		return fmt.Errorf("list the time-tracking managers: %w", err)
	}
	school, err := s.deps.Schools.School(ctx, problem.TenantID)
	if err != nil {
		return fmt.Errorf("read the school for the notice: %w", err)
	}
	linkURL := s.frontendURL() + "/payroll"
	if tenantURL, err := payrollTenantURL(s.frontendURL(), school.Subdomain, "/payroll"); err == nil {
		linkURL = tenantURL
	}
	month := payrollMonthLabel(problem.Year, problem.Month)
	for _, manager := range managers {
		if strings.TrimSpace(manager.Email) == "" {
			continue
		}
		if _, err := s.deps.Mail.Enqueue(ctx, workforce.PayrollMailMessage{
			TenantID: problem.TenantID, ToName: strings.TrimSpace(manager.FirstName + " " + manager.LastName), ToAddress: manager.Email,
			Subject: "Monatsdatei " + month + " nicht verschickt", Template: payrollProblemTemplate,
			Content: workforce.PayrollMailContent{
				LogoURL: s.logoURL(), FirstName: manager.FirstName, MonthLabel: month,
				Correction: problem.Correction, ReasonText: payrollReasonText(problem.Reason), LinkURL: linkURL,
			},
			Type: PayrollProblemMailType, ReferenceID: problem.DispatchID,
			IdempotencyKey: fmt.Sprintf("payroll-dispatch-problem-%d-%d", problem.DispatchID, manager.StaffID),
			Backoff:        []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute},
		}); err != nil {
			return fmt.Errorf("enqueue the notice: %w", err)
		}
	}
	return nil
}

func (s *PayrollDispatch) frontendURL() string {
	return strings.TrimRight(strings.TrimSpace(s.deps.FrontendURL), "/")
}

func (s *PayrollDispatch) logoURL() string {
	return s.frontendURL() + "/images/moto-logo-mit-schriftzug.png"
}

var payrollMonthNames = [...]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August",
	"September", "Oktober", "November", "Dezember"}

func payrollMonthLabel(year, month int) string {
	if month < 1 || month > 12 {
		return fmt.Sprintf("%02d/%d", month, year)
	}
	return fmt.Sprintf("%s %d", payrollMonthNames[month-1], year)
}

// payrollReasonText is the notice's sentence for a reason code; /payroll
// uses the same wording.
func payrollReasonText(reason string) string {
	switch reason {
	case workforce.PayrollReasonDatevIncomplete:
		return "Die DATEV-Angaben sind nicht vollständig. Zum Beispiel fehlt eine Personalnummer oder die Nummer einer Lohnart."
	case workforce.PayrollReasonSFTPNotConfigured:
		return "Die Übertragung per SFTP ist ausgeschaltet oder nicht vollständig eingetragen."
	case workforce.PayrollReasonEmailMissing:
		return "Für den Versand per E-Mail fehlt die Adresse des Lohnbüros."
	case workforce.PayrollReasonMailFailed:
		return "Die E-Mail an das Lohnbüro konnte nicht zugestellt werden."
	case workforce.PayrollReasonInternal:
		return "Die Datei konnte nicht erstellt werden."
	default:
		return "Die Übertragung an das Lohnbüro hat viermal nicht geklappt."
	}
}

// payrollTenantURL puts the school's subdomain in front of the frontend
// host, the way the staff mails link into the school.
func payrollTenantURL(frontendURL, subdomain, targetPath string) (string, error) {
	base, err := url.Parse(frontendURL)
	if err != nil || base.Hostname() == "" || strings.TrimSpace(subdomain) == "" {
		return "", fmt.Errorf("cannot build the school link")
	}
	host := strings.TrimSpace(subdomain) + "." + base.Hostname()
	if port := base.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	base.Host, base.Path, base.RawQuery, base.Fragment = host, targetPath, "", ""
	return base.String(), nil
}
