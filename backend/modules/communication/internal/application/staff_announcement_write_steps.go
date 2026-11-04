package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func normalizeAnnouncementTargets(in *Input) ([]*StaffParentAnnouncementTarget, error) {
	seen := make(map[string]bool, len(in.Targets))
	out := make([]*StaffParentAnnouncementTarget, 0, len(in.Targets))
	for _, t := range in.Targets {
		target, err := normalizeAnnouncementTarget(t, in.ResponseType)
		if err != nil {
			return nil, err
		}
		key := dedupeKey(target)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, target)
	}
	return out, nil
}

func normalizeAnnouncementTarget(t TargetInput, responseType string) (*StaffParentAnnouncementTarget, error) {
	if !StaffValidAnnouncementTargetType(t.TargetType) {
		return nil, fmt.Errorf("%w: target_type %q", ErrValidation, t.TargetType)
	}
	target := &StaffParentAnnouncementTarget{TargetType: t.TargetType}
	switch t.TargetType {
	case StaffAnnouncementTargetClass:
		if t.RefText == nil || strings.TrimSpace(*t.RefText) == "" {
			return nil, fmt.Errorf("%w: class target needs ref_text", ErrValidation)
		}
		text := strings.TrimSpace(*t.RefText)
		target.TargetRefText = &text
	case StaffAnnouncementTargetGroup,
		StaffAnnouncementTargetActivityGroup,
		StaffAnnouncementTargetStudent:
		if t.RefID == nil || *t.RefID <= 0 {
			return nil, fmt.Errorf("%w: %s target needs ref_id", ErrValidation, t.TargetType)
		}
		target.TargetRefID = t.RefID
	case StaffAnnouncementTargetSchoolAll:
		// no ref
	case StaffAnnouncementTargetPendingEnrollment:
		// A poll is answered per CHILD, and an open application has no
		// enrolled child yet — the applicant would see a question with no
		// answer row. The staff form hides the option for polls; refuse it
		// here too so the API cannot create that dead end.
		if responseType == StaffParentAnnouncementResponseSingleChoice ||
			responseType == StaffParentAnnouncementResponseMultiChoice {
			return nil, fmt.Errorf("%w: pending_enrollment cannot be targeted by a poll", ErrValidation)
		}
	}
	return target, nil
}

func applyAnnouncementInput(a *StaffParentAnnouncement, in Input) {
	a.Title = in.Title
	a.Body = in.Body
	a.Priority = in.Priority
	a.LinkURL = in.LinkURL
	a.RequiresAcknowledgement = in.RequiresAcknowledgement
	a.SendEmail = in.SendEmail
	a.ExpiresAt = in.ExpiresAt
	a.ResponseType = in.ResponseType
	a.ResponseDeadline = in.ResponseDeadline
	a.DeliveryMode = in.DeliveryMode
	a.EmailAudience = in.EmailAudience
	a.ReminderAt = in.ReminderAt
	a.ReminderText = in.ReminderText
	a.Declaration = in.Declaration
}

func validateAnnouncementPublication(a *StaffParentAnnouncement, now time.Time) error {
	// Refuse an already-expired draft: parents could never open it, and a
	// published announcement cannot be corrected in place.
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return fmt.Errorf("%w: expires_at is already in the past", ErrValidation)
	}
	if a.ResponseDeadline != nil && !a.ResponseDeadline.After(now) {
		return fmt.Errorf("%w: response_deadline is already in the past", ErrValidation)
	}
	// A reminder that already lies in the past would never fire (the due
	// scan only looks back a bounded window), so the draft would go live
	// with a reminder that silently never happens. Refuse it; staff moves
	// or removes the reminder and publishes again.
	if err := validateReminder(a.ReminderAt, a.ExpiresAt, now); err != nil {
		return err
	}
	return nil
}

func (s *service) finishAnnouncementPublication(ctx context.Context, id int64) error {
	// The `a` loaded above may be stale: PublishIfDraft blocks on the row
	// lock behind a concurrent Update, then flips the POST-edit row. So an
	// edit that changed the title, toggled send_email off, or moved
	// expires_at into the past can commit in that window. Re-read the
	// committed row (READ COMMITTED tenant tx → sees the concurrent commit)
	// and decide from it, not from the pre-publish snapshot.
	fresh, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("announcement: reload after publish: %w", err)
	}
	if fresh == nil {
		return fmt.Errorf("announcement: reload after publish: row not found")
	}
	if err := s.settlePublication(ctx, fresh); err != nil {
		return err
	}
	s.logger.Info("parent announcement published", slog.Int64("announcement_id", id))
	if err := s.notifyAnnouncementGuardians(ctx, fresh); err != nil {
		return fmt.Errorf("announcement: publish push notifications: %w", err)
	}
	// E-mail is deliberately independent of the push decision. The
	// dispatch switch and the active window exist so a phone does not
	// ring at night; an e-mail does not ring, and a school that opted
	// into e-mail for an announcement expects it to leave. The e-mail
	// path applies its own consent rule (see enqueueAnnouncementEmails).
	if fresh.SendEmail {
		// An Elternbrief (and any announcement mailing beyond the portal
		// audience) takes the tracked path: it records one delivery row per
		// addressed person, including the ones that get nothing, because the
		// recipient matrix is the deliverable. Everything else keeps the
		// original untracked path unchanged.
		enqueue := s.enqueueAnnouncementEmails
		if needsDeliveryTracking(fresh) {
			enqueue = s.enqueueTrackedEmails
		}
		if err := enqueue(ctx, fresh); err != nil {
			return fmt.Errorf("announcement: publish e-mails: %w", err)
		}
	}
	return nil
}

func dedupeKey(t *StaffParentAnnouncementTarget) string {
	switch {
	case t.TargetRefID != nil:
		return fmt.Sprintf("%s:id:%d", t.TargetType, *t.TargetRefID)
	case t.TargetRefText != nil:
		return fmt.Sprintf("%s:txt:%s", t.TargetType, *t.TargetRefText)
	default:
		return t.TargetType
	}
}
