package owners

import (
	"context"
	"fmt"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// parentBillingLetterMailRenderer binds the Elternabrechnung's mail of a
// confirmation letter (#4670, ADR 0123) to Delivery's worker: a mail of the
// Träger, sent with the Träger's name, with the stored PDF attached.
func parentBillingLetterMailRenderer(render parentbilling.LetterMailRenderer, defaultFrom email.Email) deliveryCompose.MailRenderer {
	return deliveryCompose.MailRendererFunc(func(ctx context.Context, intent *deliveryCompose.MailIntent) (*email.Message, error) {
		mail, err := render(ctx, intent.PayloadJSON)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", intent.Kind, err)
		}
		return &email.Message{
			From:    email.NewEmail(deliveryModule.SchoolSender(mail.SenderName, defaultFrom.Name, defaultFrom.Address)),
			Subject: mail.Subject, Template: mail.Template,
			Content: map[string]any{
				"GuardianName": mail.Content.GuardianName, "SchoolName": mail.Content.TraegerName,
				"PeriodName": mail.Content.PeriodName, "Number": mail.Content.Number,
			},
			Attachments: []email.Attachment{{Filename: mail.Filename, ContentType: "application/pdf", Data: mail.PDF}},
		}, nil
	})
}
