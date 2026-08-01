package parent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

var (
	// ErrAnnouncementNotFound is returned when an announcement does not exist,
	// is not live, or the guardian is not in its audience — all collapse to one
	// error so the endpoint never reveals an announcement the guardian may not
	// see. Handler maps it to 404.
	ErrAnnouncementNotFound = errors.New("parent: announcement not found")
	// ErrAnnouncementAckNotRequired is returned when a guardian tries to
	// acknowledge an announcement that does not require acknowledgement.
	// Handler maps it to 400.
	ErrAnnouncementAckNotRequired = errors.New("parent: announcement does not require acknowledgement")
	// ErrAnnouncementStale is returned when the guardian's read/ack request
	// carries a published_at that no longer matches the live announcement — the
	// documented correction flow (unpublish -> edit -> republish) reuses the
	// same id but stamps a fresh published_at and clears the old read rows, so a
	// stale client that loaded the RETRACTED wording must not record a read/ack
	// against the corrected wording it never saw. Handler maps it to 409 so the
	// portal can refetch and show the current text. Checked only AFTER audience
	// authorization AND the school's parent-news flag (both collapse to 404
	// first), so it never reveals an announcement to a non-audience caller or one
	// whose school disabled the feature.
	ErrAnnouncementStale = errors.New("parent: announcement changed since it was loaded")
	// ErrAnnouncementNotAPoll is returned when an answer is submitted for an
	// announcement that asks no question. Handler maps it to 400.
	ErrAnnouncementNotAPoll = errors.New("parent: announcement does not accept answers")
	// ErrPollClosed is returned when the answer deadline has passed. Handler maps
	// it to 409 so the portal refetches and shows the closed state instead of
	// silently dropping the answer.
	ErrPollClosed = errors.New("parent: the answer deadline for this poll has passed")
	// ErrInvalidPollResponse is returned when a selection is not made up of this
	// poll's options, repeats an option, or violates the poll's response type.
	// Handler maps it to 400; importantly, validation happens before a prior
	// answer can be replaced.
	ErrInvalidPollResponse = errors.New("parent: invalid poll response")
	// ErrChildNotAnswerable is returned when the guardian may not answer for the
	// given child — not their child, or a child this poll does not reach. It
	// collapses with "unknown child" on purpose so a parent token cannot probe
	// student ids. Handler maps it to 404.
	ErrChildNotAnswerable = errors.New("parent: no answerable child for this poll")
)

// ListAnnouncements returns the guardian's parent-news feed: published, active,
// unexpired announcements they are targeted by across all their children's
// schools, newest-published first, each with the guardian's read/ack state.
// Schools that have turned the news feature off are excluded so the feed agrees
// with the unread badge. Cross-tenant.
func (s *service) ListAnnouncements(ctx context.Context, accountID int64) ([]*usersModels.AnnouncementFeedItem, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	tenantIDs, err := s.newsEnabledTenants(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(tenantIDs) == 0 {
		return []*usersModels.AnnouncementFeedItem{}, nil
	}
	var out []*usersModels.AnnouncementFeedItem
	if txErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		rows, err := s.AnnouncementRepo.ListFeedForAccount(adminCtx, accountID, tenantIDs)
		if err != nil {
			return err
		}
		out = rows
		return s.attachPollData(adminCtx, accountID, rows)
	}); txErr != nil {
		return nil, fmt.Errorf("parent: list announcements: %w", txErr)
	}
	return out, nil
}

