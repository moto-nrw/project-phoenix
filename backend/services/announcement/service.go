// Package announcement holds the staff-side service for parent announcements
// (Elternmitteilungen, #1669): tenant-authored broadcast news to guardians.
// Staff requests run inside the per-request tenant transaction (TenantTxMiddleware),
// so methods use the context directly — the repository picks up that tx via
// base.GetDB.
package announcement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// OutboxEnqueuer is the slice of the email outbox service this package needs:
// queue e-mail rows inside the current tenant transaction, and cancel not-yet-
// sent rows when an announcement is retracted before the worker drains them.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error)
	// CancelPendingByRelatedEntity cancels the still-pending outbox rows queued
	// for an announcement when it is unpublished or deleted, so a retracted or
	// corrected announcement can never deliver its stale title + portal link.
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}

const (
	maxTitleLen   = 200
	maxBodyLen    = 4000
	maxLinkURLLen = 2000

	// Poll (Umfrage) option bounds. Two options is the minimum that asks
	// anything ("Ja"/"Nein"); the upper bound keeps the parent card readable on a
	// phone and the result bars meaningful.
	minPollOptions  = 2
	maxPollOptions  = 10
	maxPollLabelLen = 120

	// relatedEntityTypeAnnouncement links an outbox row back to its announcement.
	// Shared by the enqueue and cancel paths so they never drift apart.
	relatedEntityTypeAnnouncement = "parent_announcement"

	parentAnnouncementNotificationType  = "parent_announcement"
	parentAnnouncementNotificationTitle = "Neue Elternmitteilung"
	parentAnnouncementNotificationBody  = "Eine neue Mitteilung ist im Elternportal verfügbar."

	// Polls reuse the announcement notification type (one guardian consent switch
	// covers both — a school broadcast is a school broadcast) but say what they
	// are, so a parent can tell a question from an information at a glance.
	parentPollNotificationTitle         = "Neue Umfrage"
	parentPollNotificationBody          = "Eine Schule bittet um Ihre Rückmeldung im Elternportal."
	parentPollReminderNotificationTitle = "Erinnerung: Umfrage offen"
	parentPollReminderNotificationBody  = "Für Ihr Kind fehlt noch eine Rückmeldung im Elternportal."
)

// Sentinel errors mapped to HTTP status by the handler.
var (
	ErrNotFound     = errors.New("announcement: not found")
	ErrValidation   = errors.New("announcement: invalid input")
	ErrNewsDisabled = errors.New("announcement: parent news is disabled for this school")
	// ErrPublishedImmutable: a published announcement is frozen — parents may
	// already have read (or been e-mailed) the exact wording, so silent edits
	// are forbidden. Correcting means unpublish (retract) → edit → republish,
	// which is visible to parents and re-triggers the opted-in e-mail.
	ErrPublishedImmutable = errors.New("announcement: published announcements cannot be edited")
)

// TargetInput is one audience selector supplied by the staff author.
type TargetInput struct {
	TargetType string  `json:"target_type"`
	RefID      *int64  `json:"ref_id,omitempty"`
	RefText    *string `json:"ref_text,omitempty"`
}

// Input is the create/update payload. ExpiresAt is optional (nil = never).
// SendEmail opts the announcement into an additional e-mail to the targeted
// guardians when it is published. LinkURL is an optional http(s) link shown
// with the announcement in the parent portal.
type Input struct {
	Title                   string        `json:"title"`
	Body                    string        `json:"body"`
	Priority                string        `json:"priority"`
	LinkURL                 *string       `json:"link_url,omitempty"`
	RequiresAcknowledgement bool          `json:"requires_acknowledgement"`
	SendEmail               bool          `json:"send_email"`
	ExpiresAt               *time.Time    `json:"expires_at,omitempty"`
	Targets                 []TargetInput `json:"targets"`

	// ResponseType turns the announcement into a poll (Umfrage): "" / "none" is a
	// plain Mitteilung, single_choice/multi_choice require Options. Options are
	// the answer labels in display order; ResponseDeadline is the optional answer
	// cut-off (independent of ExpiresAt, which controls visibility).
	ResponseType     string     `json:"response_type"`
	ResponseDeadline *time.Time `json:"response_deadline,omitempty"`
	Options          []string   `json:"options,omitempty"`
}

