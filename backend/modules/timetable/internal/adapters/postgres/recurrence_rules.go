package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type recurrenceRuleRow struct {
	bun.BaseModel `bun:"table:recurrence_rules,alias:recurrence_rule"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Frequency     string     `bun:"frequency,notnull"`
	IntervalCount int        `bun:"interval_count,notnull,default:1"`
	Weekdays      []string   `bun:"weekdays,array"`
	MonthDays     []int      `bun:"month_days,array"`
	EndDate       *time.Time `bun:"end_date"`
	Count         *int       `bun:"count"`
}

func (s *Store) FindRecurrenceRule(ctx context.Context, id int64) (domain.RecurrenceRule, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.RecurrenceRule{}, false, domain.OperationStats{}, err
	}
	row := recurrenceRuleRow{}
	found, stats, err := scanOne(ctx, recurrenceRuleSelect(db, &row, tenantID).
		Where(`"recurrence_rule".id = ?`, id), "find recurrence rule")
	return recurrenceRuleToDomain(row), found, stats, err
}

func (s *Store) ListRecurrenceRules(ctx context.Context, filter domain.RecurrenceRuleFilter) ([]domain.RecurrenceRule, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []recurrenceRuleRow{}
	query := filterRecurrenceRules(recurrenceRuleSelect(db, &rows, tenantID), filter)
	stats, err := scanAll(ctx, query, "list recurrence rules")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.RecurrenceRule, 0, len(rows))
	for _, row := range rows {
		result = append(result, recurrenceRuleToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func filterRecurrenceRules(query *bun.SelectQuery, filter domain.RecurrenceRuleFilter) *bun.SelectQuery {
	if filter.Frequency != "" {
		query = query.Where(`LOWER("recurrence_rule".frequency) = LOWER(?)`, filter.Frequency)
	}
	if len(filter.Frequencies) > 0 {
		query = query.Where(`LOWER("recurrence_rule".frequency) IN (?)`, bun.List(lowerStrings(filter.Frequencies)))
	}
	if filter.Weekday != "" {
		query = query.Where(`? = ANY("recurrence_rule".weekdays)`, filter.Weekday)
	}
	if filter.ActiveAt != nil {
		query = query.Where(`("recurrence_rule".end_date IS NULL OR "recurrence_rule".end_date >= ?)`, *filter.ActiveAt)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return orderRecurrenceRules(query, filter.SortBy, filter.SortDescending)
}

func orderRecurrenceRules(query *bun.SelectQuery, field string, descending bool) *bun.SelectQuery {
	if descending {
		return orderRecurrenceRulesDescending(query, field)
	}
	switch field {
	case "id":
		return query.OrderExpr(`"recurrence_rule".id ASC`)
	case "frequency":
		return query.OrderExpr(`"recurrence_rule".frequency ASC`)
	case "interval_count":
		return query.OrderExpr(`"recurrence_rule".interval_count ASC`)
	case "end_date":
		return query.OrderExpr(`"recurrence_rule".end_date ASC`)
	case "count":
		return query.OrderExpr(`"recurrence_rule".count ASC`)
	case "created_at":
		return query.OrderExpr(`"recurrence_rule".created_at ASC`)
	case "updated_at":
		return query.OrderExpr(`"recurrence_rule".updated_at ASC`)
	default:
		return query
	}
}

func orderRecurrenceRulesDescending(query *bun.SelectQuery, field string) *bun.SelectQuery {
	switch field {
	case "id":
		return query.OrderExpr(`"recurrence_rule".id DESC`)
	case "frequency":
		return query.OrderExpr(`"recurrence_rule".frequency DESC`)
	case "interval_count":
		return query.OrderExpr(`"recurrence_rule".interval_count DESC`)
	case "end_date":
		return query.OrderExpr(`"recurrence_rule".end_date DESC`)
	case "count":
		return query.OrderExpr(`"recurrence_rule".count DESC`)
	case "created_at":
		return query.OrderExpr(`"recurrence_rule".created_at DESC`)
	case "updated_at":
		return query.OrderExpr(`"recurrence_rule".updated_at DESC`)
	default:
		return query
	}
}

func lowerStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.ToLower(value))
	}
	return result
}

func (s *Store) CreateRecurrenceRule(ctx context.Context, fields domain.RecurrenceRuleFields) (domain.RecurrenceRule, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.RecurrenceRule{}, domain.OperationStats{}, err
	}
	row := recurrenceRuleRow{TenantID: tenantID}
	applyRecurrenceRuleFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.recurrence_rules`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.RecurrenceRule{}, stats, classifyWriteError("create recurrence rule", err, &stats)
	}
	stats.Rows = 1
	return recurrenceRuleToDomain(row), stats, nil
}

func (s *Store) UpdateRecurrenceRule(ctx context.Context, id int64, fields domain.RecurrenceRuleFields) (domain.RecurrenceRule, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.RecurrenceRule{}, false, domain.OperationStats{}, err
	}
	row := recurrenceRuleRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`schedule.recurrence_rules`).
		Set("frequency = ?", fields.Frequency).Set("interval_count = ?", fields.IntervalCount).
		Set("weekdays = ?", pgdialect.Array(fields.Weekdays)).Set("month_days = ?", pgdialect.Array(fields.MonthDays)).
		Set("end_date = ?", fields.EndDate).Set("count = ?", fields.Count).
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RecurrenceRule{}, false, stats, nil
	}
	if err != nil {
		return domain.RecurrenceRule{}, false, stats, classifyWriteError("update recurrence rule", err, &stats)
	}
	stats.Rows = 1
	return recurrenceRuleToDomain(row), true, stats, nil
}

func (s *Store) DeleteRecurrenceRule(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.recurrence_rules").
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete recurrence rule")
}

func recurrenceRuleSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.recurrence_rules AS "recurrence_rule"`).
		ColumnExpr(`"recurrence_rule".*`).Where(`"recurrence_rule".tenant_id = ?`, tenantID)
}

func applyRecurrenceRuleFields(row *recurrenceRuleRow, fields domain.RecurrenceRuleFields) {
	row.Frequency, row.IntervalCount = fields.Frequency, fields.IntervalCount
	row.Weekdays, row.MonthDays = fields.Weekdays, fields.MonthDays
	row.EndDate, row.Count = fields.EndDate, fields.Count
}

func recurrenceRuleToDomain(row recurrenceRuleRow) domain.RecurrenceRule {
	return domain.RecurrenceRule{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Frequency: row.Frequency, IntervalCount: row.IntervalCount, Weekdays: row.Weekdays, MonthDays: row.MonthDays,
		EndDate: row.EndDate, Count: row.Count}
}
