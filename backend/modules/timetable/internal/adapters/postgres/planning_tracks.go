package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type planningTrackRow struct {
	bun.BaseModel `bun:"table:planning_tracks,alias:planning_track"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string     `bun:"name,notnull"`
	Color         string     `bun:"color,notnull"`
	SortOrder     int        `bun:"sort_order,notnull"`
	ArchivedAt    *time.Time `bun:"archived_at"`
}

func (s *Store) FindPlanningTrack(ctx context.Context, id int64, lock string) (domain.PlanningTrack, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlanningTrack{}, false, domain.OperationStats{}, err
	}
	row := planningTrackRow{}
	query := planningTrackSelect(db, &row, tenantID).Where(`"planning_track".id = ?`, id)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find planning track")
	return planningTrackToDomain(row), found, stats, err
}

func (s *Store) ListPlanningTracks(ctx context.Context, filter domain.PlanningTrackFilter) ([]domain.PlanningTrack, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []planningTrackRow{}
	query := planningTrackSelect(db, &rows, tenantID)
	if len(filter.IDs) > 0 {
		query = query.Where(`"planning_track".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.Ordered {
		query = query.OrderExpr(`"planning_track".archived_at ASC NULLS FIRST`).
			OrderExpr(`"planning_track".sort_order ASC`).OrderExpr(`LOWER("planning_track".name) ASC`)
	}
	stats, err := scanAll(ctx, query, "list planning tracks")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.PlanningTrack, 0, len(rows))
	for _, row := range rows {
		result = append(result, planningTrackToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreatePlanningTrack(ctx context.Context, fields domain.PlanningTrackFields) (domain.PlanningTrack, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlanningTrack{}, domain.OperationStats{}, err
	}
	row := planningTrackRow{TenantID: tenantID}
	applyPlanningTrackFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.planning_tracks`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.PlanningTrack{}, stats, classifyWriteError("create planning track", err, &stats)
	}
	stats.Rows = 1
	return planningTrackToDomain(row), stats, nil
}

func (s *Store) UpdatePlanningTrack(ctx context.Context, id int64, fields domain.PlanningTrackFields, activeOnly bool) (domain.PlanningTrack, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlanningTrack{}, false, domain.OperationStats{}, err
	}
	row := planningTrackRow{}
	query := db.NewUpdate().Model(&row).ModelTableExpr(`schedule.planning_tracks`).
		Set("name = ?", fields.Name).Set("color = ?", fields.Color).Set("sort_order = ?", fields.SortOrder).
		Set("archived_at = ?", fields.ArchivedAt).Set("updated_at = NOW()").
		Where("id = ?", id).Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("archived_at IS NULL")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PlanningTrack{}, false, stats, nil
	}
	if err != nil {
		return domain.PlanningTrack{}, false, stats, classifyWriteError("update planning track", err, &stats)
	}
	stats.Rows = 1
	return planningTrackToDomain(row), true, stats, nil
}

func (s *Store) DeletePlanningTrack(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.planning_tracks").
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete planning track")
}

func (s *Store) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (domain.PlanningTrack, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlanningTrack{}, false, domain.OperationStats{}, err
	}
	row := planningTrackRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`schedule.planning_tracks`).Set("archived_at = ?", value).
		Where("tenant_id = ?", tenantID).Where("id = ?", id).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PlanningTrack{}, false, stats, nil
	}
	if err != nil {
		return domain.PlanningTrack{}, false, stats, classifyWriteError("archive planning track", err, &stats)
	}
	stats.Rows = 1
	return planningTrackToDomain(row), true, stats, nil
}

func (s *Store) ReorderPlanningTracks(ctx context.Context, ids []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil || len(ids) == 0 {
		return domain.OperationStats{}, err
	}
	activeRows := []struct {
		ID int64 `bun:"id"`
	}{}
	stats, err := scanAll(ctx, db.NewSelect().Model(&activeRows).ModelTableExpr(`schedule.planning_tracks AS "planning_track"`).
		ColumnExpr(`"planning_track".id`).Where(`"planning_track".tenant_id = ?`, tenantID).
		Where(`"planning_track".archived_at IS NULL`).For("UPDATE"), "validate planning track order")
	if err != nil {
		return stats, err
	}
	activeIDs := make([]int64, 0, len(activeRows))
	for _, row := range activeRows {
		activeIDs = append(activeIDs, row.ID)
	}
	return s.applyPlanningTrackOrder(ctx, db, tenantID, ids, activeIDs, stats)
}

func (s *Store) applyPlanningTrackOrder(ctx context.Context, db bun.IDB, tenantID int64, ids, activeIDs []int64, stats domain.OperationStats) (domain.OperationStats, error) {
	if !sameIDSet(ids, activeIDs) {
		return stats, domain.ErrPlanningTrackNotFound
	}
	writeStats, err := execMeasuredWrite(ctx, db.NewRaw(`
		UPDATE schedule.planning_tracks AS "planning_track"
		SET sort_order = ("ordered".position - 1)::integer, updated_at = NOW()
		FROM unnest(?::bigint[]) WITH ORDINALITY AS "ordered"(id, position)
		WHERE "planning_track".tenant_id = ?
			AND "planning_track".id = "ordered".id
			AND "planning_track".archived_at IS NULL`, pgdialect.Array(ids), tenantID), "reorder planning tracks")
	stats.Add(writeStats)
	return stats, err
}

func (s *Store) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (domain.PlanningTrack, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlanningTrack{}, false, domain.OperationStats{}, err
	}
	row := planningTrackRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		UPDATE schedule.planning_tracks AS "planning_track"
		SET archived_at = NULL,
			sort_order = COALESCE((SELECT MAX("active".sort_order) + 1 FROM schedule.planning_tracks AS "active"
				WHERE "active".tenant_id = ? AND "active".archived_at IS NULL), 0),
			updated_at = NOW()
		WHERE "planning_track".tenant_id = ? AND "planning_track".id = ? AND "planning_track".archived_at IS NOT NULL
		RETURNING "planning_track".*`, tenantID, tenantID, id).Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PlanningTrack{}, false, stats, nil
	}
	if err != nil {
		return domain.PlanningTrack{}, false, stats, classifyWriteError("restore planning track", err, &stats)
	}
	stats.Rows = 1
	return planningTrackToDomain(row), true, stats, nil
}

func planningTrackSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.planning_tracks AS "planning_track"`).
		ColumnExpr(`"planning_track".*`).Where(`"planning_track".tenant_id = ?`, tenantID)
}

func applyPlanningTrackFields(row *planningTrackRow, fields domain.PlanningTrackFields) {
	row.Name, row.Color, row.SortOrder, row.ArchivedAt = fields.Name, fields.Color, fields.SortOrder, fields.ArchivedAt
}

func planningTrackToDomain(row planningTrackRow) domain.PlanningTrack {
	return domain.PlanningTrack{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, Color: row.Color, SortOrder: row.SortOrder, ArchivedAt: row.ArchivedAt}
}

func sameIDSet(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[int64]struct{}, len(left))
	for _, id := range left {
		values[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := values[id]; !ok {
			return false
		}
	}
	return true
}
