package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/services/active"
)

// absenceEmailMessage is the consumer-owned input mapped by this adapter.
type absenceEmailMessage = active.AbsenceEmailMessage

type absenceEmailDispatcher struct {
	dispatcher interface {
		Dispatch(context.Context, email.DeliveryRequest)
	}
	from     email.Email
	identity email.ReplyToResolver
	logger   *slog.Logger
}

func (d absenceEmailDispatcher) Dispatch(ctx context.Context, value absenceEmailMessage) {
	d.dispatcher.Dispatch(ctx, d.request(ctx, value))
}

func (d absenceEmailDispatcher) request(ctx context.Context, value absenceEmailMessage) email.DeliveryRequest {
	content := map[string]any{
		"FirstName":        value.Content.FirstName,
		"LastName":         value.Content.LastName,
		"AbsenceTypeLabel": value.Content.AbsenceTypeLabel,
		"DateRange":        value.Content.DateRange,
		"LinkURL":          value.Content.LinkURL,
		"LogoURL":          value.Content.LogoURL,
	}
	if value.Template == "absence-request-received.html" {
		content["RequesterName"] = value.Content.RequesterName
		content["Note"] = value.Content.Note
		content["PreviousQuestion"] = value.Content.PreviousQuestion
	} else {
		content["DecisionNote"] = value.Content.DecisionNote
	}
	message := email.Message{
		From: d.from, To: email.NewEmail(value.To.Name, value.To.Address),
		Subject: value.Subject, Template: value.Template, Content: content,
	}
	if value.TenantID > 0 {
		identity := email.ResolveReplyToIdentity(ctx, d.identity, value.TenantID, d.logger)
		message.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	}
	return email.DeliveryRequest{
		Message:  message,
		Metadata: email.DeliveryMetadata{Type: value.Type, ReferenceID: value.ReferenceID, Recipient: value.Recipient},
	}
}
