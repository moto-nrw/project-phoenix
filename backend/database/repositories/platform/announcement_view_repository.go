package platform

import (
	"context"
	"database/sql"
	"errors"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tablePlatformAnnouncementViews      = "platform.announcement_views"
	tablePlatformAnnouncementViewsAlias = `platform.announcement_views AS "view"`
)

// AnnouncementViewRepository implements platform.AnnouncementViewRepository interface
type AnnouncementViewRepository struct {
	db *bun.DB
}

// NewAnnouncementViewRepository creates a new AnnouncementViewRepository
func NewAnnouncementViewRepository(db *bun.DB) platform.AnnouncementViewRepository {
	return &AnnouncementViewRepository{db: db}
}

// MarkSeen marks an announcement as seen by a user
func (r *AnnouncementViewRepository) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	view := &platform.AnnouncementView{
		UserID:         userID,
		AnnouncementID: announcementID,
		SeenAt:         time.Now(),
		Dismissed:      false,
	}

	_, err := r.db.NewInsert().
		Model(view).
		ModelTableExpr(tablePlatformAnnouncementViews).
		On("CONFLICT (user_id, announcement_id) DO UPDATE").
		Set("seen_at = EXCLUDED.seen_at").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark announcement seen",
			Err: err,
		}
	}

	return nil
}

// MarkDismissed marks an announcement as dismissed by a user
func (r *AnnouncementViewRepository) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	view := &platform.AnnouncementView{
		UserID:         userID,
		AnnouncementID: announcementID,
		SeenAt:         time.Now(),
		Dismissed:      true,
	}

	_, err := r.db.NewInsert().
		Model(view).
		ModelTableExpr(tablePlatformAnnouncementViews).
		On("CONFLICT (user_id, announcement_id) DO UPDATE").
		Set("seen_at = EXCLUDED.seen_at").
		Set("dismissed = true").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark announcement dismissed",
			Err: err,
		}
	}

	return nil
}

// GetUnreadForUser retrieves all unread active announcements for a user filtered by roles
func (r *AnnouncementViewRepository) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error) {
	var announcements []*platform.Announcement
	now := time.Now()

	// Use raw SQL for complex query with aliases to avoid BUN's quote escaping issues
	// target_roles = '{}' means all roles can see it, otherwise check overlap with user's roles
	err := r.db.NewRaw(`
		SELECT a.*
		FROM platform.announcements a
		LEFT JOIN platform.announcement_views v
			ON v.announcement_id = a.id AND v.user_id = ?
		WHERE a.active = true
			AND a.published_at IS NOT NULL
			AND a.published_at <= ?
			AND (a.expires_at IS NULL OR a.expires_at > ?)
			AND (v.seen_at IS NULL OR v.dismissed = false)
			AND (a.target_roles = '{}' OR EXISTS (
				SELECT 1 FROM unnest(a.target_roles) AS r WHERE r IN (?)))
		ORDER BY a.published_at DESC
	`, userID, now, now, bun.In(userRoles)).Scan(ctx, &announcements)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get unread announcements for user",
			Err: err,
		}
	}

	return announcements, nil
}

// CountUnread counts unread announcements for a user filtered by roles
func (r *AnnouncementViewRepository) CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error) {
	now := time.Now()

	var count int
	err := r.db.NewRaw(`
		SELECT COUNT(*)
		FROM platform.announcements a
		LEFT JOIN platform.announcement_views v
			ON v.announcement_id = a.id AND v.user_id = ?
		WHERE a.active = true
			AND a.published_at IS NOT NULL
			AND a.published_at <= ?
			AND (a.expires_at IS NULL OR a.expires_at > ?)
			AND (v.seen_at IS NULL OR v.dismissed = false)
			AND (a.target_roles = '{}' OR EXISTS (
				SELECT 1 FROM unnest(a.target_roles) AS r WHERE r IN (?)))
	`, userID, now, now, bun.In(userRoles)).Scan(ctx, &count)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count unread announcements",
			Err: err,
		}
	}

	return count, nil
}

// GetStats retrieves view statistics for an announcement
func (r *AnnouncementViewRepository) GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
	stats := &platform.AnnouncementStats{
		AnnouncementID: announcementID,
	}

	// Get the target_roles for this announcement
	var targetRoles []string
	err := r.db.NewRaw(`
		SELECT COALESCE(target_roles, '{}') FROM platform.announcements WHERE id = ?
	`, announcementID).Scan(ctx, &targetRoles)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get announcement target_roles",
			Err: err,
		}
	}

	// Count target users (users with matching roles)
	if len(targetRoles) == 0 {
		// All users can see it - count all accounts with roles
		err = r.db.NewRaw(`
			SELECT COUNT(DISTINCT acc.id)
			FROM auth.accounts acc
			WHERE acc.id IS NOT NULL
		`).Scan(ctx, &stats.TargetCount)
	} else {
		// Only users with matching roles
		err = r.db.NewRaw(`
			SELECT COUNT(DISTINCT acc.id)
			FROM auth.accounts acc
			JOIN auth.account_roles ar ON ar.account_id = acc.id
			JOIN auth.roles r ON r.id = ar.role_id
			WHERE r.name IN (?)
		`, bun.In(targetRoles)).Scan(ctx, &stats.TargetCount)
	}
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count target users",
			Err: err,
		}
	}

	// Count seen and dismissed
	err = r.db.NewRaw(`
		SELECT
			COALESCE(SUM(CASE WHEN seen_at IS NOT NULL THEN 1 ELSE 0 END), 0) as seen_count,
			COALESCE(SUM(CASE WHEN dismissed = true THEN 1 ELSE 0 END), 0) as dismissed_count
		FROM platform.announcement_views
		WHERE announcement_id = ?
	`, announcementID).Scan(ctx, &stats.SeenCount, &stats.DismissedCount)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get announcement view stats",
			Err: err,
		}
	}

	return stats, nil
}

// HasSeen checks if a user has seen a specific announcement
func (r *AnnouncementViewRepository) HasSeen(ctx context.Context, userID, announcementID int64) (bool, error) {
	view := new(platform.AnnouncementView)
	err := r.db.NewSelect().
		Model(view).
		ModelTableExpr(tablePlatformAnnouncementViewsAlias).
		Where(`"view".user_id = ?`, userID).
		Where(`"view".announcement_id = ?`, announcementID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, &modelBase.DatabaseError{
			Op:  "check if announcement seen",
			Err: err,
		}
	}

	return true, nil
}

// GetViewDetails returns detailed view information including user names
func (r *AnnouncementViewRepository) GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
	var details []*platform.AnnouncementViewDetail

	// Join with auth.accounts and users.persons to get user names
	// Persons are linked directly to accounts via person.account_id
	err := r.db.NewRaw(`
		SELECT
			v.user_id,
			COALESCE(
				CONCAT(p.first_name, ' ', p.last_name),
				acc.email
			) as user_name,
			v.seen_at,
			v.dismissed
		FROM platform.announcement_views v
		JOIN auth.accounts acc ON acc.id = v.user_id
		LEFT JOIN users.persons p ON p.account_id = acc.id
		WHERE v.announcement_id = ?
		ORDER BY v.seen_at DESC
	`, announcementID).Scan(ctx, &details)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get announcement view details",
			Err: err,
		}
	}

	return details, nil
}