// attachPollData loads the answer options and the guardian's answerable children
// for the poll items of a feed page, in two batched queries rather than one per
// item. Plain Mitteilungen are skipped entirely, so a feed without polls costs
// nothing extra.
func (s *service) attachPollData(ctx context.Context, accountID int64, items []*usersModels.AnnouncementFeedItem) error {
	pollIDs := make([]int64, 0, len(items))
	byID := make(map[int64]*usersModels.AnnouncementFeedItem, len(items))
	for _, item := range items {
		if !item.IsPoll() {
			continue
		}
		pollIDs = append(pollIDs, item.ID)
		byID[item.ID] = item
		item.Options = []*usersModels.ParentAnnouncementOption{}
		item.Children = []*usersModels.AnnouncementPollChild{}
	}
	if len(pollIDs) == 0 {
		return nil
	}
	options, err := s.AnnouncementRepo.ListOptionsForAnnouncements(ctx, pollIDs)
	if err != nil {
		return fmt.Errorf("parent: load poll options: %w", err)
	}
	for _, o := range options {
		if item, ok := byID[o.AnnouncementID]; ok {
			item.Options = append(item.Options, o)
		}
	}
	children, err := s.AnnouncementRepo.AnswerableChildren(ctx, accountID, pollIDs)
	if err != nil {
		return fmt.Errorf("parent: load answerable children: %w", err)
	}
	for _, c := range children {
		if item, ok := byID[c.AnnouncementID]; ok {
			item.Children = append(item.Children, c)
		}
	}
	return nil
}

// UnreadAnnouncementCount returns how many feed announcements the guardian has
// not read across all their (news-enabled) children's schools — the parent
// portal's Neuigkeiten badge. Cross-tenant.
func (s *service) UnreadAnnouncementCount(ctx context.Context, accountID int64) (int, error) {
	if accountID <= 0 {
		return 0, fmt.Errorf("parent: account_id must be positive")
	}
	tenantIDs, err := s.newsEnabledTenants(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	var count int
	if txErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		n, err := s.AnnouncementRepo.CountUnreadForAccount(adminCtx, accountID, tenantIDs)
		if err != nil {
			return err
		}
		count = n
		return nil
	}); txErr != nil {
		return 0, fmt.Errorf("parent: unread announcement count: %w", txErr)
	}
	return count, nil
}

// MarkAnnouncementRead records that the guardian opened an announcement. It
// resolves the announcement (cross-tenant), refuses one that is not live or that
// the account is not in the audience of (both surface as ErrAnnouncementNotFound
// so the endpoint never confirms an announcement the guardian may not see),
// rejects a stale request whose expectedPublishedAt no longer matches the live
// announcement (ErrAnnouncementStale), then upserts the read row.
// expectedPublishedAt is the published_at the client held when it loaded the
// feed item.
func (s *service) MarkAnnouncementRead(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time) error {
	return s.stampAnnouncement(ctx, accountID, announcementID, expectedPublishedAt, false)
}

// AcknowledgeAnnouncement records an explicit "gelesen und bestätigt". It is
// only valid for an announcement that requires acknowledgement; otherwise it
// returns ErrAnnouncementAckNotRequired. Same resolution/authorization (incl.
// the stale-version check) as MarkAnnouncementRead.
func (s *service) AcknowledgeAnnouncement(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time) error {
	return s.stampAnnouncement(ctx, accountID, announcementID, expectedPublishedAt, true)
}

