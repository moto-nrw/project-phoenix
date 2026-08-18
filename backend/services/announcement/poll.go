package announcement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/localization"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// This file holds the poll (Umfrage, #1371) half of the staff announcement
// service: option validation, the staff-facing evaluation, and the manual
// reminder to guardians who have not answered.

var (
	// ErrNotAPoll is returned when a poll-only operation (results, reminder) is
	// called on a plain Mitteilung. Handler maps it to 400.
	ErrNotAPoll = errors.New("announcement: not a poll")
	// ErrPollNotOpen is returned when a reminder is requested for a poll that is
	// not currently collecting answers — an unpublished draft, or one whose
	// deadline has passed. Reminding about a closed poll would send parents to a
	// card they can no longer act on. Handler maps it to 409.
	ErrPollNotOpen = errors.New("announcement: poll is not open for answers")
)

const (
	relatedEntityTypePollReminder = "parent_announcement_poll_reminder"

	pollEmailKicker         = "Umfrage"
	pollReminderEmailKicker = "Erinnerung"
)

// normalizePollOptions validates the poll half of the input and returns the
// option rows to persist. A plain Mitteilung (empty/"none" response type) must
// carry neither options nor a deadline; a poll needs between minPollOptions and
// maxPollOptions non-blank, case-insensitively distinct labels and, if it has a
// deadline, one in the future.
//
// It normalizes in place (in.ResponseType is set to the canonical "none" and
// stray options/deadlines on a non-poll are cleared) so the caller can persist
// the input as-is.
func normalizePollOptions(in *Input) ([]*usersModels.ParentAnnouncementOption, error) {
	if strings.TrimSpace(in.ResponseType) == "" {
		in.ResponseType = usersModels.ParentAnnouncementResponseNone
	}
	if !usersModels.ValidAnnouncementResponseType(in.ResponseType) {
		return nil, fmt.Errorf("%w: response_type", ErrValidation)
	}
	if in.ResponseType == usersModels.ParentAnnouncementResponseNone {
		// A Mitteilung with leftover poll fields is a client bug, not something to
		// silently persist: the DB CHECK rejects a deadline without a response type
		// anyway, and stored orphan options would resurface if the row later became
		// a poll.
		if len(in.Options) > 0 {
			return nil, fmt.Errorf("%w: options require a response_type", ErrValidation)
		}
		in.ResponseDeadline = nil
		return []*usersModels.ParentAnnouncementOption{}, nil
	}
	// A poll response is the parent's explicit confirmation. Requiring a second
	// acknowledgement for the same announcement only creates an impossible-to-
	// explain extra step in the portal.
	in.RequiresAcknowledgement = false

	seen := make(map[string]bool, len(in.Options))
	out := make([]*usersModels.ParentAnnouncementOption, 0, len(in.Options))
	for _, raw := range in.Options {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}
		if len([]rune(label)) > maxPollLabelLen {
			return nil, fmt.Errorf("%w: option label too long", ErrValidation)
		}
		key := strings.ToLower(label)
		if seen[key] {
			// Two identical options would split the same answer across two bars and
			// make the result unreadable.
			return nil, fmt.Errorf("%w: duplicate option %q", ErrValidation, label)
		}
		seen[key] = true
		out = append(out, &usersModels.ParentAnnouncementOption{Label: label, Position: len(out)})
	}
	if len(out) < minPollOptions {
		return nil, fmt.Errorf("%w: a poll needs at least %d options", ErrValidation, minPollOptions)
	}
	if len(out) > maxPollOptions {
		return nil, fmt.Errorf("%w: a poll allows at most %d options", ErrValidation, maxPollOptions)
	}
	if in.ResponseDeadline != nil && !in.ResponseDeadline.After(time.Now()) {
		return nil, fmt.Errorf("%w: response_deadline must be in the future", ErrValidation)
	}
	// A deadline after the announcement disappears from the feed would be
	// unreachable: parents can only answer what they can still see.
	if in.ResponseDeadline != nil && in.ExpiresAt != nil && in.ResponseDeadline.After(*in.ExpiresAt) {
		return nil, fmt.Errorf("%w: response_deadline must not be after expires_at", ErrValidation)
	}
	return out, nil
}

// PollResults returns the per-option tally plus reached/answered child counts.
func (s *service) PollResults(ctx context.Context, id int64) (*usersModels.AnnouncementPollResults, error) {
	a, err := s.loadPoll(ctx, id)
	if err != nil {
		return nil, err
	}
	results, err := s.repo.PollResults(ctx, a.GetTenantID(), id)
	if err != nil {
		return nil, fmt.Errorf("announcement: poll results: %w", err)
	}
	return results, nil
}

