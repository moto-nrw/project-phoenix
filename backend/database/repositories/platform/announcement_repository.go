package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// Table constant
const tablePlatformAnnouncements = "platform.announcements"

// AnnouncementRepository implements platform.AnnouncementRepository interface
type AnnouncementRepository struct {
	*base.Repository[*platform.Announcement]
	db *bun.DB
}

// NewAnnouncementRepository creates a new AnnouncementRepository
func NewAnnouncementRepository(db *bun.DB) platform.AnnouncementRepository {
	return &AnnouncementRepository{
		Repository: base.NewRepository[*platform.Announcement](db, tablePlatformAnnouncements, "Announcement"),
		db:         db,
	}
}

// Create inserts a new announcement
func (r *AnnouncementRepository) Create(ctx context.Context, announcement *platform.Announcement) error {
	if announcement == nil {
		return fmt.Errorf("announcement cannot be nil")
	}

	if err := announcement.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, announcement)
}

// FindByID retrieves an announcement by ID
func (r *AnnouncementRepository) FindByID(ctx context.Context, id int64) (*platform.Announcement, error) {
	announcement := new(platform.Announcement)
	err := r.db.NewSelect().
		TableExpr(tablePlatformAnnouncements).
		ColumnExpr("*").
		Where("id = ?", id).
		Scan(ctx, announcement)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find announcement by id",
			Err: err,
		}
	}

	return announcement, nil
}

// Update updates an announcement
func (r *AnnouncementRepository) Update(ctx context.Context, announcement *platform.Announcement) error {
	if announcement == nil {
		return fmt.Errorf("announcement cannot be nil")
	}

	if err := announcement.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, announcement)
}

// Delete removes an announcement by ID
func (r *AnnouncementRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// List retrieves announcements, optionally including inactive ones
func (r *AnnouncementRepository) List(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	var announcements []*platform.Announcement
	query := r.db.NewSelect().
		TableExpr(tablePlatformAnnouncements).
		ColumnExpr("*")

	if !includeInactive {
		query = query.Where("active = true")
	}

	err := query.
		Order("created_at DESC").
		Scan(ctx, &announcements)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list announcements",
			Err: err,
		}
	}

	return announcements, nil
}

// ListPublished retrieves only published and non-expired active announcements
func (r *AnnouncementRepository) ListPublished(ctx context.Context) ([]*platform.Announcement, error) {
	var announcements []*platform.Announcement
	now := time.Now()

	err := r.db.NewSelect().
		TableExpr(tablePlatformAnnouncements).
		ColumnExpr("*").
		Where("active = true").
		Where("published_at IS NOT NULL").
		Where("published_at <= ?", now).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				Where("expires_at IS NULL").
				WhereOr("expires_at > ?", now)
		}).
		Order("published_at DESC").
		Scan(ctx, &announcements)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list published announcements",
			Err: err,
		}
	}

	return announcements, nil
}

// Publish sets the published_at timestamp to now
func (r *AnnouncementRepository) Publish(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*platform.Announcement)(nil)).
		ModelTableExpr(tablePlatformAnnouncements).
		Set("published_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "publish announcement",
			Err: err,
		}
	}

	return nil
}

// Unpublish clears the published_at timestamp
func (r *AnnouncementRepository) Unpublish(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*platform.Announcement)(nil)).
		ModelTableExpr(tablePlatformAnnouncements).
		Set("published_at = NULL").
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "unpublish announcement",
			Err: err,
		}
	}

	return nil
}