// stampAnnouncement is the shared resolve+authorize+write path for read and
// acknowledge. ack=true stamps acknowledged_at and requires the announcement to
// have requires_acknowledgement set. expectedPublishedAt is verified against the
// live announcement so a stale client (one that loaded a since-retracted
// wording) cannot record a read/ack against the corrected announcement.
func (s *service) stampAnnouncement(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time, ack bool) error {
	if accountID <= 0 || announcementID <= 0 {
		return fmt.Errorf("parent: account_id and announcement_id must be positive")
	}
	// Phase 1: resolve + authorize inside the admin tx, capturing what the write
	// needs (tenant + ack requirement).
	var announcementTenantID int64
	var requiresAck bool
	var announcementPublishedAt time.Time
	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load announcement: %w", err)
		}
		if a == nil || !announcementIsLive(a) {
			return ErrAnnouncementNotFound
		}
		// Authorize audience membership BEFORE any other 4xx: an unmatched caller
		// must always get 404 so a parent token cannot enumerate live announcement
		// IDs (or their ack flags) by distinguishing ack-not-required (400) from
		// not-found (404) responses across tenants.
		matched, err := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
		if err != nil {
			return fmt.Errorf("parent: match announcement audience: %w", err)
		}
		if !matched {
			return ErrAnnouncementNotFound
		}
		announcementTenantID = a.GetTenantID()
		requiresAck = a.RequiresAcknowledgement
		// Capture published_at for the stale-version check, which is deferred
		// until AFTER the parent-news flag check below so a disabled school
		// collapses to 404 before a mismatch could surface a 409.
		// announcementIsLive guarantees a.PublishedAt is non-nil here.
		announcementPublishedAt = *a.PublishedAt
		return nil
	}); err != nil {
		return err
	}
	// Re-check the school's parent-news flag before stamping any read/ack row.
	// The list + unread-badge paths already hide a school that turned the feature
	// off (newsEnabledTenants); a direct read/acknowledge on a stale page or a
	// known ID must not keep collecting interaction stats for that disabled
	// school, so a disabled school collapses to the same 404 the feed shows. Runs
	// OUTSIDE the admin tx because ResolveBoolForTenant opens its own tenant tx.
	// Fails CLOSED: an unwired settings service or a resolve error excludes the
	// school just like the feed does.
	if s.Settings == nil {
		return ErrAnnouncementNotFound
	}
	on, err := s.Settings.ResolveBoolForTenant(ctx, announcementTenantID, configModel.KeyParentNewsEnabled)
	if err != nil {
		s.Logger.Warn("parent: resolve parent_news_enabled failed, refusing stamp",
			slog.Int64("tenant_id", announcementTenantID),
			slog.Int64("announcement_id", announcementID),
			slog.String("error", err.Error()),
		)
		return ErrAnnouncementNotFound
	}
	if !on {
		return ErrAnnouncementNotFound
	}
	// Version check AFTER audience authorization AND the parent-news flag (both
	// return 404 first) so a stale request can never reveal a hidden or
	// disabled-school announcement via a 409: the client echoes the published_at
	// it loaded. On the unpublish -> edit -> republish correction path the id is
	// reused but published_at is re-stamped, so a mismatch means the client is
	// acting on retracted wording — reject it rather than count a read/ack for
	// text the guardian never saw.
	if !announcementPublishedAt.Equal(expectedPublishedAt) {
		return ErrAnnouncementStale
	}
	if ack && !requiresAck {
		return ErrAnnouncementAckNotRequired
	}
	// Phase 2: stamp the read/ack row in a fresh admin tx. Phase 1 ran in an
	// earlier tx and the settings check opened its own, so the announcement may
	// have been retracted/republished since. The repo write is guarded on the
	// announcement still being live at expectedPublishedAt (the version verified
	// in Phase 1), and reports whether it applied: a correction that slipped in
	// between Phase 1 and here makes the guard miss and the stamp a no-op, which
	// must surface as ErrAnnouncementStale (409) — not a silent 200 that would let
	// the client mark the retracted wording read/acknowledged locally while the DB
	// and staff stats stay unchanged.
	return tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		if ack {
			applied, err := s.AnnouncementRepo.MarkAcknowledged(adminCtx, announcementTenantID, announcementID, accountID, expectedPublishedAt)
			if err != nil {
				return err
			}
			if !applied {
				return ErrAnnouncementStale
			}
			s.Logger.Info("parent acknowledged announcement",
				slog.Int64("account_id", accountID),
				slog.Int64("announcement_id", announcementID),
			)
			return nil
		}
		applied, err := s.AnnouncementRepo.MarkRead(adminCtx, announcementTenantID, announcementID, accountID, expectedPublishedAt)
		if err != nil {
			return err
		}
		if !applied {
			return ErrAnnouncementStale
		}
		return nil
	})
}

