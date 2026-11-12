package api

import (
	"context"
	"encoding/json"
	"fmt"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// EnqueueCareLetterMail puts the mail of a letter of a monthly run on
// Delivery's outbox as a mail of the Träger with the PDF in its payload, in
// the caller's Träger transaction (#4684, ADR 0165). It is the mail kind of
// the holiday letters; only the related entity names the monthly run.
func (p parentBillingLetterPost) EnqueueCareLetterMail(ctx context.Context, mail parentbilling.BillingLetterMail) (int64, error) {
	payload, err := json.Marshal(mail)
	if err != nil {
		return 0, fmt.Errorf("care letter mail payload: %w", err)
	}
	enqueued, err := p.delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: parentbilling.MailKindBillingLetter,
		Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient, Name: mail.GuardianName},
		Payload:   payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: parentbilling.MailRelatedCareRun, ID: mail.RunID},
	})
	if err != nil {
		return 0, err
	}
	return enqueued.ID, nil
}

// CareLetterMailStates reads the mails of a monthly run Delivery still keeps.
func (p parentBillingLetterPost) CareLetterMailStates(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingLetterMailState, error) {
	statuses, err := p.delivery.TraegerEmailStatuses(ctx, organizationID,
		deliveryContract.RelatedEntity{Type: parentbilling.MailRelatedCareRun, ID: runID})
	if err != nil {
		return nil, err
	}
	states := make([]parentbilling.BillingLetterMailState, len(statuses))
	for i, status := range statuses {
		states[i] = parentbilling.BillingLetterMailState{
			DeliveryID: status.ID, Status: parentBillingMailStatus(status), Attempts: status.Attempts,
			SentAt: status.SentAt, Outcome: string(status.Outcome),
		}
	}
	return states, nil
}

// EnqueueCareNoticeLetterMail puts the mail of a Jahresbescheid on Delivery's
// outbox as a mail of the Träger with the stored PDF in its payload, in the
// caller's Träger transaction (#6347). It is the mail kind of every letter
// of the Träger; only the related entity names the notice run.
func (p parentBillingLetterPost) EnqueueCareNoticeLetterMail(ctx context.Context, mail parentbilling.BillingLetterMail) (int64, error) {
	payload, err := json.Marshal(mail)
	if err != nil {
		return 0, fmt.Errorf("care notice letter mail payload: %w", err)
	}
	enqueued, err := p.delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: parentbilling.MailKindBillingLetter,
		Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient, Name: mail.GuardianName},
		Payload:   payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: parentbilling.MailRelatedCareNoticeRun, ID: mail.RunID},
	})
	if err != nil {
		return 0, err
	}
	return enqueued.ID, nil
}

// CareNoticeLetterMailStates reads the mails of a notice run Delivery still keeps.
func (p parentBillingLetterPost) CareNoticeLetterMailStates(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingLetterMailState, error) {
	statuses, err := p.delivery.TraegerEmailStatuses(ctx, organizationID,
		deliveryContract.RelatedEntity{Type: parentbilling.MailRelatedCareNoticeRun, ID: runID})
	if err != nil {
		return nil, err
	}
	states := make([]parentbilling.BillingLetterMailState, len(statuses))
	for i, status := range statuses {
		states[i] = parentbilling.BillingLetterMailState{
			DeliveryID: status.ID, Status: parentBillingMailStatus(status), Attempts: status.Attempts,
			SentAt: status.SentAt, Outcome: string(status.Outcome),
		}
	}
	return states, nil
}
