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
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OutboxEnqueuer is the slice of the email outbox service this package needs:
// queue one e-mail row inside the current tenant transaction.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error)
}

const (
	maxTitleLen   = 200
	maxLinkURLLen = 2000
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
}

// ServiceConfig is the dependency bundle. Outbox + ParentsURL are optional: a
// nil Outbox disables e-mail notification (the in-app feed still works).
type ServiceConfig struct {
	Repo       usersModels.ParentAnnouncementRepository
	Settings   configService.SettingsService
	Outbox     OutboxEnqueuer
	ParentsURL string
	Logger     *slog.Logger
}

type service struct {
	repo       usersModels.ParentAnnouncementRepository
	settings   configService.SettingsService
	outbox     OutboxEnqueuer
	parentsURL string
	logger     *slog.Logger
}

// NewService wires the staff announcement service.
func NewService(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		repo:       cfg.Repo,
		settings:   cfg.Settings,
		outbox:     cfg.Outbox,
		parentsURL: cfg.ParentsURL,
		logger:     logger,
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

	a := &usersModels.ParentAnnouncement{
		Title:                   in.Title,
		Body:                    in.Body,
		Priority:                in.Priority,
		LinkURL:                 in.LinkURL,
		RequiresAcknowledgement: in.RequiresAcknowledgement,
		SendEmail:               in.SendEmail,
		ExpiresAt:               in.ExpiresAt,
		Active:                  true,
		CreatedBy:               createdBy,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("announcement: create: %w", err)
	}
	if err := s.repo.ReplaceTargets(ctx, a.GetTenantID(), a.ID, targets); err != nil {
		return nil, fmt.Errorf("announcement: set targets: %w", err)
	}
	a.Targets = targets
	s.logger.Info("parent announcement created",
		slog.Int64("announcement_id", a.ID),
		slog.Int64("created_by", createdBy),
		slog.Int("target_count", len(targets)),
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

	a.Title = in.Title
	a.Body = in.Body
	a.Priority = in.Priority
	a.LinkURL = in.LinkURL
	a.RequiresAcknowledgement = in.RequiresAcknowledgement
	a.SendEmail = in.SendEmail
	a.ExpiresAt = in.ExpiresAt
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
		return nil, fmt.Errorf("announcement: update targets: %w", err)
	}
	a.Targets = targets
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
		// Atomic publish: PublishIfDraft's UPDATE is guarded by
		// published_at IS NULL, so only ONE of two concurrent publishes flips the
		// row. Enqueue the opt-in e-mails only for that winner — a double-click or
		// two-tab race must not double-send the parent notification.
		published, err := s.repo.PublishIfDraft(ctx, id, now)
		if err != nil {
			return nil, fmt.Errorf("announcement: publish: %w", err)
		}
		if published {
			a.PublishedAt = &now
			s.logger.Info("parent announcement published", slog.Int64("announcement_id", id))
			if a.SendEmail {
				if err := s.enqueueAnnouncementEmails(ctx, a); err != nil {
					return nil, fmt.Errorf("announcement: publish e-mails: %w", err)
				}
			}
		}
	}
	return s.Get(ctx, id)
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
	recipients, err := s.repo.ResolveAudienceEmails(ctx, tenantID, a.ID)
	if err != nil {
		return fmt.Errorf("announcement: resolve audience e-mails: %w", err)
	}
	if len(recipients) == 0 {
		return nil
	}
	schoolName, err := s.repo.SchoolName(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("announcement: resolve school name: %w", err)
	}
	queued := 0
	for _, rcpt := range recipients {
		// Deliberately NO announcement body here: e-mail is the least trusted
		// channel, so the mail carries only the title plus a link into the
		// parent portal, where reading also counts for the read stats.
		if _, err := s.outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: platformModels.EmailKindParentAnnouncement,
			Payload: map[string]any{
				emailPayloadRecipient:  rcpt.Email,
				emailPayloadFirstName:  rcpt.FirstName,
				emailPayloadLastName:   rcpt.LastName,
				emailPayloadTitle:      a.Title,
				emailPayloadSchoolName: schoolName,
				emailPayloadPortalURL:  s.parentsURL,
			},
			RelatedEntityType: "parent_announcement",
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
// persist. It enforces: non-empty title (<=200)/body, a known priority, an
// optional absolute http(s) link, at least one target, and per-type ref
// consistency (class -> text, group/AG/student -> id, school_all/
// pending_enrollment -> neither). Duplicate targets are collapsed.
func normalizeInput(in *Input) ([]*usersModels.ParentAnnouncementTarget, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if in.Title == "" || len([]rune(in.Title)) > maxTitleLen {
		return nil, fmt.Errorf("%w: title", ErrValidation)
	}
	if in.Body == "" {
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
		case usersModels.AnnouncementTargetSchoolAll, usersModels.AnnouncementTargetPendingEnrollment:
			// no ref
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

// TenantID returns the current request's tenant from context, or 0. Exposed for
// handlers that need it for logging; the service itself relies on the request
// tenant tx for scoping.
func TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }
