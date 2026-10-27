package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// letterMailTemplate is the mail that carries a confirmation letter.
const letterMailTemplate = "billing-letter.html"

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
	subject := "Bestätigung und Rechnung"
	if mail.PeriodName != "" {
		subject += ": " + mail.PeriodName
	}
	return parentbilling.RenderedLetterMail{
		SenderName: strings.TrimSpace(mail.TraegerName),
		Subject:    subject,
		Template:   letterMailTemplate,
		Content: parentbilling.LetterMailContent{
			GuardianName: mail.GuardianName, TraegerName: mail.TraegerName, PeriodName: mail.PeriodName, Number: mail.Number,
		},
		Filename: mail.Filename,
		PDF:      mail.PDF,
	}, nil
}
