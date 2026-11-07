package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// noticeMailTemplate is the mail that confirms a declaration.
const noticeMailTemplate = "care-contract-notice.html"

// confirmationValues are the values of the placeholders of the Träger's
// confirmation template (setting care_contract.*_confirmation).
type confirmationValues struct {
	Child   string
	Modules string
	School  string
	EndsOn  *calendar.Date
	// Received is the receipt as the confirmation names it: day and time
	// for a declaration with the button (§ 312k Abs. 4 BGB), the day for a
	// letter the Träger-Büro records.
	Received string
	Traeger  string
	// Reason is the parents' reason; NeedsReason tells that the declaration
	// is an extraordinary termination, whose confirmation repeats it.
	Reason      string
	NeedsReason bool
}

// confirmationText fills the Träger's template. The setting admits only
// these placeholders; an empty template is ErrNoticeTemplateMissing, never a
// built-in text. What § 312k Abs. 4 BGB requires the confirmation to name
// is never left to the template: a template without {Eingang}, or without
// {Grund} for an extraordinary termination, gets the line appended.
func confirmationText(template string, values confirmationValues) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", carecontract.ErrNoticeTemplateMissing
	}
	template = strings.TrimSpace(template)
	if !strings.Contains(template, "{Eingang}") {
		template += "\n\nEingang Ihrer Erklärung: {Eingang}"
	}
	if values.NeedsReason && !strings.Contains(template, "{Grund}") {
		template += "\n\nIhr Grund: {Grund}"
	}
	endsOn := ""
	if values.EndsOn != nil {
		endsOn = germanDate(*values.EndsOn)
	}
	replacer := strings.NewReplacer(
		"{Kind}", values.Child, "{Module}", values.Modules, "{Schule}", values.School,
		"{Vertragsende}", endsOn, "{Eingang}", values.Received, "{Grund}", values.Reason, "{Träger}", values.Traeger,
	)
	return strings.TrimSpace(replacer.Replace(template)), nil
}

// receipt names when the declaration reached the Träger. A declaration with
// the button reaches it when the parents send it, so its confirmation names
// day and time (§ 312k Abs. 4 BGB); a letter the Träger-Büro records has
// only the day it arrived.
func receipt(notice ports.NewNotice) string {
	if notice.Channel != carecontract.NoticeChannelParents {
		return germanDate(notice.ReceivedOn)
	}
	return germanDate(calendar.DateFromTime(notice.DeclaredAt)) + " um " + *calendar.FormatBerlinClock(&notice.DeclaredAt) + " Uhr"
}

// templateKind selects the Träger's template: the withdrawal, the
// termination, or the extraordinary termination, whose end is still open.
func templateKind(notice ports.NewNotice) string {
	if notice.TerminationKind == carecontract.TerminationExtraordinary {
		return carecontract.TerminationExtraordinary
	}
	return notice.Kind
}

// RenderNoticeMail turns the payload of a claimed confirmation into the mail
// the worker sends: the Träger is the sender, the confirmation text the body.
func RenderNoticeMail(_ context.Context, payload []byte) (carecontract.RenderedNoticeMail, error) {
	var mail carecontract.NoticeMail
	if err := json.Unmarshal(payload, &mail); err != nil {
		return carecontract.RenderedNoticeMail{}, fmt.Errorf("%s payload is not a confirmation: %w", carecontract.MailKindNotice, err)
	}
	if strings.TrimSpace(mail.ConfirmationText) == "" {
		return carecontract.RenderedNoticeMail{}, errors.New(carecontract.MailKindNotice + " payload has no text")
	}
	// The subject names no child: the mailers log it at Info level.
	subject := "Bestätigung Ihrer Kündigung"
	if mail.Kind == carecontract.NoticeWithdrawal {
		subject = "Bestätigung Ihres Widerrufs"
	}
	paragraphs := []string{}
	for _, paragraph := range strings.Split(mail.ConfirmationText, "\n\n") {
		if paragraph = strings.TrimSpace(paragraph); paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return carecontract.RenderedNoticeMail{
		SenderName: strings.TrimSpace(mail.TraegerName), Subject: subject, Template: noticeMailTemplate,
		GuardianName: mail.GuardianName, TraegerName: mail.TraegerName,
		Withdrawal: mail.Kind == carecontract.NoticeWithdrawal, Paragraphs: paragraphs,
	}, nil
}