// PollChildren returns every reached child with the answer given for them, so
// staff can chase the open ones by name.
func (s *service) PollChildren(ctx context.Context, id int64) ([]*usersModels.AnnouncementPollChildStatus, error) {
	a, err := s.loadPoll(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.PollChildren(ctx, a.GetTenantID(), id)
	if err != nil {
		return nil, fmt.Errorf("announcement: poll children: %w", err)
	}
	return rows, nil
}

// loadPoll fetches an announcement and asserts it is a poll.
func (s *service) loadPoll(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load poll: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if !a.IsPoll() {
		return nil, ErrNotAPoll
	}
	return a, nil
}

// RemindUnanswered notifies the guardians of reached children that still have no
// answer: one push (subject to the guardian's consent) and one e-mail per
// person, never per child. It returns how many guardians were reached.
//
// This is deliberately a MANUAL action rather than an automatic scheduler job:
// schools want to decide when to nudge, and an unattended reminder loop would be
// the single most effective way to teach parents to mute the app.
func (s *service) RemindUnanswered(ctx context.Context, id int64) (int, error) {
	a, err := s.loadPoll(ctx, id)
	if err != nil {
		return 0, err
	}
	if !a.IsPublished() || !a.Active {
		return 0, ErrPollNotOpen
	}
	now := time.Now()
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return 0, ErrPollNotOpen
	}
	if !a.AcceptsResponsesAt(now) {
		return 0, ErrPollNotOpen
	}
	enabled, err := s.newsEnabled(ctx)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve news flag: %w", err)
	}
	if !enabled {
		return 0, ErrNewsDisabled
	}

	tenantID := a.GetTenantID()
	recipients, err := s.repo.UnansweredReminderRecipients(ctx, tenantID, id)
	if err != nil {
		return 0, fmt.Errorf("announcement: poll reminder recipients: %w", err)
	}
	if len(recipients) == 0 {
		return 0, nil
	}

	emailed, err := s.enqueuePollReminderEmails(ctx, a, recipients)
	if err != nil {
		return 0, err
	}
	pushed, err := s.pushPollReminder(ctx, a, recipients)
	if err != nil {
		return 0, err
	}
	delivered := make(map[int64]struct{}, len(pushed)+len(emailed))
	for _, accountID := range pushed {
		delivered[accountID] = struct{}{}
	}
	for _, accountID := range emailed {
		delivered[accountID] = struct{}{}
	}
	s.logger.Info("parent poll reminder sent",
		slog.Int64("announcement_id", id),
		slog.Int("recipient_count", len(delivered)),
		slog.Int("emails_queued", len(emailed)),
	)
	return len(delivered), nil
}

// pushPollReminder sends the in-app/web push half of the reminder, narrowed by
// the guardian's own notification consent. A tenant-level gate (disabled or
// outside the active window) suppresses it without failing the action — the
// e-mail still goes out.
func (s *service) pushPollReminder(ctx context.Context, a *usersModels.ParentAnnouncement, recipients []*usersModels.AnnouncementPollReminderRecipient) ([]int64, error) {
	if s.notifier == nil {
		return nil, nil
	}
	accountIDs := make([]int64, 0, len(recipients))
	seen := make(map[int64]struct{}, len(recipients))
	localeByAccount := make(map[int64]string, len(recipients))
	for _, r := range recipients {
		if r.AccountID <= 0 {
			continue
		}
		if _, exists := seen[r.AccountID]; exists {
			continue
		}
		seen[r.AccountID] = struct{}{}
		accountIDs = append(accountIDs, r.AccountID)
		localeByAccount[r.AccountID] = localization.NormalizeLocale(r.PortalLocale)
	}
	if len(accountIDs) == 0 {
		return nil, nil
	}
	if s.preferences != nil {
		filtered, err := s.preferences.FilterOptedIn(ctx, notifications.TypeParentAnnouncement, accountIDs)
		if err != nil {
			return nil, fmt.Errorf("announcement: filter opted-in reminder recipients: %w", err)
		}
		accountIDs = filtered
		if len(accountIDs) == 0 {
			return nil, nil
		}
	}
	groups := make(map[string][]int64)
	for _, accountID := range accountIDs {
		groups[localeByAccount[accountID]] = append(groups[localeByAccount[accountID]], accountID)
	}
	for locale, group := range groups {
		title, body := notifications.ParentAnnouncementCopy(locale, notifications.ParentPollReminder)
		err := s.notifier.Notify(ctx, notifications.Event{
			Type:     parentAnnouncementNotificationType,
			Title:    title,
			Body:     body,
			DeepLink: "/",
			Priority: notifications.PriorityNormal,
			Audience: notifications.Audience{
				TenantID:           a.GetTenantID(),
				Scope:              notifications.ScopeGuardian,
				GuardianAccountIDs: group,
			},
		})
		if errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
			s.logger.Info("parent poll reminder push suppressed by tenant notification gate",
				slog.Int64("announcement_id", a.ID),
				slog.String("reason", err.Error()),
			)
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("announcement: notify poll reminder audience: %w", err)
		}
	}
	return accountIDs, nil
}