// RespondToAnnouncement records the guardian's answer to a poll for ONE child.
// optionIDs replaces the child's previous selection; an empty slice withdraws
// the answer. The resolution mirrors stampAnnouncement — audience/feature-flag
// checks collapse to 404 before any other 4xx so a parent token can neither
// enumerate announcement ids nor probe student ids — and adds the poll-specific
// checks: it must be a poll, the deadline must not have passed, and the account
// must be a portal-enabled guardian of a child the poll actually reaches.
func (s *service) RespondToAnnouncement(ctx context.Context, accountID, announcementID, studentID int64, optionIDs []int64, expectedPublishedAt time.Time) error {
	if accountID <= 0 || announcementID <= 0 || studentID <= 0 {
		return fmt.Errorf("parent: account_id, announcement_id and student_id must be positive")
	}
	var announcementTenantID int64
	var announcementPublishedAt time.Time
	var isPoll bool
	var responseType string
	var deadline *time.Time
	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load announcement: %w", err)
		}
		if a == nil || !announcementIsLive(a) {
			return ErrAnnouncementNotFound
		}
		// Authorize the CHILD, not just the account: AccountMayAnswerForStudent
		// checks the guardian link and the poll's audience in one query, so a
		// guardian can neither answer for someone else's child nor for a child the
		// poll does not target.
		allowed, err := s.AnnouncementRepo.AccountMayAnswerForStudent(adminCtx, a.GetTenantID(), announcementID, accountID, studentID)
		if err != nil {
			return fmt.Errorf("parent: check answer permission: %w", err)
		}
		if !allowed {
			// A guardian outside the audience entirely must not be able to tell
			// "wrong child" from "no such announcement", so both are 404 — but a
			// guardian who IS in the audience gets the more useful child error.
			matched, matchErr := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
			if matchErr != nil {
				return fmt.Errorf("parent: match announcement audience: %w", matchErr)
			}
			if !matched {
				return ErrAnnouncementNotFound
			}
			return ErrChildNotAnswerable
		}
		announcementTenantID = a.GetTenantID()
		announcementPublishedAt = *a.PublishedAt
		isPoll = a.IsPoll()
		responseType = a.ResponseType
		deadline = a.ResponseDeadline
		return nil
	}); err != nil {
		return err
	}
	// Same fail-closed feature check as the read/ack path: a school that turned
	// parent news off must stop collecting answers, not just stop showing them.
	if s.Settings == nil {
		return ErrAnnouncementNotFound
	}
	on, err := s.Settings.ResolveBoolForTenant(ctx, announcementTenantID, configModel.KeyParentNewsEnabled)
	if err != nil {
		s.Logger.Warn("parent: resolve parent_news_enabled failed, refusing poll answer",
			slog.Int64("tenant_id", announcementTenantID),
			slog.Int64("announcement_id", announcementID),
			slog.String("error", err.Error()),
		)
		return ErrAnnouncementNotFound
	}
	if !on {
		return ErrAnnouncementNotFound
	}
	if !announcementPublishedAt.Equal(expectedPublishedAt) {
		return ErrAnnouncementStale
	}
	if !isPoll {
		return ErrAnnouncementNotAPoll
	}
	if deadline != nil && !deadline.After(time.Now()) {
		return ErrPollClosed
	}
	var options []*usersModels.ParentAnnouncementOption
	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		var err error
		options, err = s.AnnouncementRepo.ListOptions(adminCtx, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load poll options: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if !validPollSelection(responseType, options, optionIDs) {
		return ErrInvalidPollResponse
	}
	// Phase 2: the write re-evaluates liveness, version AND deadline inside the
	// same statement, so a deadline that passes between here and the write cannot
	// let a late answer through. A missed guard is a 409, never a silent 200.
	return tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		applied, err := s.AnnouncementRepo.SetResponse(adminCtx, announcementTenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
		if err != nil {
			return err
		}
		if !applied {
			return ErrAnnouncementStale
		}
		s.Logger.Info("parent answered announcement poll",
			slog.Int64("account_id", accountID),
			slog.Int64("announcement_id", announcementID),
			slog.Int64("student_id", studentID),
			slog.Int("option_count", len(optionIDs)),
		)
		return nil
	})
}

