package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// ResendFailedEmails re-queues the mails that ended up FAILED, and only those.
//
// It is deliberately a separate action from the reminder: "hat nicht bestätigt"
// and "die Mail kam nie an" are different problems with different fixes, and
// merging them into one button would send a nag to people whose only issue is a
// wrong address. Recipients with no address are skipped — retrying changes
// nothing until the school fixes the data.
func (s *service) ResendFailedEmails(ctx context.Context, id int64) (int, error) {
	a, err := s.resendableAnnouncement(ctx, id)
	if err != nil {
		return 0, err
	}
	if s.deliveries == nil || s.outbox == nil {
		return 0, nil
	}
	tenantID := a.GetTenantID()
	rows, err := s.deliveries.ListForEntity(ctx, tenantID, relatedEntityTypeAnnouncement, id)
	if err != nil {
		return 0, fmt.Errorf("announcement: load deliveries for resend: %w", err)
	}

	schoolName, err := s.repo.SchoolName(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve school name: %w", err)
	}
	logoURL := s.resolveSchoolLogoURL(ctx, tenantID)
	motoLogoURL := s.deliveryPolicy.MotoLogoURL(s.parentsURL)
	portalURL := s.letterPortalURL(a.ID)
	kicker := defaultEmailKicker
	intro := ""
	body := ""
	if a.IsLetter() {
		kicker = letterEmailKicker
		intro = letterIntro
		body = a.Body
	}

	intro = classLetterMailIntro(a, schoolName, intro)

	resent := 0
	layout := letterMailLayout{schoolName: schoolName, logoURL: logoURL, motoLogoURL: motoLogoURL, portalURL: portalURL, kicker: kicker, intro: intro, body: body}

	byAddress := NewMailReceiptIndex()
	portalAccessByAddress := deliveryPortalAccess(rows)
	for _, row := range rows {
		queued, err := s.resendFailedLetterRecipient(ctx, a, row, layout, byAddress, portalAccessByAddress)
		if err != nil {
			return 0, err
		}
		if queued {
			resent++
		}
	}
	s.logger.Info("parent announcement failed e-mails re-queued",
		slog.Int64("announcement_id", id),
		slog.Int("resent", resent),
	)
	return resent, nil
}

type letterMailLayout struct {
	schoolName, logoURL, motoLogoURL, portalURL, kicker, intro, body string
}

func (s *service) enqueueLetterRecipient(ctx context.Context, a *StaffParentAnnouncement, r *letterRecipient, layout letterMailLayout, address, recipientPortalURL string, portalAccess bool, spec mailSpec, hasAttachment bool) (*communication.MailEnqueued, error) {
	return s.outbox.Enqueue(ctx, MailEnqueueInput{
		Kind: MailKindParentAnnouncement,
		Payload: map[string]json.RawMessage{
			emailPayloadRecipient:   JSONString(address),
			emailPayloadFirstName:   JSONString(r.src.FirstName),
			emailPayloadLastName:    JSONString(r.src.LastName),
			emailPayloadTitle:       JSONString(spec.title),
			emailPayloadSchoolName:  JSONString(layout.schoolName),
			emailPayloadPortalURL:   JSONString(recipientPortalURL),
			emailPayloadLogoURL:     JSONString(layout.logoURL),
			emailPayloadMotoLogoURL: JSONString(layout.motoLogoURL),
			emailPayloadKicker:      JSONString(layout.kicker),
			emailPayloadIntro:       JSONString(layout.intro),
			emailPayloadBody:        JSONString(layout.body),
			emailPayloadAckRequired: JSONBool(spec.ackRequired && portalAccess),
			// Only for recipients who can actually reach the portal: a hint
			// pointing somewhere the reader cannot go is worse than none.
			emailPayloadHasAttachment: JSONBool(hasAttachment && portalAccess),
		},
		RelatedEntityType: spec.relatedType,
		RelatedEntityID:   a.ID,
		IdempotencyKey:    spec.idempotencyKey(address),
	}.Public())
}

