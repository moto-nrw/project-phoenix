package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

func (s *service) filterAnnouncementEmailRecipients(ctx context.Context, a *StaffParentAnnouncement, recipients []*StaffAnnouncementRecipient) ([]*StaffAnnouncementRecipient, error) {
	if s.preferences == nil {
		return recipients, nil
	}
	accountIDs := make([]int64, 0, len(recipients))
	seen := make(map[int64]struct{}, len(recipients))
	for _, recipient := range recipients {
		if recipient.AccountID <= 0 {
			continue
		}
		if _, exists := seen[recipient.AccountID]; exists {
			continue
		}
		seen[recipient.AccountID] = struct{}{}
		accountIDs = append(accountIDs, recipient.AccountID)
	}
	// E-mail honours an explicit "no" but not a missing decision: guardians
	// received announcement e-mails before the consent switches existed, and
	// most families have no row at all. Requiring an opt-in here would stop
	// the e-mails for practically everybody.
	notificationType, _, _ := pushShapeFor(a)
	remaining, err := s.preferences.FilterNotOptedOut(ctx, notificationType, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("announcement: filter opted-out e-mail recipients: %w", err)
	}
	allowed := make(map[int64]struct{}, len(remaining))
	for _, accountID := range remaining {
		allowed[accountID] = struct{}{}
	}
	filtered := make([]*StaffAnnouncementRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		if _, ok := allowed[recipient.AccountID]; ok {
			filtered = append(filtered, recipient)
		}
	}
	if len(filtered) < len(recipients) {
		s.logger.Info("parent announcement e-mails narrowed by opt-outs",
			slog.Int64("announcement_id", a.ID),
			slog.Int("candidate_count", len(recipients)),
			slog.Int("recipient_count", len(filtered)),
		)
	}
	recipients = filtered
	if len(recipients) == 0 {
		return nil, nil
	}
	return recipients, nil
}

func uniqueAnnouncementEmails(recipients []*StaffAnnouncementRecipient) []*StaffAnnouncementRecipient {
	uniqueRecipients := make([]*StaffAnnouncementRecipient, 0, len(recipients))
	seenEmails := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		normalizedEmail := strings.ToLower(strings.TrimSpace(recipient.Email))
		if normalizedEmail == "" {
			continue
		}
		if _, exists := seenEmails[normalizedEmail]; exists {
			continue
		}
		seenEmails[normalizedEmail] = struct{}{}
		normalizedRecipient := *recipient
		normalizedRecipient.Email = normalizedEmail
		uniqueRecipients = append(uniqueRecipients, &normalizedRecipient)
	}
	return uniqueRecipients
}

func (s *service) enqueueAnnouncementEmailBatch(ctx context.Context, a *StaffParentAnnouncement, spec mailSpec, recipients []*StaffAnnouncementRecipient, schoolName string) (int, error) {
	tenantID := a.GetTenantID()
	// Header/footer branding, resolved once for the whole batch. The same logo
	// URLs the enrollment mails use, so the announcement mail renders the school
	// logo (header) and the moto logo (footer) instead of the plain fallbacks.
	logoURL := s.resolveSchoolLogoURL(ctx, tenantID)
	motoLogoURL := s.deliveryPolicy.MotoLogoURL(s.parentsURL)
	kicker := spec.kicker
	intro := classLetterMailIntro(a, schoolName, spec.intro(schoolName))
	// Every recipient of this path has portal access by construction
	// (ResolveAudienceEmails is portal-audience only), so the attachment hint
	// always points somewhere the reader can go.
	hasAttachment := s.hasAttachments(ctx, a.ID)
	queued := 0
	for _, rcpt := range recipients {
		// Deliberately NO announcement body here: e-mail is the least trusted
		// channel, so the mail carries only the title plus a link into the
		// parent portal, where reading also counts for the read stats.
		request := MailEnqueueInput{
			Kind: MailKindParentAnnouncement,
			Payload: map[string]json.RawMessage{
				emailPayloadRecipient:     JSONString(rcpt.Email),
				emailPayloadFirstName:     JSONString(rcpt.FirstName),
				emailPayloadLastName:      JSONString(rcpt.LastName),
				emailPayloadTitle:         JSONString(spec.title),
				emailPayloadSchoolName:    JSONString(schoolName),
				emailPayloadPortalURL:     JSONString(s.parentsURL),
				emailPayloadLogoURL:       JSONString(logoURL),
				emailPayloadMotoLogoURL:   JSONString(motoLogoURL),
				emailPayloadKicker:        JSONString(kicker),
				emailPayloadIntro:         JSONString(intro),
				emailPayloadHasAttachment: JSONBool(hasAttachment),
			},
			RelatedEntityType: spec.relatedType,
			RelatedEntityID:   a.ID,
		}.Public()
		if spec.idempotencyKey != nil {
			request.IdempotencyKey = spec.idempotencyKey(rcpt.Email)
		}
		if _, err := s.outbox.Enqueue(ctx, request); err != nil {
			return queued, fmt.Errorf("announcement: enqueue e-mail: %w", err)
		}
		queued++
	}
	s.logger.Info("parent announcement e-mails queued",
		slog.Int64("announcement_id", a.ID),
		slog.Int("queued", queued),
	)
	return queued, nil
}
