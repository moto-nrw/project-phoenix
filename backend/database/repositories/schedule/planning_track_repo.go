package schedule

import (
	"context"
	"database/sql"
	"errors"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const planningTracksTable = "schedule.planning_tracks"

type PlanningTrackRepository struct {
	*repoBase.Repository[*model.PlanningTrack]
	db *bun.DB
}

func NewPlanningTrackRepository(db *bun.DB) model.PlanningTrackRepository {
	repo := repoBase.NewRepository[*model.PlanningTrack](db, planningTracksTable, "PlanningTrack")
	repo.TenantScoped = true
	return &PlanningTrackRepository{Repository: repo, db: db}
}

func (r *PlanningTrackRepository) ListAll(ctx context.Context) ([]*model.PlanningTrack, error) {
	tracks := make([]*model.PlanningTrack, 0)
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&tracks).
		ModelTableExpr(`schedule.planning_tracks AS "planning_track"`).
		OrderExpr(`"planning_track".archived_at ASC NULLS FIRST`).
		OrderExpr(`"planning_track".sort_order ASC`).
		OrderExpr(`LOWER("planning_track".name) ASC`)
	query = repoBase.WithTenantFilter(ctx, query, "planning_track")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list planning tracks", Err: err}
	}
	return tracks, nil
}

func (r *PlanningTrackRepository) FindByIDForShare(ctx context.Context, id int64) (*model.PlanningTrack, error) {
	track := new(model.PlanningTrack)
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(track).
		ModelTableExpr(`schedule.planning_tracks AS "planning_track"`).
		Where(`"planning_track".id = ?`, id).
		For("SHARE")
	query = repoBase.WithTenantFilter(ctx, query, "planning_track")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find planning track for share", Err: err}
	}
	return track, nil
}

func (r *PlanningTrackRepository) UpdateIfActive(ctx context.Context, track *model.PlanningTrack) (bool, error) {
	if track == nil {
		return false, errors.New("planning track cannot be nil")
	}
	if err := track.Validate(); err != nil {
		return false, err
	}
	track.UpdatedAt = time.Now()
	query := repoBase.GetDB(ctx, r.db).NewUpdate().
		Model(track).
		ModelTableExpr(planningTracksTable).
		Column("name", "color", "sort_order", "updated_at").
		Where("id = ?", track.ID).
		Where("archived_at IS NULL")
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "update active planning track", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "update active planning track", Err: err}
	}
	return rows == 1, nil
}

func (r *PlanningTrackRepository) UpdateSortOrders(ctx context.Context, ids []int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("no tenant in context")
	}
	if len(ids) == 0 {
		return nil
	}
	db := repoBase.GetDB(ctx, r.db)
	activeIDs := make([]int64, 0, len(ids))
	if err := db.NewSelect().Table(planningTracksTable).
		Column("id").
		Where("tenant_id = ?", tenantID).
		Where("archived_at IS NULL").
		OrderExpr("id ASC").
		For("UPDATE").
		Scan(ctx, &activeIDs); err != nil {
		return &modelBase.DatabaseError{Op: "validate planning track order", Err: err}
	}
	if len(activeIDs) != len(ids) {
		return &modelBase.DatabaseError{Op: "validate planning track order", Err: sql.ErrNoRows}
	}
	active := make(map[int64]struct{}, len(activeIDs))
	for _, id := range activeIDs {
		active[id] = struct{}{}
	}
	for _, id := range ids {
		if _, exists := active[id]; !exists {
			return &modelBase.DatabaseError{Op: "validate planning track order", Err: sql.ErrNoRows}
		}
	}
	for sortOrder, id := range ids {
		result, err := db.NewUpdate().Table(planningTracksTable).
			Set("sort_order = ?", sortOrder).
			Set("updated_at = ?", time.Now()).
			Where("tenant_id = ?", tenantID).
			Where("id = ?", id).
			Where("archived_at IS NULL").
			Exec(ctx)
		if err != nil {
			return &modelBase.DatabaseError{Op: "reorder planning tracks", Err: err}
		}
		if err := repoBase.AssertRowsAffected(result, 1, "reorder planning track"); err != nil {
			return err
		}
	}
	return nil
}
