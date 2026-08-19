package announcement

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// letterIntro is the opening line of an Elternbrief mail. The body follows it,
// then the acknowledgement notice — see templates/email/announcement-published.html.
const letterIntro = "die folgende Mitteilung erreicht Sie als Elternbrief. Den vollständigen Text finden Sie unten und jederzeit im Eltern-Portal."

// needsDeliveryTracking reports whether publishing this announcement must go
// through the tracked path: an Elternbrief (its recipient matrix is the whole
// point of #2384) or any announcement whose e-mail deliberately reaches beyond
// the portal audience (someone has to be able to see who that was).
//
// A plain Mitteilung to the portal audience keeps the original, untracked path
// unchanged — no behaviour change for everything that shipped before #2384.
func needsDeliveryTracking(a *usersModels.ParentAnnouncement) bool {
	return a.IsLetter() || a.ReachesContactsWithoutPortal()
}

// letterRecipient is one resolved person plus the decision taken about them.
type letterRecipient struct {
	src          *usersModels.AnnouncementDeliveryRecipient
	reachability string
	// outboxID is set once a mail has been queued for this person's address.
	outboxID *int64
}

// enqueueTrackedEmails is the Elternbrief publish path. It resolves every
// guardian linked to a reached child — including those that will get nothing —
// decides per person what happens, queues the mails, and writes one delivery row
// each so the staff matrix can show the whole picture rather than only the
// successes.
//
// Like enqueueAnnouncementEmails this runs inside the publish tenant tx, so the
// rows and the outbox entries commit atomically with published_at.
func (s *service) enqueueTrackedEmails(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	tenantID := a.GetTenantID()
	resolved, err := s.repo.ResolveDeliveryRecipients(ctx, tenantID, a.ID)
	if err != nil {
		return fmt.Errorf("announcement: resolve delivery recipients: %w", err)
	}
	if len(resolved) == 0 {
		return nil
	}

	recipients := make([]*letterRecipient, 0, len(resolved))
	for _, r := range resolved {
		recipients = append(recipients, &letterRecipient{
			src:          r,
			reachability: classifyReachability(r, a),
		})
	}
	s.applyEmailOptOuts(ctx, recipients)

	if err := s.queueLetterMails(ctx, a, recipients); err != nil {
		return err
	}

	if s.deliveries == nil {
		s.logger.Warn("announcement tracked delivery has no recorder wired; matrix will be empty",
			slog.Int64("announcement_id", a.ID))
		return nil
	}
	rows := make([]*platformModels.EmailDelivery, 0, len(recipients))
	for _, r := range recipients {
		row := &platformModels.EmailDelivery{
			GuardianProfileID: &r.src.GuardianProfileID,
			AccountID:         r.src.AccountID,
			Reachability:      r.reachability,
			OutboxID:          r.outboxID,
		}
		if r.src.Email != "" {
			email := r.src.Email
			row.RecipientEmail = &email
		}
		rows = append(rows, row)
	}
	if err := s.deliveries.ReplaceForEntity(ctx, tenantID, relatedEntityTypeAnnouncement, a.ID, rows); err != nil {
		return fmt.Errorf("announcement: record deliveries: %w", err)
	}
	return nil
}

// classifyReachability decides what happens to one person, and why.
//
// The order matters: a missing address beats everything else, because it is the
// gap the school can actually fix. "No portal access" is only a reason to
// withhold the mail when the announcement kept the narrow audience — with
// all_contacts the school has decided this content may reach them, and the row
// still records that they cannot acknowledge in moto.
func classifyReachability(r *usersModels.AnnouncementDeliveryRecipient, a *usersModels.ParentAnnouncement) string {
	if r.Email == "" {
		return platformModels.ReachabilityNoEmail
	}
	if !r.HasPortalAccess && !a.ReachesContactsWithoutPortal() {
		return platformModels.ReachabilityNoPortal
	}
	return platformModels.ReachabilityOK
}

// applyEmailOptOuts downgrades recipients who explicitly refused this
// notification type. It honours an explicit "no" but not a missing decision —
// the same rule the untracked path uses, and for the same reason: most families
// have no preference row at all, so requiring an opt-in would silence the mails
// for practically everybody.
//
// A preference lookup failure is deliberately non-fatal: it must not roll back a
// publish. Failing open matches the pre-#2384 behaviour of this channel.
func (s *service) applyEmailOptOuts(ctx context.Context, recipients []*letterRecipient) {
	if s.preferences == nil {
		return
	}
	accountIDs := make([]int64, 0, len(recipients))
	for _, r := range recipients {
		if r.reachability == platformModels.ReachabilityOK && r.src.AccountID != nil {
			accountIDs = append(accountIDs, *r.src.AccountID)
		}
	}
	if len(accountIDs) == 0 {
		return
	}
	remaining, err := s.preferences.FilterNotOptedOut(ctx, notifications.TypeParentAnnouncement, accountIDs)
	if err != nil {
		s.logger.Warn("announcement: opt-out filter failed, mailing all addressed guardians",
			slog.String("error", err.Error()))
		return
	}
	allowed := make(map[int64]struct{}, len(remaining))
	for _, id := range remaining {
		allowed[id] = struct{}{}
	}
	excluded := 0
	for _, r := range recipients {
		if r.reachability != platformModels.ReachabilityOK || r.src.AccountID == nil {
			continue
		}
		if _, ok := allowed[*r.src.AccountID]; !ok {
			r.reachability = platformModels.ReachabilityExcluded
			excluded++
		}
	}
	if excluded > 0 {
		s.logger.Info("announcement e-mails narrowed by opt-outs", slog.Int("excluded", excluded))
	}
}

