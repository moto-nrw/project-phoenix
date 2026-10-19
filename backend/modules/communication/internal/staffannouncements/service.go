// Package announcement holds the staff-side service for parent announcements
// (Elternmitteilungen, #1669): tenant-authored broadcast news to guardians.
// Staff requests run inside the per-request tenant transaction (TenantTxMiddleware),
// so methods use the context directly — the repository picks up that tx via
// base.GetDB.
package announcement

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

type service struct {
	deliveryPolicy   deliveryPorts.ParentDeliveryPolicy
	runtime          Runtime
	repo             ParentAnnouncementRepository
	settings         Settings
	outbox           OutboxEnqueuer
	pushOutbox       PushOutboxCanceller
	notifier         deliveryPorts.NotificationSender
	reminderNotifier deliveryPorts.DurableNotificationSender
	preferences      deliveryPorts.NotificationPreferences
	deliveries       DeliveryRecorder
	fingerprints     Fingerprints
	parentsURL       string
	// attachments is the file side of #2890, injected after construction (the
	// two services point at each other).
	attachments AttachmentPurger
	logger      *slog.Logger
}

// newsEnabled reports whether the parent-news feature is on for the current
// tenant. Runs inside the request tenant tx, so ResolveBool is correct.
func (s *service) newsEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.ParentNewsEnabled(ctx)
}

func (s *service) List(ctx context.Context, includeInactive bool) ([]*ParentAnnouncement, error) {
	rows, err := s.repo.ListForTenant(ctx, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("announcement: list: %w", err)
	}
	// Attach poll options in one batched query so the staff list can render an
	// Umfrage row (its options) without an N+1 fetch per announcement.
	ids := make([]int64, 0, len(rows))
	byID := make(map[int64]*ParentAnnouncement, len(rows))
	for _, a := range rows {
		if !a.IsPoll() {
			continue
		}
		ids = append(ids, a.ID)
		byID[a.ID] = a
		a.Options = []*ParentAnnouncementOption{}
	}
	if len(ids) == 0 {
		return rows, nil
	}
	options, err := s.repo.ListOptionsForAnnouncements(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("announcement: list options: %w", err)
	}
	for _, o := range options {
		if a, ok := byID[o.AnnouncementID]; ok {
			a.Options = append(a.Options, o)
		}
	}
	return rows, nil
}

func (s *service) Get(ctx context.Context, id int64) (*ParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: get: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	targets, err := s.repo.ListTargets(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: get targets: %w", err)
	}
	a.Targets = targets
	if a.IsPoll() {
		options, err := s.repo.ListOptions(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("announcement: get options: %w", err)
		}
		a.Options = options
	}
	return a, nil
}

func (s *service) Create(ctx context.Context, createdBy int64, in Input) (*ParentAnnouncement, error) {
	enabled, err := s.newsEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve news flag: %w", err)
	}
	if !enabled {
		return nil, ErrNewsDisabled
	}
	targets, err := normalizeInput(&in)
	if err != nil {
		return nil, err
	}
	options, err := normalizePollOptions(&in)
	if err != nil {
		return nil, err
	}
	if err := normalizeReminder(&in); err != nil {
		return nil, err
	}

	a := &ParentAnnouncement{
		Title:                   in.Title,
		Body:                    in.Body,
		Priority:                in.Priority,
		LinkURL:                 in.LinkURL,
		RequiresAcknowledgement: in.RequiresAcknowledgement,
		SendEmail:               in.SendEmail,
		ExpiresAt:               in.ExpiresAt,
		ResponseType:            in.ResponseType,
		ResponseDeadline:        in.ResponseDeadline,
		DeliveryMode:            in.DeliveryMode,
		EmailAudience:           in.EmailAudience,
		ReminderAt:              in.ReminderAt,
		ReminderText:            in.ReminderText,
		Declaration:             in.Declaration,
		Active:                  true,
		CreatedBy:               createdBy,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("announcement: create: %w", err)
	}
	if err := s.repo.ReplaceTargets(ctx, a.GetTenantID(), a.ID, targets); err != nil {
		return nil, fmt.Errorf("announcement: set targets: %w", err)
	}
	if err := s.repo.ReplaceOptions(ctx, a.GetTenantID(), a.ID, options); err != nil {
		return nil, fmt.Errorf("announcement: set options: %w", err)
	}
	a.Targets = targets
	a.Options = options
	s.logger.Info("parent announcement created",
		slog.Int64("announcement_id", a.ID),
		slog.Int64("created_by", createdBy),
		slog.Int("target_count", len(targets)),
		slog.String("response_type", a.ResponseType),
		slog.String("delivery_mode", a.DeliveryMode),
		slog.String("email_audience", a.EmailAudience),
		slog.Int("option_count", len(options)),
	)
	return a, nil
}