func validPollSelection(responseType string, options []*usersModels.ParentAnnouncementOption, optionIDs []int64) bool {
	if responseType != usersModels.ParentAnnouncementResponseSingleChoice &&
		responseType != usersModels.ParentAnnouncementResponseMultiChoice {
		return false
	}
	if len(optionIDs) == 0 {
		return true
	}
	optionSet := make(map[int64]struct{}, len(options))
	for _, option := range options {
		optionSet[option.ID] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(optionIDs))
	for _, optionID := range optionIDs {
		if _, exists := optionSet[optionID]; !exists {
			return false
		}
		if _, duplicate := seen[optionID]; duplicate {
			return false
		}
		seen[optionID] = struct{}{}
	}
	return responseType != usersModels.ParentAnnouncementResponseSingleChoice || len(optionIDs) == 1
}

// newsEnabledTenants returns the distinct tenants a guardian can receive news
// from, filtered to those with the parent-news feature enabled. The tenant set
// is the union of two sources:
//
//   - schools where the account has a linked child (childRepo), and
//   - schools where the account has an open (non-withdrawn) enrollment request.
//
// The enrollment source is what makes the pending_enrollment target visible:
// an applicant can be in an announcement's audience via
// enrollment.requests.guardian_account_id BEFORE any child is linked at that
// school, so deriving tenants from children alone hid those announcements from
// the feed and unread badge while stats + e-mail still counted the applicant.
// A widened tenant set never over-exposes anything: the per-announcement
// audience predicate (reachedPredicate in the repository) still decides actual
// membership, so tenants reached only through a decided/withdrawn request simply
// yield no matching announcements.
//
// ResolveBoolForTenant opens its own tenant tx, so it runs OUTSIDE the admin tx
// (mirrors the messaging UnreadMessageCount pattern). Fails CLOSED: a school is
// included only when its flag resolves to true, so a disabled school never leaks
// announcements.
func (s *service) newsEnabledTenants(ctx context.Context, accountID int64) ([]int64, error) {
	var allTenantIDs []int64
	seen := make(map[int64]bool)
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			allTenantIDs = append(allTenantIDs, id)
		}
	}
	if txErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		children, err := s.ChildRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		for _, id := range distinctTenantIDs(children) {
			add(id)
		}
		requests, err := s.EnrollmentRequestRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		for _, req := range requests {
			if req == nil || req.WithdrawnAt != nil {
				continue
			}
			add(req.TenantID)
		}
		return nil
	}); txErr != nil {
		return nil, fmt.Errorf("parent: resolve news tenants: %w", txErr)
	}
	if len(allTenantIDs) == 0 || s.Settings == nil {
		return nil, nil
	}
	enabled := make([]int64, 0, len(allTenantIDs))
	for _, tenantID := range allTenantIDs {
		on, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyParentNewsEnabled)
		if err != nil {
			// Fail CLOSED on a resolve error: news is opt-in (default off), so a
			// transient settings hiccup must not surface a school's announcements.
			s.Logger.Warn("parent: resolve parent_news_enabled failed, excluding tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			continue
		}
		if on {
			enabled = append(enabled, tenantID)
		}
	}
	return enabled, nil
}

// announcementIsLive reports whether an announcement is currently visible to
// guardians: active, published, publish time reached, not past expiry.
func announcementIsLive(a *usersModels.ParentAnnouncement) bool {
	if a == nil || !a.Active || a.PublishedAt == nil {
		return false
	}
	now := time.Now()
	if a.PublishedAt.After(now) {
		return false
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return false
	}
	return true
}
