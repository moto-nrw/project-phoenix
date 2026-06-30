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
	tenantIDs, err := s.newsEnabledChildTenants(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(tenantIDs) == 0 {
		return []*usersModels.AnnouncementFeedItem{}, nil
	}
	var out []*usersModels.AnnouncementFeedItem
	if txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		rows, err := s.announcementRepo.ListFeedForAccount(adminCtx, accountID, tenantIDs)
		if err != nil {
			return err
		}
		out = rows
		return nil
	}); txErr != nil {
		return nil, fmt.Errorf("parent: list announcements: %w", txErr)
	}
	return out, nil
}

// UnreadAnnouncementCount returns how many feed announcements the guardian has
// not read across all their (news-enabled) children's schools — the parent
// portal's Neuigkeiten badge. Cross-tenant.
func (s *service) UnreadAnnouncementCount(ctx context.Context, accountID int64) (int, error) {
	if accountID <= 0 {
		return 0, fmt.Errorf("parent: account_id must be positive")
	}
	tenantIDs, err := s.newsEnabledChildTenants(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	var count int
	if txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		n, err := s.announcementRepo.CountUnreadForAccount(adminCtx, accountID, tenantIDs)
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
// so the endpoint never confirms an announcement the guardian may not see), then
// upserts the read row.
func (s *service) MarkAnnouncementRead(ctx context.Context, accountID, announcementID int64) error {
	return s.stampAnnouncement(ctx, accountID, announcementID, false)
}

// AcknowledgeAnnouncement records an explicit "gelesen und bestätigt". It is
// only valid for an announcement that requires acknowledgement; otherwise it
// returns ErrAnnouncementAckNotRequired. Same resolution/authorization as
// MarkAnnouncementRead.
func (s *service) AcknowledgeAnnouncement(ctx context.Context, accountID, announcementID int64) error {
	return s.stampAnnouncement(ctx, accountID, announcementID, true)
}

// stampAnnouncement is the shared resolve+authorize+write path for read and
// acknowledge. ack=true stamps acknowledged_at and requires the announcement to
// have requires_acknowledgement set.
func (s *service) stampAnnouncement(ctx context.Context, accountID, announcementID int64, ack bool) error {
	if accountID <= 0 || announcementID <= 0 {
		return fmt.Errorf("parent: account_id and announcement_id must be positive")
	}
	return tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		a, err := s.announcementRepo.FindByID(adminCtx, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load announcement: %w", err)
		}
		if a == nil || !announcementIsLive(a) {
			return ErrAnnouncementNotFound
		}
		if ack && !a.RequiresAcknowledgement {
			return ErrAnnouncementAckNotRequired
		}
		matched, err := s.announcementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
		if err != nil {
			return fmt.Errorf("parent: match announcement audience: %w", err)
		}
		if !matched {
			return ErrAnnouncementNotFound
		}
		if ack {
			if err := s.announcementRepo.MarkAcknowledged(adminCtx, a.GetTenantID(), announcementID, accountID); err != nil {
				return err
			}
			s.logger.Info("parent acknowledged announcement",
				slog.Int64("account_id", accountID),
				slog.Int64("announcement_id", announcementID),
			)
			return nil
		}
		return s.announcementRepo.MarkRead(adminCtx, a.GetTenantID(), announcementID, accountID)
	})
}

// newsEnabledChildTenants returns the distinct tenants the guardian has children
// at, filtered to those with the parent-news feature enabled. ResolveBoolForTenant
// opens its own tenant tx, so it runs OUTSIDE the children admin tx (mirrors the
// messaging UnreadMessageCount pattern). Fails CLOSED: a school is included only
// when its flag resolves to true, so a disabled school never leaks announcements.
func (s *service) newsEnabledChildTenants(ctx context.Context, accountID int64) ([]int64, error) {
	var allTenantIDs []int64
	if txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		children, err := s.childRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		allTenantIDs = distinctTenantIDs(children)
		return nil
	}); txErr != nil {
		return nil, fmt.Errorf("parent: resolve child tenants: %w", txErr)
	}
	if len(allTenantIDs) == 0 || s.settings == nil {
		return nil, nil
	}
	enabled := make([]int64, 0, len(allTenantIDs))
	for _, tenantID := range allTenantIDs {
		on, err := s.settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyParentNewsEnabled)
		if err != nil {
			// Fail CLOSED on a resolve error: news is opt-in (default off), so a
			// transient settings hiccup must not surface a school's announcements.
			s.logger.Warn("parent: resolve parent_news_enabled failed, excluding tenant",
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
