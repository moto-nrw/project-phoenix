package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// letterMailTemplate is the mail that carries a confirmation letter,
// correctionMailTemplate the one that carries the letter of a Storno or a
// Gutschrift (#4690).
const (
	letterMailTemplate     = "billing-letter.html"
	correctionMailTemplate = "billing-correction-letter.html"
	// careLetterMailTemplate carries the letter of a monthly invoice (#4684).
	careLetterMailTemplate = "billing-care-letter.html"
	// invoiceLetterMailTemplate carries a holiday invoice that is no
	// confirmation letter, noticeLetterMailTemplate a fee notice and
	// noticeCorrectionMailTemplate its Aufhebungs- or Änderungsbescheid
	// (#4688).
	invoiceLetterMailTemplate    = "billing-invoice-letter.html"
	noticeLetterMailTemplate     = "billing-notice-letter.html"
	noticeCorrectionMailTemplate = "billing-notice-correction-letter.html"
	// careChangeNoticeMailTemplate carries the change notice of a
	// Jahresbescheid (#6347); it may raise or lower the amount.
	careChangeNoticeMailTemplate = "billing-care-change-notice-letter.html"
)

// letterKindOf is the kind of document a mail names: empty for the kinds
// issued before #4688, so their payload stays as it was.
func letterKindOf(documentKind string) string {
	if documentKind == parentbilling.DocumentKindInvoice || documentKind == parentbilling.DocumentKindFeeNotice {
		return documentKind
	}
	return ""
}

// letterMailOf is the subject and the template of a letter mail.
func letterMailOf(mail parentbilling.BillingLetterMail) (string, string) {
	if subject, template, ok := careNoticeMailOf(mail.DocumentKind); ok {
		return subject, template
	}
	correction := mail.DocumentKind == parentbilling.DocumentCancellation || mail.DocumentKind == parentbilling.MailDocumentCareCancellation
	switch {
	case mail.LetterKind == parentbilling.DocumentKindFeeNotice && correction:
		return "Aufhebung Ihres Gebührenbescheids", noticeCorrectionMailTemplate
	case mail.LetterKind == parentbilling.DocumentKindFeeNotice && mail.DocumentKind == parentbilling.DocumentCredit:
		return "Änderung Ihres Gebührenbescheids", noticeCorrectionMailTemplate
	case mail.LetterKind == parentbilling.DocumentKindFeeNotice:
		return "Ihr Gebührenbescheid", noticeLetterMailTemplate
	}
	switch mail.DocumentKind {
	case parentbilling.DocumentCancellation:
		return "Storno Ihrer Rechnung", correctionMailTemplate
	case parentbilling.DocumentCredit:
		return "Gutschrift zu Ihrer Rechnung", correctionMailTemplate
	case parentbilling.MailDocumentCareInvoice:
		return "Ihre Rechnung", careLetterMailTemplate
	case parentbilling.MailDocumentCareCancellation:
		return "Storno Ihrer Rechnung", correctionMailTemplate
	}
	if mail.LetterKind == parentbilling.DocumentKindInvoice {
		return "Ihre Rechnung", invoiceLetterMailTemplate
	}
	return "Bestätigung und Rechnung", letterMailTemplate
}

// careNoticeMailOf is the subject and the template of the mail of a
// Jahresbescheid (#6347).
func careNoticeMailOf(documentKind string) (string, string, bool) {
	switch documentKind {
	case parentbilling.MailDocumentCareFeeNotice:
		return "Ihr Gebührenbescheid", noticeLetterMailTemplate, true
	case parentbilling.MailDocumentCareChangeNotice:
		return "Änderungsbescheid zu Ihrem Gebührenbescheid", careChangeNoticeMailTemplate, true
	}
	return "", "", false
}

// RenderLetterMail turns the payload of a claimed letter mail into the mail
// the worker sends: the Träger is the sender and the stored PDF the
// attachment (#4670).
func RenderLetterMail(_ context.Context, payload json.RawMessage) (parentbilling.RenderedLetterMail, error) {
	var mail parentbilling.BillingLetterMail
	if err := json.Unmarshal(payload, &mail); err != nil {
		return parentbilling.RenderedLetterMail{}, fmt.Errorf("%s payload is not a letter mail: %w", parentbilling.MailKindBillingLetter, err)
	}
	if len(mail.PDF) == 0 || strings.TrimSpace(mail.Filename) == "" {
		return parentbilling.RenderedLetterMail{}, errors.New(parentbilling.MailKindBillingLetter + " payload has no letter")
	}
	subject, template := letterMailOf(mail)
	if mail.PeriodName != "" {
		subject += ": " + mail.PeriodName
	}
	return parentbilling.RenderedLetterMail{
		SenderName: strings.TrimSpace(mail.TraegerName),
		Subject:    subject,
		Template:   template,
		Content: parentbilling.LetterMailContent{
			GuardianName: mail.GuardianName, TraegerName: mail.TraegerName, PeriodName: mail.PeriodName, Number: mail.Number,
		},
		Filename: mail.Filename,
		PDF:      mail.PDF,
	}, nil
}
