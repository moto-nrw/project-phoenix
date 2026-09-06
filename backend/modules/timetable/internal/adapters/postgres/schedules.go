package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type scheduleRow struct {
	bun.BaseModel    `bun:"table:schedules,alias:schedule"`
	ID               int64     `bun:"id,pk,autoincrement"`
	TenantID         int64     `bun:"tenant_id,notnull"`
	CreatedAt        time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Weekday          int       `bun:"weekday,notnull"`
	TimeframeID      *int64    `bun:"timeframe_id"`
	ActivityGroupID  int64     `bun:"activity_group_id,notnull"`
	WeekPattern      int       `bun:"week_pattern,notnull,default:0"`
	CalendarPeriodID *int64    `bun:"calendar_period_id"`
	ValidUntil       *string   `bun:"valid_until"`
	ValidFrom        *string   `bun:"valid_from"`
}

func (s *Store) FindSchedule(ctx context.Context, id int64) (domain.Schedule, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Schedule{}, false, domain.OperationStats{}, err
	}
	row := scheduleRow{}
	query := scheduleSelect(db, &row, tenantID).Where(`"schedule".id = ?`, id)
	found, stats, err := scanOne(ctx, query, "find schedule")
	return scheduleToDomain(row), found, stats, err
}

func (s *Store) ListSchedules(ctx context.Context, filter domain.ScheduleFilter) ([]domain.Schedule, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []scheduleRow{}
	query := scheduleSelect(db, &rows, tenantID)
	if len(filter.GroupIDs) > 0 {
		query = query.Where(`"schedule".activity_group_id IN (?)`, bun.List(filter.GroupIDs))
	}
	if filter.Weekday != nil {
		query = query.Where(`"schedule".weekday = ?`, *filter.Weekday)
	}
	query = query.OrderExpr(`"schedule".weekday ASC, "schedule".timeframe_id ASC`)
	stats, err := scanAll(ctx, query, "list schedules")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Schedule, 0, len(rows))
	for _, row := range rows {
		result = append(result, scheduleToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) FindTemplateStartTimes(ctx context.Context, groupIDs []int64) ([]domain.TemplateStartTime, domain.OperationStats, error) {
	if len(groupIDs) == 0 {
		return []domain.TemplateStartTime{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []struct {
		ActivityGroupID int64     `bun:"activity_group_id"`
		Weekday         int       `bun:"weekday"`
		StartTime       time.Time `bun:"start_time"`
	}{}
	query := db.NewSelect().Model(&rows).TableExpr(`activities.schedules AS "schedule"`).
		ColumnExpr(`"schedule".activity_group_id, "schedule".weekday, "timeframe".start_time`).
		Join(`INNER JOIN schedule.timeframes AS "timeframe" ON "timeframe".id = "schedule".timeframe_id`).
		Where(`"schedule".tenant_id = ?`, tenantID).Where(`"timeframe".tenant_id = ?`, tenantID).
		Where(`"schedule".activity_group_id IN (?)`, bun.List(groupIDs)).
		OrderExpr(`"schedule".activity_group_id ASC, "schedule".weekday ASC, "timeframe".start_time ASC`)
	stats, err := scanAll(ctx, query, "find template start times")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.TemplateStartTime, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.TemplateStartTime(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateSchedule(ctx context.Context, fields domain.ScheduleFields) (domain.Schedule, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Schedule{}, domain.OperationStats{}, err
	}
	row := scheduleRow{TenantID: tenantID}
	applyScheduleFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`activities.schedules`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Schedule{}, stats, classifyWriteError("create schedule", err, &stats)
	}
	stats.Rows = 1
	return scheduleToDomain(row), stats, nil
}

func (s *Store) UpdateSchedule(ctx context.Context, id int64, fields domain.ScheduleFields) (domain.Schedule, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Schedule{}, false, domain.OperationStats{}, err
	}
	row := scheduleRow{ID: id, TenantID: tenantID}
	applyScheduleFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`activities.schedules`).
		Column("weekday", "timeframe_id", "activity_group_id", "week_pattern", "calendar_period_id", "valid_until", "valid_from").
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Schedule{}, false, stats, nil
	}
	if err != nil {
		return domain.Schedule{}, false, stats, classifyWriteError("update schedule", err, &stats)
	}
	stats.Rows = 1
	return scheduleToDomain(row), true, stats, nil
}

func (s *Store) DeleteSchedule(ctx context.Context, id int64) (domain.OperationStats, error) {
	return s.deleteSchedules(ctx, "delete schedule", func(query *bun.DeleteQuery) *bun.DeleteQuery {
		return query.Where("id = ?", id)
	})
}

func (s *Store) DeleteSchedulesByGroup(ctx context.Context, groupID int64) (domain.OperationStats, error) {
	return s.deleteSchedules(ctx, "delete schedules by group", func(query *bun.DeleteQuery) *bun.DeleteQuery {
		return query.Where("activity_group_id = ?", groupID)
	})
}

func (s *Store) deleteSchedules(ctx context.Context, operation string, apply func(*bun.DeleteQuery) *bun.DeleteQuery) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := apply(db.NewDelete().Table("activities.schedules").Where("tenant_id = ?", tenantID)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError(operation, err, &stats)
	}
	_, err = rowsAffected(result, operation, &stats)
	return stats, err
}

func (s *Store) CapScheduleValidUntil(ctx context.Context, groupID int64, validUntil string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Table("activities.schedules").Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", tenantID).Where("activity_group_id = ?", groupID).
		Where("(valid_until IS NULL OR valid_until > ?)", validUntil).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, classifyWriteError("cap schedule valid_until", err, &stats)
	}
	rows, err := rowsAffected(result, "capped schedules", &stats)
	return rows, stats, err
}

func scheduleSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`activities.schedules AS "schedule"`).
		ColumnExpr(`"schedule".*`).Where(`"schedule".tenant_id = ?`, tenantID)
}

func applyScheduleFields(row *scheduleRow, fields domain.ScheduleFields) {
	row.Weekday, row.TimeframeID = fields.Weekday, fields.TimeframeID
	row.ActivityGroupID, row.WeekPattern = fields.ActivityGroupID, fields.WeekPattern
	row.CalendarPeriodID = fields.CalendarPeriodID
	row.ValidUntil, row.ValidFrom = fields.ValidUntil, fields.ValidFrom
}

func scheduleToDomain(row scheduleRow) domain.Schedule {
	return domain.Schedule{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Weekday: row.Weekday, TimeframeID: row.TimeframeID, ActivityGroupID: row.ActivityGroupID,
		WeekPattern: row.WeekPattern, CalendarPeriodID: row.CalendarPeriodID,
		ValidUntil: row.ValidUntil, ValidFrom: row.ValidFrom,
	}
}