func (s *service) Update(ctx context.Context, id int64, in Input) (*ParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for update: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	// Published announcements are immutable (user decision, #1669): parents may
	// already have read or been e-mailed this exact wording. The correction
	// path is unpublish (retract) → edit the then-draft → republish.
	if a.IsPublished() {
		return nil, ErrPublishedImmutable
	}
	targets, options, err := s.prepareUpdate(ctx, a, &in)
	if err != nil {
		return nil, err
	}

	applyAnnouncementInput(a, in)
	if err := s.repo.Update(ctx, a); err != nil {
		// The write is guarded by published_at IS NULL: a publish between the
		// load and here matches no row and surfaces the same conflict.
		if s.repo.IsPublishedConflict(err) {
			return nil, ErrPublishedImmutable
		}
		return nil, fmt.Errorf("announcement: update: %w", err)
	}
	if err := s.repo.ReplaceTargets(ctx, a.GetTenantID(), a.ID, targets); err != nil {
		// ReplaceTargets locks the row and re-checks published_at IS NULL: if the
		// draft was published between the guarded content Update above and here,
		// it refuses the swap so e-mails and the live audience can't diverge.
		// Surface the same immutability conflict the load-time check would have.
		if s.repo.IsPublishedConflict(err) {
			return nil, ErrPublishedImmutable
		}
		return nil, fmt.Errorf("announcement: update targets: %w", err)
	}
	if err := s.repo.ReplaceOptions(ctx, a.GetTenantID(), a.ID, options); err != nil {
		// Same draft guard as ReplaceTargets: a publish that slipped in between
		// makes the option swap refuse rather than re-label answers.
		if s.repo.IsPublishedConflict(err) {
			return nil, ErrPublishedImmutable
		}
		return nil, fmt.Errorf("announcement: update options: %w", err)
	}
	a.Targets = targets
	a.Options = options
	return a, nil
}

func (s *service) Delete(ctx context.Context, id int64) error {
	// Unter der Zeilensperre laden, nicht nur lesen: der Anhang-Upload nimmt
	// dieselbe Sperre (#2890). Ohne sie kann ein Upload committen, nachdem
	// purgeAttachments die Anhänge aufgezählt hat — seine Zeile verschwindet
	// dann mit dem Cascade, ohne dass je ein Cleanup-Intent für ihre Bytes
	// geschrieben wurde.
	a, err := s.repo.FindByIDForUpdate(ctx, id)
	if err != nil {
		return fmt.Errorf("announcement: load for delete: %w", err)
	}
	if a == nil {
		return ErrNotFound
	}
	if a.IsSystem() {
		return ErrSystemAnnouncementImmutable
	}
	if err := s.guardDeclarationDelete(ctx, a); err != nil {
		return err
	}
	return s.deleteLocked(ctx, a)
}

// deleteLocked removes an announcement the caller loaded under its row lock,
// with everything that points at it only weakly. Both the manual delete and
// the nightly retention go this way.
func (s *service) deleteLocked(ctx context.Context, a *ParentAnnouncement) error {
	id := a.ID
	// Cancel any not-yet-sent e-mails and reminder pushes before removing the
	// announcement: their outbox rows reference it only by related_entity_id
	// (no FK), so a delete would otherwise leave pending notifications for an
	// announcement that no longer exists.
	if err := s.cancelPendingEmails(ctx, id); err != nil {
		return err
	}
	if err := s.cancelPendingReminderPushes(ctx, id); err != nil {
		return err
	}
	// Delivery rows carry the same weak link (related_entity_id, no FK), so they
	// need the same explicit cleanup.
	if err := s.dropDeliveryRows(ctx, a.GetTenantID(), id); err != nil {
		return err
	}
	// The attachment rows DO cascade — which is exactly the problem: once they
	// are gone, nothing points at their bytes any more. Record the cleanup
	// intents first, in this transaction, so a rollback takes them with it
	// (#2890).
	if err := s.purgeAttachments(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("announcement: delete: %w", err)
	}
	return nil
}

