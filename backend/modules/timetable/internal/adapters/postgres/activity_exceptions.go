package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type activityExceptionRow struct {
	bun.BaseModel   `bun:"table:activity_exceptions,alias:activity_exception"`
	ID              int64     `bun:"id,pk,autoincrement"`
	TenantID        int64     `bun:"tenant_id,notnull"`
	CreatedAt       time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	ActivityGroupID int64     `bun:"activity_group_id,notnull"`
	ExceptionDate   string    `bun:"exception_date,notnull"`
	ExceptionType   string    `bun:"exception_type,notnull"`
	StartTime       *string   `bun:"start_time"`
	EndTime         *string   `bun:"end_time"`
	RoomID          *int64    `bun:"room_id"`
	Reason          *string   `bun:"reason"`
	CreatedBy       *int64    `bun:"created_by"`
}

func (s *Store) FindActivityException(ctx context.Context, id int64) (domain.ActivityException, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityException{}, false, domain.OperationStats{}, err
	}
	row := activityExceptionRow{}
	found, stats, err := scanOne(ctx, activityExceptionSelect(db, &row, tenantID).
		Where(`"activity_exception".id = ?`, id), "find activity exception")
	return activityExceptionToDomain(row), found, stats, err
}

func (s *Store) ListActivityExceptions(ctx context.Context, filter domain.ActivityExceptionFilter) ([]domain.ActivityException, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []activityExceptionRow{}
	query := filterActivityExceptions(activityExceptionSelect(db, &rows, tenantID), filter)
	stats, err := scanAll(ctx, query, "list activity exceptions")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.ActivityException, 0, len(rows))
	for _, row := range rows {
		result = append(result, activityExceptionToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func filterActivityExceptions(query *bun.SelectQuery, filter domain.ActivityExceptionFilter) *bun.SelectQuery {
	if filter.ActivityGroupID != nil {
		query = query.Where(`"activity_exception".activity_group_id = ?`, *filter.ActivityGroupID)
	}
	if filter.ExceptionDate != nil {
		query = query.Where(`"activity_exception".exception_date = ?::date`, *filter.ExceptionDate)
	}
	if filter.FromDate != nil {
		query = query.Where(`"activity_exception".exception_date >= ?::date`, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where(`"activity_exception".exception_date <= ?::date`, *filter.ToDate)
	}
	if filter.BeforeDate != nil {
		query = query.Where(`"activity_exception".exception_date < ?::date`, *filter.BeforeDate)
	}
	if filter.OrderByDate {
		query = query.OrderExpr(`"activity_exception".exception_date ASC, "activity_exception".id ASC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func (s *Store) CountActivityExceptions(ctx context.Context, before *string) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		ColumnExpr("COUNT(*)").Where(`"activity_exception".tenant_id = ?`, tenantID)
	if before != nil {
		query = query.Where(`"activity_exception".exception_date < ?::date`, *before)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var count int
	err = query.Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: count activity exceptions: %w", err)
	}
	stats.Rows = int64(count)
	return count, stats, nil
}

func (s *Store) OldestActivityExceptionBefore(ctx context.Context, before *string) (*string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		ColumnExpr(`MIN("activity_exception".exception_date)::text`).
		Where(`"activity_exception".tenant_id = ?`, tenantID)
	if before != nil {
		query = query.Where(`"activity_exception".exception_date < ?::date`, *before)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var oldest *string
	err = query.Scan(ctx, &oldest)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: find oldest activity exception: %w", err)
	}
	if oldest != nil {
		stats.Rows = 1
	}
	return oldest, stats, nil
}

func (s *Store) CreateActivityException(ctx context.Context, fields domain.ActivityExceptionFields) (domain.ActivityException, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityException{}, domain.OperationStats{}, err
	}
	row := activityExceptionRow{TenantID: tenantID}
	applyActivityExceptionFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.activity_exceptions`).
		Returning(activityExceptionColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ActivityException{}, stats, classifyWriteError("create activity exception", err, &stats)
	}
	stats.Rows = 1
	return activityExceptionToDomain(row), stats, nil
}

func (s *Store) UpdateActivityException(ctx context.Context, id int64, fields domain.ActivityExceptionFields) (domain.ActivityException, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityException{}, false, domain.OperationStats{}, err
	}
	row := activityExceptionRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`schedule.activity_exceptions`).
		Set("activity_group_id = ?", fields.ActivityGroupID).Set("exception_date = ?::date", fields.ExceptionDate).
		Set("exception_type = ?", fields.ExceptionType).Set("start_time = ?", fields.StartTime).
		Set("end_time = ?", fields.EndTime).Set("room_id = ?", fields.RoomID).
		Set("reason = ?", fields.Reason).Set("created_by = ?", fields.CreatedBy).Set("updated_at = NOW()").
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning(activityExceptionColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ActivityException{}, false, stats, nil
	}
	if err != nil {
		return domain.ActivityException{}, false, stats, classifyWriteError("update activity exception", err, &stats)
	}
	stats.Rows = 1
	return activityExceptionToDomain(row), true, stats, nil
}

func (s *Store) DeleteActivityException(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.activity_exceptions").
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete activity exception")
}

func (s *Store) DeleteActivityExceptionsBefore(ctx context.Context, before string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("schedule.activity_exceptions").
		Where("tenant_id = ?", tenantID).Where("exception_date < ?::date", before), "delete activity exceptions before")
	return stats.Rows, stats, err
}

const activityExceptionColumns = `id, tenant_id, created_at, updated_at, activity_group_id,
	exception_date::text AS exception_date, exception_type, start_time::text AS start_time,
	end_time::text AS end_time, room_id, reason, created_by`

func activityExceptionSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		ColumnExpr(`"activity_exception".id, "activity_exception".tenant_id, "activity_exception".created_at, "activity_exception".updated_at`).
		ColumnExpr(`"activity_exception".activity_group_id, "activity_exception".exception_date::text AS exception_date`).
		ColumnExpr(`"activity_exception".exception_type, "activity_exception".start_time::text AS start_time, "activity_exception".end_time::text AS end_time`).
		ColumnExpr(`"activity_exception".room_id, "activity_exception".reason, "activity_exception".created_by`).
		Where(`"activity_exception".tenant_id = ?`, tenantID)
}

func applyActivityExceptionFields(row *activityExceptionRow, fields domain.ActivityExceptionFields) {
	row.ActivityGroupID, row.ExceptionDate, row.ExceptionType = fields.ActivityGroupID, fields.ExceptionDate, fields.ExceptionType
	row.StartTime, row.EndTime, row.RoomID = fields.StartTime, fields.EndTime, fields.RoomID
	row.Reason, row.CreatedBy = fields.Reason, fields.CreatedBy
}

func activityExceptionToDomain(row activityExceptionRow) domain.ActivityException {
	return domain.ActivityException{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ActivityGroupID: row.ActivityGroupID, ExceptionDate: row.ExceptionDate, ExceptionType: row.ExceptionType,
		StartTime: row.StartTime, EndTime: row.EndTime, RoomID: row.RoomID, Reason: row.Reason, CreatedBy: row.CreatedBy,
	}
}
