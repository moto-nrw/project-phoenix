package platform

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// SQL constants
const (
	tablePlatformAnnouncements  = "platform.announcements"
	databaseCurrentTimestampSQL = "CURRENT_TIMESTAMP"
)

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

// FindByID retrieves an announcement by ID
func (r *AnnouncementRepository) FindByID(ctx context.Context, id int64) (*platform.Announcement, error) {
	announcement := new(platform.Announcement)
	err := base.GetDB(ctx, r.db).NewSelect().
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
			Err: base.TranslateNotFound(err),
		}
	}

	return announcement, nil
}

// Delete removes an announcement by ID
func (r *AnnouncementRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// List retrieves announcements, optionally including inactive ones
func (r *AnnouncementRepository) List(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	var announcements []*platform.Announcement
	query := base.GetDB(ctx, r.db).NewSelect().
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
			Err: base.TranslateNotFound(err),
		}
	}

	return announcements, nil
}

// Publish sets the published_at timestamp to now
func (r *AnnouncementRepository) Publish(ctx context.Context, id int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.Announcement)(nil)).
		ModelTableExpr(tablePlatformAnnouncements).
		Set("published_at = "+databaseCurrentTimestampSQL).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "publish announcement",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// Unpublish clears the published_at timestamp
func (r *AnnouncementRepository) Unpublish(ctx context.Context, id int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.Announcement)(nil)).
		ModelTableExpr(tablePlatformAnnouncements).
		Set("published_at = NULL").
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "unpublish announcement",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}