// Service is the staff-facing parent-announcement contract.
type Service interface {
	List(ctx context.Context, includeInactive bool) ([]*usersModels.ParentAnnouncement, error)
	Get(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error)
	Create(ctx context.Context, createdBy int64, in Input) (*usersModels.ParentAnnouncement, error)
	Update(ctx context.Context, id int64, in Input) (*usersModels.ParentAnnouncement, error)
	Delete(ctx context.Context, id int64) error
	Publish(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error)
	Unpublish(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error)
	Stats(ctx context.Context, id int64) (*usersModels.AnnouncementStats, error)
	Recipients(ctx context.Context, id int64) ([]*usersModels.AnnouncementRecipientStatus, error)

	// --- poll (Umfrage) ---
	// PollResults returns the per-option tally plus reached/answered child counts.
	PollResults(ctx context.Context, id int64) (*usersModels.AnnouncementPollResults, error)
	// PollChildren returns every reached child with the answer given for them.
	PollChildren(ctx context.Context, id int64) ([]*usersModels.AnnouncementPollChildStatus, error)
	// RemindUnanswered notifies the guardians of children that have not answered
	// yet and reports how many were reached.
	RemindUnanswered(ctx context.Context, id int64) (int, error)
}

// ServiceConfig is the dependency bundle. Outbox, Notifier and ParentsURL are
// optional: nil delivery dependencies leave the in-app feed working.
type ServiceConfig struct {
	Repo     usersModels.ParentAnnouncementRepository
	Settings configService.SettingsService
	Outbox   OutboxEnqueuer
	Notifier notifications.Service
	// Preferences gates the push by the guardian's own consent. Optional; when
	// absent no guardian is notified at all, which is the safe direction.
	Preferences notifications.PreferenceService
	ParentsURL  string
	Logger      *slog.Logger
}

type service struct {
	repo        usersModels.ParentAnnouncementRepository
	settings    configService.SettingsService
	outbox      OutboxEnqueuer
	notifier    notifications.Service
	preferences notifications.PreferenceService
	parentsURL  string
	logger      *slog.Logger
}

// NewService wires the staff announcement service.
func NewService(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		repo:        cfg.Repo,
		settings:    cfg.Settings,
		outbox:      cfg.Outbox,
		notifier:    cfg.Notifier,
		preferences: cfg.Preferences,
		parentsURL:  cfg.ParentsURL,
		logger:      logger,
	}
}

// newsEnabled reports whether the parent-news feature is on for the current
// tenant. Runs inside the request tenant tx, so ResolveBool is correct.
func (s *service) newsEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.ResolveBool(ctx, configModel.KeyParentNewsEnabled)
}

