package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type timeframeRow struct {
	bun.BaseModel `bun:"table:timeframes,alias:timeframe"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StartTime     string    `bun:"start_time,notnull"`
	EndTime       *string   `bun:"end_time"`
	IsActive      bool      `bun:"is_active,notnull,default:false"`
	Description   string    `bun:"description"`
}

func (s *Store) FindTimeframe(ctx context.Context, id int64) (domain.Timeframe, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Timeframe{}, false, domain.OperationStats{}, err
	}
	row := timeframeRow{}
	found, stats, err := scanOne(ctx, timeframeSelect(db, &row, tenantID).Where(`"timeframe".id = ?`, id), "find timeframe")
	return timeframeToDomain(row), found, stats, err
}

func (s *Store) ListTimeframes(ctx context.Context, filter domain.TimeframeFilter) ([]domain.Timeframe, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []timeframeRow{}
	query := filterTimeframes(timeframeSelect(db, &rows, tenantID), filter)
	stats, err := scanAll(ctx, query, "list timeframes")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Timeframe, 0, len(rows))
	for _, row := range rows {
		result = append(result, timeframeToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func filterTimeframes(query *bun.SelectQuery, filter domain.TimeframeFilter) *bun.SelectQuery {
	if filter.DescriptionContains != "" {
		query = query.Where(`"timeframe".description ILIKE ?`, "%"+filter.DescriptionContains+"%")
	}
	if filter.ActiveOnly {
		query = query.Where(`"timeframe".is_active = TRUE`)
	}
	if filter.OverlapsEnd != nil {
		query = query.Where(`"timeframe".start_time <= CAST(? AS TIME)`, *filter.OverlapsEnd)
	}
	if filter.OverlapsStart != nil {
		query = query.Where(`("timeframe".end_time IS NULL OR "timeframe".end_time >= CAST(? AS TIME))`, *filter.OverlapsStart)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func (s *Store) CreateTimeframe(ctx context.Context, fields domain.TimeframeFields) (domain.Timeframe, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Timeframe{}, domain.OperationStats{}, err
	}
	row := timeframeRow{TenantID: tenantID}
	applyTimeframeFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.timeframes`).
		Returning(`id, tenant_id, created_at, updated_at, start_time::text AS start_time, end_time::text AS end_time, is_active, description`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Timeframe{}, stats, classifyWriteError("create timeframe", err, &stats)
	}
	stats.Rows = 1
	return timeframeToDomain(row), stats, nil
}

func (s *Store) UpdateTimeframe(ctx context.Context, id int64, fields domain.TimeframeFields) (domain.Timeframe, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Timeframe{}, false, domain.OperationStats{}, err
	}
	row := timeframeRow{ID: id, TenantID: tenantID}
	applyTimeframeFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`schedule.timeframes`).
		Column("start_time", "end_time", "is_active", "description").
		Where("id = ?", id).Where("tenant_id = ?", tenantID).
		Returning(`id, tenant_id, created_at, updated_at, start_time::text AS start_time, end_time::text AS end_time, is_active, description`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Timeframe{}, false, stats, nil
	}
	if err != nil {
		return domain.Timeframe{}, false, stats, classifyWriteError("update timeframe", err, &stats)
	}
	stats.Rows = 1
	return timeframeToDomain(row), true, stats, nil
}

func (s *Store) DeleteTimeframe(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.timeframes").
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete timeframe")
}

func timeframeSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		ColumnExpr(`"timeframe".id, "timeframe".tenant_id, "timeframe".created_at, "timeframe".updated_at`).
		ColumnExpr(`"timeframe".start_time::text AS start_time, "timeframe".end_time::text AS end_time`).
		ColumnExpr(`"timeframe".is_active, "timeframe".description`).
		Where(`"timeframe".tenant_id = ?`, tenantID)
}

func applyTimeframeFields(row *timeframeRow, fields domain.TimeframeFields) {
	row.StartTime, row.EndTime = fields.StartTime, fields.EndTime
	row.IsActive, row.Description = fields.IsActive, fields.Description
}

func timeframeToDomain(row timeframeRow) domain.Timeframe {
	return domain.Timeframe{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartTime: row.StartTime, EndTime: row.EndTime, IsActive: row.IsActive, Description: row.Description}
}