func (s *service) enqueueLetterRetry(ctx context.Context, a *StaffParentAnnouncement, row EmailDeliveryStatus, layout letterMailLayout, address, recipientPortalURL string, portalAccess bool) (*communication.MailEnqueued, error) {
	return s.outbox.Enqueue(ctx, MailEnqueueInput{
		Kind: MailKindParentAnnouncement,
		Payload: map[string]json.RawMessage{
			emailPayloadRecipient:   JSONString(address),
			emailPayloadFirstName:   JSONString(row.FirstName),
			emailPayloadLastName:    JSONString(row.LastName),
			emailPayloadTitle:       JSONString(a.Title),
			emailPayloadSchoolName:  JSONString(layout.schoolName),
			emailPayloadPortalURL:   JSONString(recipientPortalURL),
			emailPayloadLogoURL:     JSONString(layout.logoURL),
			emailPayloadMotoLogoURL: JSONString(layout.motoLogoURL),
			emailPayloadKicker:      JSONString(layout.kicker),
			emailPayloadIntro:       JSONString(layout.intro),
			emailPayloadBody:        JSONString(layout.body),
			emailPayloadAckRequired: JSONBool(a.RequiresAcknowledgement && portalAccess),
		},
		RelatedEntityType: relatedEntityTypeAnnouncement,
		RelatedEntityID:   a.ID,
		// A retry needs a key of its own, otherwise the original publication's
		// key would swallow it as a duplicate and nothing would be sent.
		IdempotencyKey: fmt.Sprintf("%s:retry:%d", letterIdempotencyKey(a, address), time.Now().UTC().UnixNano()),
	}.Public())
}

func letterRecipientPortalAccess(recipients []*letterRecipient, a *StaffParentAnnouncement) map[string]bool {
	portalAccessByAddress := make(map[string]bool, len(recipients))
	for _, r := range recipients {
		if !canQueueLetterMail(r, a) || r.reachability == reachabilityExcluded {
			continue
		}
		address := strings.ToLower(strings.TrimSpace(r.src.Email))
		portalAccessByAddress[address] = portalAccessByAddress[address] || r.src.HasPortalAccess
	}
	return portalAccessByAddress
}

func deliveryPortalAccess(rows []EmailDeliveryStatus) map[string]bool {
	portalAccessByAddress := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.RecipientEmail != nil && row.Reachability == reachabilityOK {
			address := strings.ToLower(strings.TrimSpace(*row.RecipientEmail))
			portalAccessByAddress[address] = true
		}
	}
	return portalAccessByAddress
}

func (s *service) resendFailedLetterRecipient(ctx context.Context, a *StaffParentAnnouncement, row EmailDeliveryStatus, layout letterMailLayout, byAddress map[string]*communication.MailEnqueued, portalAccessByAddress map[string]bool) (queued bool, err error) {
	tenantID := a.GetTenantID()
	if row.EmailStatus != "failed" || row.RecipientEmail == nil || *row.RecipientEmail == "" {
		return false, nil
	}
	claimed, err := s.deliveries.ClaimFailedDelivery(ctx, tenantID, row.DeliveryID)
	if err != nil {
		return false, fmt.Errorf("announcement: claim failed e-mail for resend: %w", err)
	}
	if !claimed {
		return false, nil
	}
	address := strings.ToLower(strings.TrimSpace(*row.RecipientEmail))
	outboxRow, ok := byAddress[address]
	if !ok {
		recipientPortalURL := ""
		if portalAccessByAddress[address] {
			recipientPortalURL = layout.portalURL
		}
		outboxRow, err = s.enqueueLetterRetry(ctx, a, row, layout, address, recipientPortalURL, portalAccessByAddress[address])
		if err != nil {
			return false, fmt.Errorf("announcement: resend failed e-mail: %w", err)
		}
		byAddress[address] = outboxRow
		queued = true
	}
	if outboxRow == nil || outboxRow.ID <= 0 {
		return false, fmt.Errorf("announcement: resend failed e-mail did not create an outbox row")
	}
	if err := s.deliveries.AttachOutbox(ctx, tenantID, row.DeliveryID, outboxRow.ID); err != nil {
		return false, fmt.Errorf("announcement: attach resend outbox row: %w", err)
	}
	return queued, nil
}

func (s *service) resendableAnnouncement(ctx context.Context, id int64) (*StaffParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for resend: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if err := guardClassLetterWrite(ctx, a); err != nil {
		return nil, err
	}
	if !a.IsPublished() || !a.Active {
		return nil, ErrNotPublished
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now()) {
		return nil, ErrNotPublished
	}
	return a, nil
}