func (s *service) List(ctx context.Context, includeInactive bool) ([]*usersModels.ParentAnnouncement, error) {
	rows, err := s.repo.ListForTenant(ctx, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("announcement: list: %w", err)
	}
	// Attach poll options in one batched query so the staff list can render an
	// Umfrage row (its options) without an N+1 fetch per announcement.
	ids := make([]int64, 0, len(rows))
	byID := make(map[int64]*usersModels.ParentAnnouncement, len(rows))
	for _, a := range rows {
		if !a.IsPoll() {
			continue
		}
		ids = append(ids, a.ID)
		byID[a.ID] = a
		a.Options = []*usersModels.ParentAnnouncementOption{}
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

func (s *service) Get(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
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

func (s *service) Create(ctx context.Context, createdBy int64, in Input) (*usersModels.ParentAnnouncement, error) {
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

	a := &usersModels.ParentAnnouncement{
		Title:                   in.Title,
		Body:                    in.Body,
		Priority:                in.Priority,
		LinkURL:                 in.LinkURL,
		RequiresAcknowledgement: in.RequiresAcknowledgement,
		SendEmail:               in.SendEmail,
		ExpiresAt:               in.ExpiresAt,
		ResponseType:            in.ResponseType,
		ResponseDeadline:        in.ResponseDeadline,
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
		slog.Int("option_count", len(options)),
	)
	return a, nil
}

func (s *service) Update(ctx context.Context, id int64, in Input) (*usersModels.ParentAnnouncement, error) {
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
	targets, err := normalizeInput(&in)
	if err != nil {
		return nil, err
	}
	options, err := normalizePollOptions(&in)
	if err != nil {
		return nil, err
	}

	a.Title = in.Title
	a.Body = in.Body
	a.Priority = in.Priority
	a.LinkURL = in.LinkURL
	a.RequiresAcknowledgement = in.RequiresAcknowledgement
	a.SendEmail = in.SendEmail
	a.ExpiresAt = in.ExpiresAt
	a.ResponseType = in.ResponseType
	a.ResponseDeadline = in.ResponseDeadline
	if err := s.repo.Update(ctx, a); err != nil {
		// The write is guarded by published_at IS NULL: if the draft was
		// published between the load above and here, no row matched. Surface the
		// same immutability conflict the load-time check would have.
		if errors.Is(err, usersModels.ErrAnnouncementPublished) {
			return nil, ErrPublishedImmutable
		}
		return nil, fmt.Errorf("announcement: update: %w", err)
	}
	if err := s.repo.ReplaceTargets(ctx, a.GetTenantID(), a.ID, targets); err != nil {
		// ReplaceTargets locks the row and re-checks published_at IS NULL: if the
		// draft was published between the guarded content Update above and here,
		// it refuses the swap so e-mails and the live audience can't diverge.
		// Surface the same immutability conflict the load-time check would have.
		if errors.Is(err, usersModels.ErrAnnouncementPublished) {
			return nil, ErrPublishedImmutable
		}
		return nil, fmt.Errorf("announcement: update targets: %w", err)
	}
	if err := s.repo.ReplaceOptions(ctx, a.GetTenantID(), a.ID, options); err != nil {
		// Same draft guard as ReplaceTargets: a publish that slipped in between
		// makes the option swap refuse rather than re-label answers.
		if errors.Is(err, usersModels.ErrAnnouncementPublished) {
			return nil, ErrPublishedImmutable
		}
		return nil, fmt.Errorf("announcement: update options: %w", err)
	}
	a.Targets = targets
	a.Options = options
	return a, nil
}

func (s *service) Delete(ctx context.Context, id int64) error {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("announcement: load for delete: %w", err)
	}
	if a == nil {
		return ErrNotFound
	}
	// Cancel any not-yet-sent e-mails before removing the announcement: outbox
	// rows reference it only by related_entity_id (no FK), so a delete would
	// otherwise leave pending rows that still deliver a notification for an
	// announcement that no longer exists.
	if err := s.cancelPendingEmails(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("announcement: delete: %w", err)
	}
	return nil
}

func (s *service) Publish(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
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
		// Refuse an already-expired draft: the parent feed only shows rows with
		// expires_at > NOW(), so publishing one would e-mail guardians about an
		// announcement they can never open — and published announcements are
		// immutable, so the mistake can't be corrected in place. Block it before
		// any state change or e-mail.
		if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
			return nil, fmt.Errorf("%w: expires_at is already in the past", ErrValidation)
		}
		if a.ResponseDeadline != nil && !a.ResponseDeadline.After(now) {
			return nil, fmt.Errorf("%w: response_deadline is already in the past", ErrValidation)
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
			// The `a` loaded above may be stale: PublishIfDraft blocks on the row
			// lock behind a concurrent Update, then flips the POST-edit row. So an
			// edit that changed the title, toggled send_email off, or moved
			// expires_at into the past can commit in that window. Re-read the
			// committed row (READ COMMITTED tenant tx → sees the concurrent commit)
			// and decide from it, not from the pre-publish snapshot.
			fresh, err := s.repo.FindByID(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("announcement: reload after publish: %w", err)
			}
			if fresh == nil {
				return nil, fmt.Errorf("announcement: reload after publish: row not found")
			}
			// A concurrent edit may have moved expires_at into the past between the
			// pre-publish check and the atomic flip. Publishing an already-expired
			// announcement is invalid (invisible to parents, immutable), so fail
			// with a 5xx: the tenant tx rolls back the flip and staff can retry.
			if fresh.ExpiresAt != nil && !fresh.ExpiresAt.After(now) {
				return nil, fmt.Errorf("announcement: draft expired concurrently during publish; rolled back")
			}
			if fresh.ResponseDeadline != nil && !fresh.ResponseDeadline.After(now) {
				return nil, fmt.Errorf("announcement: draft response deadline elapsed concurrently during publish; rolled back")
			}
			s.logger.Info("parent announcement published", slog.Int64("announcement_id", id))
			if err := s.notifyAnnouncementGuardians(ctx, fresh); err != nil {
				return nil, fmt.Errorf("announcement: publish push notifications: %w", err)
			}
			// E-mail is deliberately independent of the push decision. The
			// dispatch switch and the active window exist so a phone does not
			// ring at night; an e-mail does not ring, and a school that opted
			// into e-mail for an announcement expects it to leave. The e-mail
			// path applies its own consent rule (see enqueueAnnouncementEmails).
			if fresh.SendEmail {
				if err := s.enqueueAnnouncementEmails(ctx, fresh); err != nil {
					return nil, fmt.Errorf("announcement: publish e-mails: %w", err)
				}
			}
		}
	}
	return s.Get(ctx, id)
}

