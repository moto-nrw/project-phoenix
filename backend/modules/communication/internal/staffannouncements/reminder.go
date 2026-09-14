package announcement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

// Scheduled reminder (#3162).
//
// A school writes "Am letzten Schultag endet die Betreuung um 13:00 Uhr" two
// weeks ahead so families can plan, and wants the same announcement to reach
// everybody again the day before, when it is actually actionable. The reminder
// is therefore a SECOND DELIVERY of the same announcement to its whole
// audience: same targets, same channels, same audience rules, regardless of
// who has read or confirmed. It changes nothing about read or acknowledgement
// state and asks for no second confirmation.
//
// This is deliberately separate from the manual poll/letter reminder
// (RemindOutstanding), which reaches only the families that still owe an
// answer. The two must stay distinguishable in code and in the UI.

const (
	// maxReminderTextLen bounds the optional reminder wording. The reminder is
	// meant to bring the point, not repeat the whole announcement.
	maxReminderTextLen = 500

	// relatedEntityTypeReminder ties the reminder's outbox rows and push intents
	// to the announcement, separately from the publish rows, so a retraction
	// can cancel pending reminder mail alongside the publish mail.
	relatedEntityTypeReminder = "parent_announcement_reminder"

	reminderEmailKicker       = "Erinnerung"
	letterReminderEmailKicker = "Erinnerung: Elternbrief"
	reminderTitlePrefix       = "Erinnerung: "
)

// ErrReminderAlreadySent: the reminder went out, so its moment and wording are
// history and cannot be changed or removed any more.
var ErrReminderAlreadySent = errors.New("announcement: the reminder has already been sent")

// ReminderInput is the narrow post-publish edit: move the reminder, reword it,
// or remove it (nil ReminderAt). Title, text, audience and channels of a
// published announcement stay immutable.
type ReminderInput struct {
	ReminderAt   *time.Time `json:"reminder_at,omitempty"`
	ReminderText *string    `json:"reminder_text,omitempty"`
}

// normalizeReminder trims the reminder wording and validates the reminder
// against the rest of the input. A poll never carries a scheduled reminder:
// its manual "Eltern ohne Antwort erinnern" is the reminder a poll has, and a
// second automatic one to everybody would make the two indistinguishable.
func normalizeReminder(in *Input, now time.Time) error {
	in.ReminderText = normalizeReminderText(in.ReminderText)
	if in.ReminderAt == nil {
		if in.ReminderText != nil {
			return fmt.Errorf("%w: reminder_text requires reminder_at", ErrValidation)
		}
		return nil
	}
	if in.ReminderText != nil && len([]rune(*in.ReminderText)) > maxReminderTextLen {
		return fmt.Errorf("%w: reminder_text is at most %d characters", ErrValidation, maxReminderTextLen)
	}
	if in.ResponseType == usersModels.ParentAnnouncementResponseSingleChoice ||
		in.ResponseType == usersModels.ParentAnnouncementResponseMultiChoice {
		return fmt.Errorf("%w: polls do not support a scheduled reminder", ErrValidation)
	}
	return validateReminder(in.ReminderAt, in.ExpiresAt, now)
}

// normalizeReminderText trims the wording, folds blank to nil and bounds the
// length.
func normalizeReminderText(text *string) *string {
	if text == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*text)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// validateReminder is the moment rule shared by save, publish and the
// post-publish edit: a reminder lies in the future and never after the
// announcement's expiry, because it would then point at an announcement the
// portal no longer shows.
func validateReminder(reminderAt, expiresAt *time.Time, now time.Time) error {
	if reminderAt == nil {
		return nil
	}
	if !reminderAt.After(now) {
		return fmt.Errorf("%w: reminder_at must be in the future", ErrValidation)
	}
	if expiresAt != nil && reminderAt.After(*expiresAt) {
		return fmt.Errorf("%w: reminder_at must not be after expires_at", ErrValidation)
	}
	return nil
}

