package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// noticeMailTemplate is the mail that confirms a declaration.
const noticeMailTemplate = "care-contract-notice.html"

// confirmationValues are the values of the placeholders of the Träger's
// confirmation template (setting care_contract.*_confirmation).
type confirmationValues struct {
	Child      string
	Modules    string
	School     string
	EndsOn     *calendar.Date
	ReceivedOn calendar.Date
	Traeger    string
}

// confirmationText fills the Träger's template. The setting admits only
// these placeholders; an empty template is ErrNoticeTemplateMissing, never a
// built-in text.
func confirmationText(template string, values confirmationValues) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", carecontract.ErrNoticeTemplateMissing
	}
	endsOn := ""
	if values.EndsOn != nil {
		endsOn = germanDate(*values.EndsOn)
	}
	replacer := strings.NewReplacer(
		"{Kind}", values.Child, "{Module}", values.Modules, "{Schule}", values.School,
		"{Vertragsende}", endsOn, "{Eingang}", germanDate(values.ReceivedOn), "{Träger}", values.Traeger,
	)
	return strings.TrimSpace(replacer.Replace(template)), nil
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