func (s *service) Publish(ctx context.Context, id int64) (*ParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for publish: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	// Re-check the flag at publish time: a draft authored while the feature was
	// on must not go live after a school turned it off.
	enabled, err := s.newsEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve news flag: %w", err)
	}
	if !enabled {
		return nil, ErrNewsDisabled
	}
	if !a.IsPublished() {
		now := time.Now()
		if err := validateAnnouncementPublication(a, now); err != nil {
			return nil, err
		}
		// Atomic publish: PublishIfDraft's UPDATE is guarded by
		// published_at IS NULL, so only ONE of two concurrent publishes flips the
		// row. Enqueue the opt-in e-mails only for that winner — a double-click or
		// two-tab race must not double-send the parent notification.
		published, err := s.repo.PublishIfDraft(ctx, id, now)
		if err != nil {
			return nil, fmt.Errorf("announcement: publish: %w", err)
		}
		if published {
			if err := s.finishAnnouncementPublication(ctx, id); err != nil {
				return nil, err
			}
		}
	}
	return s.Get(ctx, id)
}

// pushShape is everything that differs between the publish push and the
// scheduled-reminder push of the same announcement: the consent type, the
// locale-copy kind, the deep link, the idempotency key, and the related type
// that ties the intent to its cause.
type pushShape struct {
	notificationType string
	copyKind         string
	deepLink         string
	idempotencyKey   string
	relatedType      string
}

func publishPushShape(a *ParentAnnouncement) pushShape {
	notificationType, copyKind, deepLink := pushShapeFor(a)
	return pushShape{
		notificationType: notificationType,
		copyKind:         copyKind,
		deepLink:         deepLink,
		idempotencyKey:   fmt.Sprintf("parent-announcement:%d:%d:%s", a.ID, a.UpdatedAt.UTC().UnixNano(), copyKind),
		relatedType:      relatedEntityTypeAnnouncement,
	}
}

func (s *service) notifyAnnouncementGuardians(ctx context.Context, a *ParentAnnouncement) error {
	_, err := s.notifyAnnouncementGuardiansAs(ctx, a, publishPushShape(a))
	return err
}

// notifyAnnouncementGuardiansAs pushes to the announcement's current audience.
// The publication and the scheduled reminder share this resolution, so the
// reminder can never reach anyone the announcement itself would not.
func (s *service) notifyAnnouncementGuardiansAs(ctx context.Context, a *ParentAnnouncement, shape pushShape) (int, error) {
	if s.notifier == nil {
		return 0, nil
	}
	accepted, err := s.notifyAnnouncementGuardiansWith(ctx, a, shape, deliveryPorts.NewSingleReceiptSender(s.notifier))
	if s.deliveryPolicy.IsNotificationsDisabled(err) || s.deliveryPolicy.IsOutsideActiveWindow(err) {
		return accepted, nil
	}
	return accepted, err
}

// notifyReminderGuardians uses the durable receipt path. The reminder claim
// must roll back when neither Web Push nor the selected e-mail channel queued
// a delivery; a successful router call alone is not evidence of that.
func (s *service) notifyReminderGuardians(ctx context.Context, a *ParentAnnouncement, shape pushShape) (int, error) {
	if s.reminderNotifier == nil {
		return 0, nil
	}
	return s.notifyAnnouncementGuardiansWith(ctx, a, shape, s.reminderNotifier)
}