// UpdateReminder is the one edit a published announcement still accepts: the
// reminder can be moved, reworded or removed until it has been sent. Everything
// else stays frozen (ErrPublishedImmutable in Update).
func (s *service) UpdateReminder(ctx context.Context, id int64, in ReminderInput) (*usersModels.ParentAnnouncement, error) {
	// Under the row lock: the due tick claims the reminder with a guarded
	// UPDATE, and this edit must see either the unsent row or the sent one,
	// never an in-between.
	a, err := s.repo.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for reminder update: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.IsSystem() {
		return nil, ErrSystemAnnouncementImmutable
	}
	if a.ReminderSent() {
		return nil, ErrReminderAlreadySent
	}
	text := normalizeReminderText(in.ReminderText)
	if in.ReminderAt != nil {
		if a.IsPoll() {
			return nil, fmt.Errorf("%w: polls do not support a scheduled reminder", ErrValidation)
		}
		if text != nil && len([]rune(*text)) > maxReminderTextLen {
			return nil, fmt.Errorf("%w: reminder_text is at most %d characters", ErrValidation, maxReminderTextLen)
		}
		if err := validateReminder(in.ReminderAt, a.ExpiresAt, time.Now()); err != nil {
			return nil, err
		}
	} else {
		text = nil
	}
	applied, err := s.repo.SetReminder(ctx, id, in.ReminderAt, text)
	if err != nil {
		return nil, fmt.Errorf("announcement: set reminder: %w", err)
	}
	if !applied {
		// The guard (reminder_sent_at IS NULL) missed: the tick sent the
		// reminder between the lock-free read of the client and this write.
		return nil, ErrReminderAlreadySent
	}
	s.logger.Info("parent announcement reminder updated",
		slog.Int64("announcement_id", id),
		slog.Bool("removed", in.ReminderAt == nil),
	)
	return s.Get(ctx, id)
}