// queueLetterMails enqueues one outbox row per distinct ADDRESS and links every
// recipient sharing that address to it.
//
// Two guardians of the same child often share one mailbox. They must receive one
// mail, not two — but both belong in the matrix, and both are correctly reported
// as "versendet", because that one mail informed both. Hence the address-keyed
// dedupe with a shared outbox id rather than a per-person send.
func (s *service) queueLetterMails(ctx context.Context, a *usersModels.ParentAnnouncement, recipients []*letterRecipient) error {
	if s.outbox == nil {
		s.logger.Warn("announcement needs e-mail but no outbox is wired",
			slog.Int64("announcement_id", a.ID))
		return nil
	}
	tenantID := a.GetTenantID()
	schoolName, err := s.repo.SchoolName(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("announcement: resolve school name: %w", err)
	}
	logoURL := s.resolveSchoolLogoURL(ctx, tenantID)
	motoLogoURL := emailbranding.MotoLogoURL(s.parentsURL)
	portalURL := s.letterPortalURL(a.ID)

	kicker := defaultEmailKicker
	intro := ""
	body := ""
	if a.IsLetter() {
		kicker = letterEmailKicker
		intro = letterIntro
		body = a.Body
	}

	byAddress := make(map[string]*int64, len(recipients))
	queued := 0
	for _, r := range recipients {
		if r.reachability != platformModels.ReachabilityOK {
			continue
		}
		address := strings.ToLower(strings.TrimSpace(r.src.Email))
		if existing, ok := byAddress[address]; ok {
			r.outboxID = existing
			continue
		}
		row, err := s.outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: platformModels.EmailKindParentAnnouncement,
			Payload: map[string]any{
				emailPayloadRecipient:   address,
				emailPayloadFirstName:   r.src.FirstName,
				emailPayloadLastName:    r.src.LastName,
				emailPayloadTitle:       a.Title,
				emailPayloadSchoolName:  schoolName,
				emailPayloadPortalURL:   portalURL,
				emailPayloadLogoURL:     logoURL,
				emailPayloadMotoLogoURL: motoLogoURL,
				emailPayloadKicker:      kicker,
				emailPayloadIntro:       intro,
				emailPayloadBody:        body,
				emailPayloadAckRequired: a.RequiresAcknowledgement,
			},
			RelatedEntityType: relatedEntityTypeAnnouncement,
			RelatedEntityID:   a.ID,
			IdempotencyKey:    letterIdempotencyKey(a, address),
		})
		if err != nil {
			return fmt.Errorf("announcement: enqueue e-mail: %w", err)
		}
		// A duplicate key is swallowed by the outbox (ON CONFLICT DO NOTHING) and
		// comes back without an id. That is a success, not a failure: the mail is
		// already queued from an earlier attempt.
		if row != nil && row.ID > 0 {
			id := row.ID
			r.outboxID = &id
			byAddress[address] = &id
			queued++
		}
	}
	s.logger.Info("parent announcement e-mails queued",
		slog.Int64("announcement_id", a.ID),
		slog.String("delivery_mode", a.DeliveryMode),
		slog.String("email_audience", a.EmailAudience),
		slog.Int("recipients", len(recipients)),
		slog.Int("queued", queued),
	)
	return nil
}

// letterPortalURL deep-links into the announcement rather than the portal root.
// The parent feed ignores an unknown query parameter today, so this degrades to
// the current behaviour until the portal learns to open the letter directly.
func (s *service) letterPortalURL(announcementID int64) string {
	if s.parentsURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/news?brief=%d", strings.TrimRight(s.parentsURL, "/"), announcementID)
}

// letterIdempotencyKey ties a queued mail to one publication of one letter for
// one address. The published_at timestamp is part of it on purpose: retries and
// concurrent publishes must not send twice, while the documented correction path
// (unpublish → edit → republish) SHOULD send the corrected letter again.
func letterIdempotencyKey(a *usersModels.ParentAnnouncement, address string) string {
	stamp := int64(0)
	if a.PublishedAt != nil {
		stamp = a.PublishedAt.UTC().Unix()
	}
	return fmt.Sprintf("parent_announcement:%d:%d:%s", a.ID, stamp, address)
}
