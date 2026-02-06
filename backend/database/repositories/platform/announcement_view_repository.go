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

// GetUnreadForUser retrieves all unread active announcements for a user
func (r *AnnouncementViewRepository) GetUnreadForUser(ctx context.Context, userID int64) ([]*platform.Announcement, error) {
	var announcements []*platform.Announcement
	now := time.Now()

	// Use raw SQL for complex query with aliases to avoid BUN's quote escaping issues
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
		ORDER BY a.published_at DESC
	`, userID, now, now).Scan(ctx, &announcements)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get unread announcements for user",
			Err: err,
		}
	}

	return announcements, nil
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