func (s *service) notifyAnnouncementGuardians(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	if s.notifier == nil {
		return nil
	}
	var err error
	priority := notifications.PriorityNormal
	if a.Priority == usersModels.ParentAnnouncementPriorityImportant {
		priority = notifications.PriorityHigh
	}
	var accountIDs []int64
	seen := make(map[int64]struct{})
	if a.IsPoll() {
		recipients, err := s.repo.UnansweredReminderRecipients(ctx, a.GetTenantID(), a.ID)
		if err != nil {
			return fmt.Errorf("resolve poll guardian recipients: %w", err)
		}
		accountIDs = make([]int64, 0, len(recipients))
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
	} else {
		recipients, err := s.repo.AudienceRecipients(ctx, a.GetTenantID(), a.ID)
		if err != nil {
			return fmt.Errorf("resolve guardian recipients: %w", err)
		}
		accountIDs = make([]int64, 0, len(recipients))
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
	}
	if len(accountIDs) == 0 {
		return nil
	}
	candidateCount := len(accountIDs)

	// Consent narrows the audience the announcement itself defined. A nil
	// preference service means the dependency was never wired, which the
	// factory always does — the pre-consent behaviour is kept for that case so
	// a partially constructed service does not silently stop notifying. When a
	// service IS present, its answer is final: no consent, no push.
	if s.preferences != nil {
		accountIDs, err = s.preferences.FilterOptedIn(ctx, notifications.TypeParentAnnouncement, accountIDs)
		if err != nil {
			return fmt.Errorf("filter opted-in guardians: %w", err)
		}
		if len(accountIDs) < candidateCount {
			s.logger.Info("parent announcement push narrowed by consent",
				slog.Int64("announcement_id", a.ID),
				slog.Int("candidate_count", candidateCount),
				slog.Int("recipient_count", len(accountIDs)),
			)
		}
		if len(accountIDs) == 0 {
			return nil
		}
	}

	title, body := parentAnnouncementNotificationTitle, parentAnnouncementNotificationBody
	if a.IsPoll() {
		title, body = parentPollNotificationTitle, parentPollNotificationBody
	}
	err = s.notifier.Notify(ctx, notifications.Event{
		Type:     parentAnnouncementNotificationType,
		Title:    title,
		Body:     body,
		DeepLink: "/",
		Priority: priority,
		Audience: notifications.Audience{
			TenantID:           a.GetTenantID(),
			Scope:              notifications.ScopeGuardian,
			GuardianAccountIDs: accountIDs,
		},
	})
	if errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
		s.logger.Info("parent announcement push suppressed by tenant notification gate",
			slog.Int64("announcement_id", a.ID),
			slog.Int("recipient_count", len(accountIDs)),
			slog.String("reason", err.Error()),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("notify guardian audience: %w", err)
	}
	return nil
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
func (s *service) enqueueAnnouncementEmails(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	if s.outbox == nil {
		s.logger.Warn("parent announcement opted into e-mail but no outbox is wired",
			slog.Int64("announcement_id", a.ID))
		return nil
	}
	tenantID := a.GetTenantID()
	var recipients []*usersModels.AnnouncementRecipient
	if a.IsPoll() {
		pollRecipients, err := s.repo.UnansweredReminderRecipients(ctx, tenantID, a.ID)
		if err != nil {
			return fmt.Errorf("announcement: resolve poll e-mail recipients: %w", err)
		}
		recipients = make([]*usersModels.AnnouncementRecipient, 0, len(pollRecipients))
		for _, recipient := range pollRecipients {
			recipients = append(recipients, &usersModels.AnnouncementRecipient{
				AccountID: recipient.AccountID,
				Email:     recipient.Email,
				FirstName: recipient.FirstName,
				LastName:  recipient.LastName,
			})
		}
	} else {
		var err error
		recipients, err = s.repo.ResolveAudienceEmails(ctx, tenantID, a.ID)
		if err != nil {
			return fmt.Errorf("announcement: resolve audience e-mails: %w", err)
		}
	}
	if len(recipients) == 0 {
		return nil
	}
	if s.preferences != nil {
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
		remaining, err := s.preferences.FilterNotOptedOut(ctx, notifications.TypeParentAnnouncement, accountIDs)
		if err != nil {
			return fmt.Errorf("announcement: filter opted-out e-mail recipients: %w", err)
		}
		allowed := make(map[int64]struct{}, len(remaining))
		for _, accountID := range remaining {
			allowed[accountID] = struct{}{}
		}
		filtered := make([]*usersModels.AnnouncementRecipient, 0, len(recipients))
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
			return nil
		}
	}
	uniqueRecipients := make([]*usersModels.AnnouncementRecipient, 0, len(recipients))
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
	recipients = uniqueRecipients
	if len(recipients) == 0 {
		return nil
	}
	schoolName, err := s.repo.SchoolName(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("announcement: resolve school name: %w", err)
	}
	// Header/footer branding, resolved once for the whole batch. The same logo
	// URLs the enrollment mails use, so the announcement mail renders the school
	// logo (header) and the moto logo (footer) instead of the plain fallbacks.
	logoURL := s.resolveSchoolLogoURL(ctx, tenantID)
	motoLogoURL := emailbranding.MotoLogoURL(s.parentsURL)
	// A poll says so in the mail: the kicker and intro differ, the rest of the
	// template (and the deliberate absence of the body) is identical.
	kicker := defaultEmailKicker
	intro := ""
	if a.IsPoll() {
		kicker = pollEmailKicker
		intro = "wir bitten Sie um eine kurze Rückmeldung. Die Umfrage finden Sie im Eltern-Portal."
		if a.ResponseDeadline != nil {
			intro = fmt.Sprintf("wir bitten Sie um eine kurze Rückmeldung bis zum %s. Die Umfrage finden Sie im Eltern-Portal.",
				a.ResponseDeadline.Format("02.01.2006"))
		}
	}
	queued := 0
	for _, rcpt := range recipients {
		// Deliberately NO announcement body here: e-mail is the least trusted
		// channel, so the mail carries only the title plus a link into the
		// parent portal, where reading also counts for the read stats.
		if _, err := s.outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: platformModels.EmailKindParentAnnouncement,
			Payload: map[string]any{
				emailPayloadRecipient:   rcpt.Email,
				emailPayloadFirstName:   rcpt.FirstName,
				emailPayloadLastName:    rcpt.LastName,
				emailPayloadTitle:       a.Title,
				emailPayloadSchoolName:  schoolName,
				emailPayloadPortalURL:   s.parentsURL,
				emailPayloadLogoURL:     logoURL,
				emailPayloadMotoLogoURL: motoLogoURL,
				emailPayloadKicker:      kicker,
				emailPayloadIntro:       intro,
			},
			RelatedEntityType: relatedEntityTypeAnnouncement,
			RelatedEntityID:   a.ID,
		}); err != nil {
			return fmt.Errorf("announcement: enqueue e-mail: %w", err)
		}
		queued++
	}
	s.logger.Info("parent announcement e-mails queued",
		slog.Int64("announcement_id", a.ID),
		slog.Int("queued", queued),
	)
	return nil
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
	return emailbranding.SchoolLogoURL(s.parentsURL, raw)
}