func (s *service) notifyAnnouncementGuardiansWith(ctx context.Context, a *ParentAnnouncement, shape pushShape, notify deliveryPorts.DurableNotificationSender) (int, error) {
	priority := deliveryPorts.PriorityNormal
	if a.Priority == ParentAnnouncementPriorityImportant {
		priority = deliveryPorts.PriorityHigh
	}
	accountIDs, localeByAccount, err := s.resolveAnnouncementPushAudience(ctx, a)
	if err != nil {
		return 0, err
	}
	if len(accountIDs) == 0 {
		return 0, nil
	}
	notificationType, copyKind, deepLink := shape.notificationType, shape.copyKind, shape.deepLink
	accountIDs, err = s.filterAnnouncementPushAudience(ctx, a.ID, notificationType, accountIDs)
	if err != nil || len(accountIDs) == 0 {
		return 0, err
	}

	groups := make(map[string][]int64)
	for _, accountID := range accountIDs {
		groups[localeByAccount[accountID]] = append(groups[localeByAccount[accountID]], accountID)
	}
	accepted := 0
	for locale, group := range groups {
		title, body := s.deliveryPolicy.ParentAnnouncementCopy(locale, copyKind)
		count, err := notify.NotifyDurably(ctx, deliveryPorts.NotificationEvent{
			Type: notificationType,
			// The push outbox deduplicates by tenant and key. Each locale is a
			// distinct payload and audience, so it needs its own replay-safe key.
			IdempotencyKey: fmt.Sprintf("%s:%s", shape.idempotencyKey, locale),
			RelatedType:    shape.relatedType, RelatedID: a.ID,
			Title:    title,
			Body:     body,
			DeepLink: deepLink,
			Priority: priority,
			Audience: deliveryPorts.NotificationAudience{
				TenantID:           a.GetTenantID(),
				Scope:              deliveryPorts.ScopeGuardian,
				GuardianAccountIDs: group,
			},
		}.Public())
		if s.deliveryPolicy.IsNotificationsDisabled(err) || s.deliveryPolicy.IsOutsideActiveWindow(err) {
			s.logger.Info("parent announcement push suppressed by tenant notification gate",
				slog.Int64("announcement_id", a.ID),
				slog.Int("recipient_count", len(accountIDs)),
				slog.String("reason", err.Error()),
			)
			return accepted, err
		}
		if err != nil {
			return accepted, fmt.Errorf("notify guardian audience: %w", err)
		}
		accepted += count
	}
	return accepted, nil
}

// enqueueAnnouncementEmails queues one outbox e-mail per targeted guardian with
// an address. This is a transactional-outbox write: the rows are inserted inside
// the publish tenant tx, so they commit atomically with published_at. The
// earlier "log and continue" swallowed DB errors, but a failed statement
// aborts the whole Postgres transaction — the subsequent read/commit then fails
// and the publish rolls back anyway, only with a misleading "success" log. So a
// genuine DB error is returned: the caller rolls back a clean, un-published
// announcement that staff can retry, rather than half-publishing with no e-mail
// and no signal. The outbox worker still handles the actual async send + retries
// (that part remains fire-and-forget). A missing outbox binding or an empty
// audience are not DB errors and stay non-fatal.
func (s *service) enqueueAnnouncementEmails(ctx context.Context, a *ParentAnnouncement) error {
	_, err := s.enqueueAnnouncementEmailsAs(ctx, a, publishMailSpec(a))
	return err
}

// mailSpec is what differs between the publish mail and the scheduled-reminder
// mail of the same announcement. The recipient resolution, the consent rule
// and the address dedupe are shared and must stay shared.
type mailSpec struct {
	title  string
	kicker string
	// intro receives the school name; the publish intro ignores it and lets the
	// template phrase the default line.
	intro       func(schoolName string) string
	body        string
	ackRequired bool
	relatedType string
	// idempotencyKey ties one queued mail to one address; nil = no key (the
	// original publish path never had one and keeps that behaviour).
	idempotencyKey func(address string) string
}

func publishMailSpec(a *ParentAnnouncement) mailSpec {
	spec := mailSpec{
		title:       a.Title,
		kicker:      defaultEmailKicker,
		intro:       func(string) string { return "" },
		relatedType: relatedEntityTypeAnnouncement,
	}
	// A poll says so in the mail: the kicker and intro differ, the rest of the
	// template (and the deliberate absence of the body) is identical.
	if a.IsPoll() {
		spec.kicker = pollEmailKicker
		intro := "wir bitten Sie um eine kurze Rückmeldung. Die Umfrage finden Sie im Eltern-Portal."
		if a.ResponseDeadline != nil {
			intro = fmt.Sprintf("wir bitten Sie um eine kurze Rückmeldung bis zum %s. Die Umfrage finden Sie im Eltern-Portal.",
				a.ResponseDeadline.Format("02.01.2006"))
		}
		spec.intro = func(string) string { return intro }
	}
	return declarationMailSpec(a, spec)
}