// enqueuePollReminderEmails queues one reminder e-mail per guardian address.
// Same transactional-outbox contract as the publish mails: rows are written in
// the request tenant tx, a DB error aborts the action rather than half-sending.
// The mail carries the poll title and a portal link, never the question's
// options — the answer belongs in the portal, where it is authenticated.
func (s *service) enqueuePollReminderEmails(ctx context.Context, a *usersModels.ParentAnnouncement, recipients []*usersModels.AnnouncementPollReminderRecipient) ([]int64, error) {
	if s.outbox == nil {
		s.logger.Warn("poll reminder requested but no outbox is wired",
			slog.Int64("announcement_id", a.ID))
		return nil, nil
	}
	tenantID := a.GetTenantID()
	if s.preferences != nil {
		accountIDs := make([]int64, 0, len(recipients))
		for _, r := range recipients {
			if r.AccountID > 0 {
				accountIDs = append(accountIDs, r.AccountID)
			}
		}
		// Mirrors the publish path: honour an explicit opt-out, but do not require
		// an opt-in (most families have no preference row at all).
		remaining, err := s.preferences.FilterNotOptedOut(ctx, notifications.TypeParentAnnouncement, accountIDs)
		if err != nil {
			return nil, fmt.Errorf("announcement: filter opted-out reminder recipients: %w", err)
		}
		allowed := make(map[int64]struct{}, len(remaining))
		for _, accountID := range remaining {
			allowed[accountID] = struct{}{}
		}
		filtered := make([]*usersModels.AnnouncementPollReminderRecipient, 0, len(recipients))
		for _, r := range recipients {
			if _, ok := allowed[r.AccountID]; ok {
				filtered = append(filtered, r)
			}
		}
		recipients = filtered
	}
	if len(recipients) == 0 {
		return nil, nil
	}
	schoolName, err := s.repo.SchoolName(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve school name: %w", err)
	}
	logoURL := s.resolveSchoolLogoURL(ctx, tenantID)
	motoLogoURL := emailbranding.MotoLogoURL(s.parentsURL)
	intro := "für Ihr Kind fehlt noch eine Rückmeldung zu dieser Umfrage. Sie können sie im Eltern-Portal abgeben."
	if a.ResponseDeadline != nil {
		intro = fmt.Sprintf("für Ihr Kind fehlt noch eine Rückmeldung zu dieser Umfrage. Bitte antworten Sie bis zum %s im Eltern-Portal.",
			a.ResponseDeadline.Format("02.01.2006"))
	}

	queuedAccountIDs := make([]int64, 0, len(recipients))
	seenEmails := make(map[string]struct{}, len(recipients))
	for _, r := range recipients {
		address := strings.ToLower(strings.TrimSpace(r.Email))
		if address == "" {
			continue
		}
		if _, exists := seenEmails[address]; exists {
			continue
		}
		seenEmails[address] = struct{}{}
		if _, err := s.outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: platformModels.EmailKindParentAnnouncement,
			Payload: map[string]any{
				emailPayloadRecipient:   address,
				emailPayloadFirstName:   r.FirstName,
				emailPayloadLastName:    r.LastName,
				emailPayloadTitle:       a.Title,
				emailPayloadSchoolName:  schoolName,
				emailPayloadPortalURL:   s.parentsURL,
				emailPayloadLogoURL:     logoURL,
				emailPayloadMotoLogoURL: motoLogoURL,
				emailPayloadKicker:      pollReminderEmailKicker,
				emailPayloadIntro:       intro,
			},
			// Reminders have their own related type so cleanup can cancel pending
			// reminder mail alongside the announcement's publish mail.
			RelatedEntityType: relatedEntityTypePollReminder,
			RelatedEntityID:   a.ID,
		}); err != nil {
			return nil, fmt.Errorf("announcement: enqueue poll reminder e-mail: %w", err)
		}
		queuedAccountIDs = append(queuedAccountIDs, r.AccountID)
	}
	return queuedAccountIDs, nil
}
