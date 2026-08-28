package announcement

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// letterIntro is the opening line of an Elternbrief mail. The body follows it,
// then the acknowledgement notice for recipients who can use the parent portal.
const letterIntro = "die folgende Mitteilung ist ein Elternbrief. Den vollständigen Text finden Sie unten."

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
	s.applyEmailOptOuts(ctx, a, recipients)

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
	if !r.HasPortalAccess {
		return platformModels.ReachabilityNoPortal
	}
	return platformModels.ReachabilityOK
}

func canQueueLetterMail(r *letterRecipient, a *usersModels.ParentAnnouncement) bool {
	return r.src.Email != "" && (r.src.HasPortalAccess || a.ReachesContactsWithoutPortal())
}

// applyEmailOptOuts downgrades recipients who explicitly refused this
// notification type. It honours an explicit "no" but not a missing decision —
// the same rule the untracked path uses, and for the same reason: most families
// have no preference row at all, so requiring an opt-in would silence the mails
// for practically everybody.
//
// A preference lookup failure is deliberately non-fatal: it must not roll back a
// publish. Failing open matches the pre-#2384 behaviour of this channel.
func (s *service) applyEmailOptOuts(ctx context.Context, a *usersModels.ParentAnnouncement, recipients []*letterRecipient) {
	if s.preferences == nil {
		return
	}
	accountIDs := make([]int64, 0, len(recipients))
	for _, r := range recipients {
		if canQueueLetterMail(r, a) && r.src.AccountID != nil {
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
		if !canQueueLetterMail(r, a) || r.src.AccountID == nil {
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
	portalAccessByAddress := make(map[string]bool, len(recipients))
	for _, r := range recipients {
		if !canQueueLetterMail(r, a) || r.reachability == platformModels.ReachabilityExcluded {
			continue
		}
		address := strings.ToLower(strings.TrimSpace(r.src.Email))
		portalAccessByAddress[address] = portalAccessByAddress[address] || r.src.HasPortalAccess
	}
	queued := 0
	for _, r := range recipients {
		if !canQueueLetterMail(r, a) || r.reachability == platformModels.ReachabilityExcluded {
			continue
		}
		address := strings.ToLower(strings.TrimSpace(r.src.Email))
		if existing, ok := byAddress[address]; ok {
			r.outboxID = existing
			continue
		}
		recipientPortalURL := ""
		if portalAccessByAddress[address] {
			recipientPortalURL = portalURL
		}
		row, err := s.outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: platformModels.EmailKindParentAnnouncement,
			Payload: map[string]any{
				emailPayloadRecipient:   address,
				emailPayloadFirstName:   r.src.FirstName,
				emailPayloadLastName:    r.src.LastName,
				emailPayloadTitle:       a.Title,
				emailPayloadSchoolName:  schoolName,
				emailPayloadPortalURL:   recipientPortalURL,
				emailPayloadLogoURL:     logoURL,
				emailPayloadMotoLogoURL: motoLogoURL,
				emailPayloadKicker:      kicker,
				emailPayloadIntro:       intro,
				emailPayloadBody:        body,
				emailPayloadAckRequired: a.RequiresAcknowledgement && portalAccessByAddress[address],
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
		stamp = a.PublishedAt.UTC().UnixNano()
	}
	return fmt.Sprintf("parent_announcement:%d:%d:%s", a.ID, stamp, address)
}

// LetterStatus is the staff-facing answer to "ist der Brief angekommen": one row
// per addressed person with the two channels reported separately, one row per
// child with the derived fulfilment, and the counts that drive the summary.
type LetterStatus struct {
	Recipients []*platformModels.EmailDeliveryStatus        `json:"recipients"`
	Children   []*usersModels.AnnouncementLetterChildStatus `json:"children"`
	Summary    LetterSummary                                `json:"summary"`
}

// LetterSummary counts what a school actually acts on, and keeps the two very
// different gaps apart.
//
// ChildrenOpen are children someone COULD confirm for and nobody has — those are
// the families to remind. ChildrenWithoutPortal are children no guardian can
// confirm for at all; reminding them changes nothing, the fix is to give a
// guardian portal access. Reporting them as one number is what made a
// school-wide letter to 116 children read as "6 offen".
type LetterSummary struct {
	ChildrenTotal         int `json:"children_total"`
	ChildrenConfirmable   int `json:"children_confirmable"`
	ChildrenFulfilled     int `json:"children_fulfilled"`
	ChildrenOpen          int `json:"children_open"`
	ChildrenWithoutPortal int `json:"children_without_portal"`
	RecipientsTotal       int `json:"recipients_total"`
	EmailsSent            int `json:"emails_sent"`
	EmailsPending         int `json:"emails_pending"`
	EmailsFailed          int `json:"emails_failed"`
	WithoutEmail          int `json:"without_email"`
	WithoutPortal         int `json:"without_portal"`
}

// LetterStatus assembles the recipient matrix and the per-child fulfilment.
//
// Both halves are resolved live rather than snapshotted at publish time, which
// is deliberate: a guardian who gains portal access after the letter went out
// should be able to confirm it, and the school should see that.
func (s *service) LetterStatus(ctx context.Context, id int64) (*LetterStatus, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for letter status: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	tenantID := a.GetTenantID()

	children, err := s.repo.LetterChildStatuses(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: letter child statuses: %w", err)
	}
	var recipients []*platformModels.EmailDeliveryStatus
	if s.deliveries != nil {
		recipients, err = s.deliveries.ListForEntity(ctx, tenantID, relatedEntityTypeAnnouncement, id)
		if err != nil {
			return nil, fmt.Errorf("announcement: letter recipients: %w", err)
		}
	}

	out := &LetterStatus{Recipients: recipients, Children: children}
	out.Summary.ChildrenTotal = len(children)
	out.Summary.RecipientsTotal = len(recipients)
	for _, c := range children {
		switch {
		case c.Fulfilled():
			out.Summary.ChildrenConfirmable++
			out.Summary.ChildrenFulfilled++
		case !c.CanConfirm:
			out.Summary.ChildrenWithoutPortal++
		default:
			out.Summary.ChildrenConfirmable++
			out.Summary.ChildrenOpen++
		}
	}
	for _, r := range recipients {
		switch r.EmailStatus {
		case "sent":
			out.Summary.EmailsSent++
		case "pending":
			out.Summary.EmailsPending++
		case "failed":
			out.Summary.EmailsFailed++
		}
		switch r.Reachability {
		case platformModels.ReachabilityNoEmail:
			out.Summary.WithoutEmail++
		case platformModels.ReachabilityNoPortal:
			out.Summary.WithoutPortal++
		}
	}
	return out, nil
}

// --- Erinnern und Nachversenden (#2384) --------------------------------------

const letterReminderKicker = "Elternbrief — Bestätigung fehlt"

// RemindOutstanding nudges whoever still owes a response, whatever kind of
// announcement this is: an unanswered poll or an unconfirmed Elternbrief.
//
// Like the poll reminder this is a MANUAL action. An unattended reminder loop
// would be the single most effective way to teach parents to mute the app, and
// schools want to decide when to nudge.
func (s *service) RemindOutstanding(ctx context.Context, id int64) (int, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("announcement: load for reminder: %w", err)
	}
	if a == nil {
		return 0, ErrNotFound
	}
	if a.IsPoll() {
		return s.RemindUnanswered(ctx, id)
	}
	if !a.IsLetter() || !a.RequiresAcknowledgement {
		// Nothing is owed, so there is nobody to remind. Refusing beats sending a
		// nag about a notice that never asked for anything.
		return 0, ErrNothingOutstanding
	}
	return s.remindUnacknowledged(ctx, a)
}

// remindUnacknowledged reaches only the guardians of children NOBODY has
// confirmed yet — the acceptance criterion "eine Erinnerung erreicht nur
// Familien, für die noch niemand bestätigt hat".
func (s *service) remindUnacknowledged(ctx context.Context, a *usersModels.ParentAnnouncement) (int, error) {
	if !a.IsPublished() || !a.Active {
		return 0, ErrNotPublished
	}
	now := time.Now()
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return 0, ErrNotPublished
	}
	enabled, err := s.newsEnabled(ctx)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve news flag: %w", err)
	}
	if !enabled {
		return 0, ErrNewsDisabled
	}

	tenantID := a.GetTenantID()
	recipients, err := s.repo.UnacknowledgedReminderRecipients(ctx, tenantID, a.ID)
	if err != nil {
		return 0, fmt.Errorf("announcement: letter reminder recipients: %w", err)
	}
	if len(recipients) == 0 {
		return 0, nil
	}

	intro := "für Ihr Kind fehlt noch die Bestätigung dieses Elternbriefs. Sie können ihn im Eltern-Portal öffnen und dort bestätigen. Es genügt, wenn eine sorgeberechtigte Person pro Kind bestätigt."
	emailed, err := s.enqueueReminderEmails(ctx, a, recipients, letterReminderKicker, intro, s.letterPortalURL(a.ID))
	if err != nil {
		return 0, err
	}
	delivered := make(map[int64]struct{}, len(emailed))
	for _, accountID := range emailed {
		delivered[accountID] = struct{}{}
	}
	s.logger.Info("parent letter reminder sent",
		slog.Int64("announcement_id", a.ID),
		slog.Int("recipient_count", len(delivered)),
		slog.Int("emails_queued", len(emailed)),
	)
	return len(delivered), nil
}

// ResendFailedEmails re-queues the mails that ended up FAILED, and only those.
//
// It is deliberately a separate action from the reminder: "hat nicht bestätigt"
// and "die Mail kam nie an" are different problems with different fixes, and
// merging them into one button would send a nag to people whose only issue is a
// wrong address. Recipients with no address are skipped — retrying changes
// nothing until the school fixes the data.
func (s *service) ResendFailedEmails(ctx context.Context, id int64) (int, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("announcement: load for resend: %w", err)
	}
	if a == nil {
		return 0, ErrNotFound
	}
	if !a.IsPublished() || !a.Active {
		return 0, ErrNotPublished
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now()) {
		return 0, ErrNotPublished
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

	resent := 0
	byAddress := make(map[string]*platformModels.EmailOutbox)
	portalAccessByAddress := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.RecipientEmail != nil && row.Reachability == platformModels.ReachabilityOK {
			address := strings.ToLower(strings.TrimSpace(*row.RecipientEmail))
			portalAccessByAddress[address] = true
		}
	}
	for _, row := range rows {
		if row.EmailStatus != "failed" || row.RecipientEmail == nil || *row.RecipientEmail == "" {
			continue
		}
		claimed, err := s.deliveries.ClaimFailedDelivery(ctx, tenantID, row.DeliveryID)
		if err != nil {
			return 0, fmt.Errorf("announcement: claim failed e-mail for resend: %w", err)
		}
		if !claimed {
			continue
		}
		address := strings.ToLower(strings.TrimSpace(*row.RecipientEmail))
		outboxRow, ok := byAddress[address]
		if !ok {
			recipientPortalURL := ""
			if portalAccessByAddress[address] {
				recipientPortalURL = portalURL
			}
			outboxRow, err = s.outbox.Enqueue(ctx, platformService.EnqueueRequest{
				Kind: platformModels.EmailKindParentAnnouncement,
				Payload: map[string]any{
					emailPayloadRecipient:   address,
					emailPayloadFirstName:   row.FirstName,
					emailPayloadLastName:    row.LastName,
					emailPayloadTitle:       a.Title,
					emailPayloadSchoolName:  schoolName,
					emailPayloadPortalURL:   recipientPortalURL,
					emailPayloadLogoURL:     logoURL,
					emailPayloadMotoLogoURL: motoLogoURL,
					emailPayloadKicker:      kicker,
					emailPayloadIntro:       intro,
					emailPayloadBody:        body,
					emailPayloadAckRequired: a.RequiresAcknowledgement && portalAccessByAddress[address],
				},
				RelatedEntityType: relatedEntityTypeAnnouncement,
				RelatedEntityID:   a.ID,
				// A retry needs a key of its own, otherwise the original publication's
				// key would swallow it as a duplicate and nothing would be sent.
				IdempotencyKey: fmt.Sprintf("%s:retry:%d", letterIdempotencyKey(a, address), time.Now().UTC().UnixNano()),
			})
			if err != nil {
				return 0, fmt.Errorf("announcement: resend failed e-mail: %w", err)
			}
			byAddress[address] = outboxRow
			resent++
		}
		if outboxRow == nil || outboxRow.ID <= 0 {
			return 0, fmt.Errorf("announcement: resend failed e-mail did not create an outbox row")
		}
		if err := s.deliveries.AttachOutbox(ctx, tenantID, row.DeliveryID, outboxRow.ID); err != nil {
			return 0, fmt.Errorf("announcement: attach resend outbox row: %w", err)
		}
	}
	s.logger.Info("parent announcement failed e-mails re-queued",
		slog.Int64("announcement_id", id),
		slog.Int("resent", resent),
	)
	return resent, nil
}