func (s *service) enqueueAnnouncementEmailsAs(ctx context.Context, a *ParentAnnouncement, spec mailSpec) (int, error) {
	if s.outbox == nil {
		s.logger.Warn("parent announcement opted into e-mail but no outbox is wired",
			slog.Int64("announcement_id", a.ID))
		return 0, nil
	}
	tenantID := a.GetTenantID()
	var recipients []*AnnouncementRecipient
	var err error
	recipients, err = s.repo.ResolveAudienceEmails(ctx, tenantID, a.ID)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve audience e-mails: %w", err)
	}
	if len(recipients) == 0 {
		return 0, nil
	}
	recipients, err = s.filterAnnouncementEmailRecipients(ctx, a, recipients)
	if err != nil || len(recipients) == 0 {
		return 0, err
	}
	recipients = uniqueAnnouncementEmails(recipients)
	if len(recipients) == 0 {
		return 0, nil
	}
	schoolName, err := s.repo.SchoolName(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve school name: %w", err)
	}
	return s.enqueueAnnouncementEmailBatch(ctx, a, spec, recipients, schoolName)
}

// resolveSchoolLogoURL returns the absolute school-logo URL for the e-mail
// header, or "" when none is configured. Branding is cosmetic, so a missing
// settings service or a lookup error is non-fatal: it degrades to the plain
// "OGS" fallback in the template rather than aborting the publish transaction.
func (s *service) resolveSchoolLogoURL(ctx context.Context, tenantID int64) string {
	if s.settings == nil {
		return ""
	}
	raw, err := s.settings.GetLoginImageURL(ctx, tenantID)
	if err != nil {
		s.logger.Warn("announcement: resolve school logo for e-mail failed, sending without it",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return ""
	}
	return s.deliveryPolicy.SchoolLogoURL(s.parentsURL, raw)
}

// cancelPendingEmails cancels announcement and poll-reminder e-mails that have
// not crossed the worker's dispatch fence when the announcement is retracted or
// deleted, so no portal link to a pulled (or since corrected) announcement goes
// out. It runs in the request tenant tx and commits with the state change; a
// missing outbox is a no-op, and a DB error is returned because a failed
// statement poisons the tx anyway.
func (s *service) cancelPendingEmails(ctx context.Context, id int64) error {
	if s.outbox == nil {
		return nil
	}
	cancelled := int64(0)
	for _, relatedType := range []string{relatedEntityTypeAnnouncement, relatedEntityTypePollReminder, relatedEntityTypeReminder} {
		n, err := s.outbox.CancelPendingByRelatedEntity(ctx, relatedType, id, "announcement no longer available")
		if err != nil {
			return fmt.Errorf("announcement: cancel e-mails: %w", err)
		}
		cancelled += n
	}
	if cancelled > 0 {
		s.logger.Info("parent announcement pending e-mails cancelled",
			slog.Int64("announcement_id", id),
			slog.Int64("cancelled", cancelled),
		)
	}
	return nil
}

// cancelPendingReminderPushes invalidates durable reminder pushes before the
// provider call. It also clears an active worker lease; the worker renews that
// lease immediately before delivery and therefore drops a retracted reminder
// instead of sending it. The cancellation runs in the retract/delete tenant
// transaction, so a rolled-back lifecycle change cannot affect a live reminder.
func (s *service) cancelPendingReminderPushes(ctx context.Context, id int64) error {
	if s.pushOutbox == nil {
		return nil
	}
	cancelled, err := s.pushOutbox.CancelPendingByRelatedEntity(ctx, relatedEntityTypeReminder, id, "announcement no longer available")
	if err != nil {
		return fmt.Errorf("announcement: cancel pending reminder pushes: %w", err)
	}
	if cancelled > 0 {
		s.logger.Info("parent announcement pending reminder pushes cancelled",
			slog.Int64("announcement_id", id),
			slog.Int64("cancelled", cancelled),
		)
	}
	return nil
}

// dropDeliveryRows removes the recipient matrix of a retracted or deleted
// announcement. A republish resolves the audience live and writes a fresh set,
// so keeping the old rows would only show a matrix for a wording parents never
// saw — and would strand recipients who have since been unlinked.
func (s *service) dropDeliveryRows(ctx context.Context, tenantID, id int64) error {
	if s.deliveries == nil {
		return nil
	}
	dropped, err := s.deliveries.DeleteForEntity(ctx, tenantID, relatedEntityTypeAnnouncement, id)
	if err != nil {
		return fmt.Errorf("announcement: drop delivery rows: %w", err)
	}
	if dropped > 0 {
		s.logger.Info("parent announcement delivery rows dropped",
			slog.Int64("announcement_id", id),
			slog.Int64("dropped", dropped),
		)
	}
	return nil
}

func (s *service) Unpublish(ctx context.Context, id int64) (*ParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for unpublish: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.IsSystem() {
		return nil, ErrSystemAnnouncementImmutable
	}
	if a.IsPublished() {
		if err := s.repo.SetPublished(ctx, id, nil); err != nil {
			return nil, fmt.Errorf("announcement: unpublish: %w", err)
		}
		// Retract queued opt-in e-mails and reminder pushes too: the correction
		// path is unpublish -> edit -> republish, so a pending notification with
		// the old wording must not go out after the announcement is pulled.
		if err := s.cancelPendingEmails(ctx, id); err != nil {
			return nil, err
		}
		if err := s.cancelPendingReminderPushes(ctx, id); err != nil {
			return nil, err
		}
		if err := s.dropDeliveryRows(ctx, a.GetTenantID(), id); err != nil {
			return nil, err
		}
		a.PublishedAt = nil
		s.logger.Info("parent announcement unpublished", slog.Int64("announcement_id", id))
	}
	return s.Get(ctx, id)
}

func (s *service) Stats(ctx context.Context, id int64) (*AnnouncementStats, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for stats: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	stats, err := s.repo.Stats(ctx, a.GetTenantID(), id)
	if err != nil {
		return nil, fmt.Errorf("announcement: stats: %w", err)
	}
	return stats, nil
}

// Recipients returns the announcement's live audience as named guardian
// accounts with their read/ack state — the per-person view behind Stats, so
// staff can see who still has to acknowledge.
func (s *service) Recipients(ctx context.Context, id int64) ([]*AnnouncementRecipientStatus, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for recipients: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	recipients, err := s.repo.AudienceRecipients(ctx, a.GetTenantID(), id)
	if err != nil {
		return nil, fmt.Errorf("announcement: recipients: %w", err)
	}
	return recipients, nil
}

// normalizeDelivery validates and completes the two delivery axes of #2384:
// delivery_mode (Mitteilung vs Elternbrief) and email_audience (who receives the
// mail, independently of who sees the announcement in the portal).
//
// The letter's mandatory channels are FORCED here rather than rejected. The
// database CHECK is the guarantee; this is the API being forgiving about how a
// client expresses the same intent — a caller that selects "Elternbrief" has
// already said it wants both channels, and failing that request with a 400 would
// only teach clients to send redundant flags. Silently WIDENING an audience or
// turning a poll into a letter, by contrast, would change what the author
// intended and is refused.
func normalizeDelivery(in *Input) error {
	if in.DeliveryMode == "" {
		in.DeliveryMode = ParentAnnouncementDeliveryStandard
	}
	if !ValidAnnouncementDeliveryMode(in.DeliveryMode) {
		return fmt.Errorf("%w: delivery_mode %q", ErrValidation, in.DeliveryMode)
	}
	if in.EmailAudience == "" {
		in.EmailAudience = EmailAudiencePortalOnly
	}
	if !ValidAnnouncementEmailAudience(in.EmailAudience) {
		return fmt.Errorf("%w: email_audience %q", ErrValidation, in.EmailAudience)
	}
	if in.EmailAudience == EmailAudienceAllContacts && in.DeliveryMode != ParentAnnouncementDeliveryLetter {
		return fmt.Errorf("%w: email_audience %q requires an Elternbrief", ErrValidation, in.EmailAudience)
	}

	if in.DeliveryMode == ParentAnnouncementDeliveryLetter {
		// A poll asks a question per child; a letter demands one acknowledgement
		// per child. Combining them would need a single row to carry two different
		// per-child completion semantics, so v1 keeps the axes separate (the
		// chk_parent_announcements_letter_not_poll constraint mirrors this).
		if in.ResponseType != "" && in.ResponseType != ParentAnnouncementResponseNone {
			return fmt.Errorf("%w: an Elternbrief cannot also be a poll", ErrValidation)
		}
		// Targeting people with an open enrollment reaches guardians that have no
		// student link yet — "the letter is fulfilled for this child" has no
		// meaning for them, and they would sit in the recipient matrix as
		// permanently outstanding. Refused rather than silently dropped so the
		// author notices while composing.
		for _, t := range in.Targets {
			if t.TargetType == AnnouncementTargetPendingEnrollment {
				return fmt.Errorf("%w: an Elternbrief cannot target pending enrollments", ErrValidation)
			}
		}
		in.SendEmail = true
		in.RequiresAcknowledgement = true
	}

	// Widening the e-mail audience without sending e-mail at all is a
	// contradiction that almost certainly means the client lost a flag. Refuse it
	// instead of persisting a setting that does nothing.
	if in.EmailAudience == EmailAudienceAllContacts && !in.SendEmail {
		return fmt.Errorf("%w: email_audience %q requires send_email", ErrValidation, in.EmailAudience)
	}
	return normalizeDeclaration(in)
}

// normalizeInput validates and trims the payload and returns the deduplicated target rows.
func normalizeInput(in *Input) ([]*ParentAnnouncementTarget, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if in.Title == "" || len([]rune(in.Title)) > maxTitleLen {
		return nil, fmt.Errorf("%w: title", ErrValidation)
	}
	if in.Body == "" || len([]rune(in.Body)) > maxBodyLen {
		return nil, fmt.Errorf("%w: body", ErrValidation)
	}
	link, err := normalizeLinkURL(in.LinkURL)
	if err != nil {
		return nil, err
	}
	in.LinkURL = link
	if in.Priority == "" {
		in.Priority = ParentAnnouncementPriorityInfo
	}
	if !ValidAnnouncementPriority(in.Priority) {
		return nil, fmt.Errorf("%w: priority", ErrValidation)
	}
	// A past expiry must fail as a 4xx here, not as a 500 later: the DB
	// constraint chk_parent_announcements_expiry (expires_at > created_at)
	// rejects it on insert, and publishing an already-expired announcement is
	// refused anyway (invisible to parents, immutable). Reject it at input time
	// so an operator who picks yesterday in the date picker gets a clean
	// validation error instead of a server error.
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return nil, fmt.Errorf("%w: expires_at must be in the future", ErrValidation)
	}
	if len(in.Targets) == 0 {
		return nil, fmt.Errorf("%w: at least one target required", ErrValidation)
	}
	if err := normalizeDelivery(in); err != nil {
		return nil, err
	}

	return normalizeAnnouncementTargets(in)
}

// normalizeLinkURL trims the optional announcement link and requires an
// absolute http(s) URL — parents tap this in their portal, so a relative or
// javascript: value must never be stored. Empty (after trim) collapses to nil.
func normalizeLinkURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	if len(trimmed) > maxLinkURLLen {
		return nil, fmt.Errorf("%w: link_url too long", ErrValidation)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("%w: link_url must be an absolute http(s) URL", ErrValidation)
	}
	return &trimmed, nil
}

func dedupeKey(t *ParentAnnouncementTarget) string {
	switch {
	case t.TargetRefID != nil:
		return fmt.Sprintf("%s:id:%d", t.TargetType, *t.TargetRefID)
	case t.TargetRefText != nil:
		return fmt.Sprintf("%s:txt:%s", t.TargetType, *t.TargetRefText)
	default:
		return t.TargetType
	}
}