// SendDueReminders is the tenant half of the scheduler tick: it claims every
// reminder that fell due in (notBefore, dueBefore] and delivers it over the
// announcement's channels. The claim (reminder_sent_at) and the deliveries
// (push intents, outbox rows) are written in the same tenant transaction, so
// either the reminder is marked sent AND queued, or neither — a crash between
// the two cannot double-send on the next tick, and a rollback cannot leave a
// sent mark behind an unsent reminder.
func (s *service) SendDueReminders(ctx context.Context, notBefore, dueBefore time.Time) (int, error) {
	enabled, err := s.newsEnabled(ctx)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve news flag: %w", err)
	}
	if !enabled {
		// The school switched the feature off after scheduling: nothing is
		// live for parents, so nothing is reminded. The row keeps its unsent
		// reminder and staff sees it as missed.
		return 0, nil
	}
	due, err := s.repo.ListDueReminders(ctx, notBefore, dueBefore)
	if err != nil {
		return 0, fmt.Errorf("announcement: list due reminders: %w", err)
	}
	sent := 0
	for _, a := range due {
		claimed, err := s.repo.ClaimReminder(ctx, a.ID, dueBefore)
		if err != nil {
			return sent, fmt.Errorf("announcement: claim reminder: %w", err)
		}
		if !claimed {
			// An overlapping tick won, or the announcement was retracted or
			// expired since the scan. Nothing to send.
			continue
		}
		sentAt := dueBefore
		a.ReminderSentAt = &sentAt
		if err := s.deliverReminder(ctx, a); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// deliverReminder sends the second delivery over the announcement's own
// channels: push always, e-mail only when the announcement opted in, with the
// audience rules of the publication (portal audience, or the wider letter
// audience) applied unchanged.
func (s *service) deliverReminder(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	if err := s.notifyAnnouncementGuardiansAs(ctx, a, reminderPushShape(a)); err != nil {
		return fmt.Errorf("announcement: reminder push notifications: %w", err)
	}
	if a.SendEmail {
		if needsDeliveryTracking(a) {
			if err := s.enqueueTrackedReminderEmails(ctx, a); err != nil {
				return err
			}
		} else if err := s.enqueueAnnouncementEmailsAs(ctx, a, reminderMailSpec(a)); err != nil {
			return err
		}
	}
	s.logger.Info("parent announcement reminder sent",
		slog.Int64("announcement_id", a.ID),
		slog.String("delivery_mode", a.DeliveryMode),
		slog.Bool("email", a.SendEmail),
	)
	return nil
}

// reminderPushShape keeps the push generic like every other parent push (no
// content leaves the portal) and keys it on the reminder moment: a moved
// reminder is a new intent, a retried tick for the same moment is not.
func reminderPushShape(a *usersModels.ParentAnnouncement) pushShape {
	stamp := int64(0)
	if a.ReminderAt != nil {
		stamp = a.ReminderAt.UTC().UnixNano()
	}
	return pushShape{
		notificationType: parentAnnouncementNotificationType,
		copyKind:         notifications.ParentAnnouncementReminder,
		deepLink:         "/",
		idempotencyKey:   fmt.Sprintf("parent-announcement-reminder:%d:%d", a.ID, stamp),
		relatedType:      relatedEntityTypeReminder,
	}
}

// reminderIdempotencyKey ties one reminder mail to one address for one
// reminder moment. A retried tick must not queue twice; a moved reminder (a
// new moment) is a new mail by definition, but a moved reminder is only
// possible BEFORE the send, so the key never has to cover that.
func reminderIdempotencyKey(a *usersModels.ParentAnnouncement, address string) string {
	stamp := int64(0)
	if a.ReminderAt != nil {
		stamp = a.ReminderAt.UTC().UnixNano()
	}
	return fmt.Sprintf("parent_announcement_reminder:%d:%d:%s", a.ID, stamp, address)
}

func reminderIntro(schoolName, what string) string {
	if schoolName == "" {
		return fmt.Sprintf("wir erinnern Sie an %s.", what)
	}
	return fmt.Sprintf("die %s erinnert Sie an %s.", schoolName, what)
}

// reminderMailSpec is the untracked reminder mail of a plain Mitteilung: like
// its publish mail it carries the title and a portal link, never the text —
// the reminder wording waits in the portal, where reading counts.
func reminderMailSpec(a *usersModels.ParentAnnouncement) mailSpec {
	return mailSpec{
		title:  reminderTitlePrefix + a.Title,
		kicker: reminderEmailKicker,
		intro: func(schoolName string) string {
			return reminderIntro(schoolName, "diese Mitteilung") + " Den vollständigen Inhalt finden Sie im Eltern-Portal."
		},
		relatedType:    relatedEntityTypeReminder,
		idempotencyKey: func(address string) string { return reminderIdempotencyKey(a, address) },
	}
}

// reminderLetterMailSpec is the tracked reminder mail. An Elternbrief already
// let its body leave the portal, so its reminder carries the reminder wording
// the same way; a wide-audience Mitteilung stays title plus link. The
// confirmation notice is deliberately absent: the reminder asks for no second
// confirmation, and nagging a family that confirmed weeks ago is exactly what
// the manual "Offene erinnern" exists to avoid.
func reminderLetterMailSpec(a *usersModels.ParentAnnouncement) mailSpec {
	spec := mailSpec{
		title:          reminderTitlePrefix + a.Title,
		kicker:         reminderEmailKicker,
		relatedType:    relatedEntityTypeReminder,
		idempotencyKey: func(address string) string { return reminderIdempotencyKey(a, address) },
	}
	if a.IsLetter() {
		spec.kicker = letterReminderEmailKicker
		spec.intro = func(schoolName string) string {
			return reminderIntro(schoolName, "diesen Elternbrief") + " Den Text finden Sie unten."
		}
		spec.body = a.ReminderBody()
		return spec
	}
	spec.intro = func(schoolName string) string {
		return reminderIntro(schoolName, "diese Mitteilung") + " Den vollständigen Inhalt finden Sie im Eltern-Portal."
	}
	return spec
}

// enqueueTrackedReminderEmails re-runs the letter audience resolution for the
// reminder: the same people, the same reachability decision, the same opt-out
// rule. It does NOT rewrite the delivery matrix — the matrix documents the
// publication, and a reminder must not make a failed publish mail look sent.
func (s *service) enqueueTrackedReminderEmails(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	resolved, err := s.repo.ResolveDeliveryRecipients(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return fmt.Errorf("announcement: resolve reminder recipients: %w", err)
	}
	if len(resolved) == 0 {
		return nil
	}
	recipients := make([]*letterRecipient, 0, len(resolved))
	for _, r := range resolved {
		recipients = append(recipients, &letterRecipient{src: r, reachability: classifyReachability(r, a)})
	}
	s.applyEmailOptOuts(ctx, a, recipients)
	return s.queueLetterMailsAs(ctx, a, recipients, reminderLetterMailSpec(a))
}