// cancelPendingEmails cancels any not-yet-sent announcement and poll-reminder
// e-mails queued for this announcement. Called when it is retracted (unpublish)
// or deleted: both kinds of mail carry a portal link that must not be delivered
// after the announcement is no longer available.
// Leaving them would deliver a notification for an announcement staff has already
// pulled — and, on the documented unpublish -> edit -> republish correction path,
// a stale e-mail followed by a second corrected one. The cancel runs in the same
// request tenant tx as the state change, so it commits atomically. A missing
// outbox binding is a no-op. A cancel DB error is returned so the tenant tx rolls
// back cleanly (mirrors enqueueAnnouncementEmails): a failed statement poisons
// the tx anyway, so swallowing it would only mask the failure behind a later
// error.
func (s *service) cancelPendingEmails(ctx context.Context, id int64) error {
	if s.outbox == nil {
		return nil
	}
	cancelled := int64(0)
	for _, relatedType := range []string{relatedEntityTypeAnnouncement, relatedEntityTypePollReminder} {
		n, err := s.outbox.CancelPendingByRelatedEntity(ctx, relatedType, id, "announcement no longer available")
		if err != nil {
			return fmt.Errorf("announcement: cancel pending e-mails: %w", err)
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

func (s *service) Unpublish(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load for unpublish: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.IsPublished() {
		if err := s.repo.SetPublished(ctx, id, nil); err != nil {
			return nil, fmt.Errorf("announcement: unpublish: %w", err)
		}
		// Retract the queued opt-in e-mails too: the correction path is
		// unpublish -> edit -> republish, so a still-pending mail with the old
		// wording must not go out after the announcement is pulled.
		if err := s.cancelPendingEmails(ctx, id); err != nil {
			return nil, err
		}
		a.PublishedAt = nil
		s.logger.Info("parent announcement unpublished", slog.Int64("announcement_id", id))
	}
	return s.Get(ctx, id)
}

func (s *service) Stats(ctx context.Context, id int64) (*usersModels.AnnouncementStats, error) {
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
func (s *service) Recipients(ctx context.Context, id int64) ([]*usersModels.AnnouncementRecipientStatus, error) {
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

// normalizeInput validates and trims the payload and returns the target rows to
// persist. It enforces: non-empty title (<=200)/body (<=4000), a known priority, an
// optional absolute http(s) link, at least one target, and per-type ref
// consistency (class -> text, group/AG/student -> id, school_all/
// pending_enrollment -> neither). Duplicate targets are collapsed.
func normalizeInput(in *Input) ([]*usersModels.ParentAnnouncementTarget, error) {
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
		in.Priority = usersModels.ParentAnnouncementPriorityInfo
	}
	if !usersModels.ValidAnnouncementPriority(in.Priority) {
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

	seen := make(map[string]bool, len(in.Targets))
	out := make([]*usersModels.ParentAnnouncementTarget, 0, len(in.Targets))
	for _, t := range in.Targets {
		if !usersModels.ValidAnnouncementTargetType(t.TargetType) {
			return nil, fmt.Errorf("%w: target_type %q", ErrValidation, t.TargetType)
		}
		target := &usersModels.ParentAnnouncementTarget{TargetType: t.TargetType}
		switch t.TargetType {
		case usersModels.AnnouncementTargetClass:
			if t.RefText == nil || strings.TrimSpace(*t.RefText) == "" {
				return nil, fmt.Errorf("%w: class target needs ref_text", ErrValidation)
			}
			text := strings.TrimSpace(*t.RefText)
			target.TargetRefText = &text
		case usersModels.AnnouncementTargetGroup,
			usersModels.AnnouncementTargetActivityGroup,
			usersModels.AnnouncementTargetStudent:
			if t.RefID == nil || *t.RefID <= 0 {
				return nil, fmt.Errorf("%w: %s target needs ref_id", ErrValidation, t.TargetType)
			}
			target.TargetRefID = t.RefID
		case usersModels.AnnouncementTargetSchoolAll:
			// no ref
		case usersModels.AnnouncementTargetPendingEnrollment:
			// A poll is answered per CHILD, and an open application has no
			// enrolled child yet — the applicant would see a question with no
			// answer row. The staff form hides the option for polls; refuse it
			// here too so the API cannot create that dead end.
			if in.ResponseType == usersModels.ParentAnnouncementResponseSingleChoice ||
				in.ResponseType == usersModels.ParentAnnouncementResponseMultiChoice {
				return nil, fmt.Errorf("%w: pending_enrollment cannot be targeted by a poll", ErrValidation)
			}
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

func dedupeKey(t *usersModels.ParentAnnouncementTarget) string {
	switch {
	case t.TargetRefID != nil:
		return fmt.Sprintf("%s:id:%d", t.TargetType, *t.TargetRefID)
	case t.TargetRefText != nil:
		return fmt.Sprintf("%s:txt:%s", t.TargetType, *t.TargetRefText)
	default:
		return t.TargetType
	}
}
