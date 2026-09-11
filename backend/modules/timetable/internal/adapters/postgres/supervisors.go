package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type plannedSupervisorRow struct {
	bun.BaseModel    `bun:"table:supervisors,alias:supervisor"`
	ID               int64     `bun:"id,pk,autoincrement"`
	TenantID         int64     `bun:"tenant_id,notnull"`
	CreatedAt        time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StaffID          int64     `bun:"staff_id,notnull"`
	GroupID          int64     `bun:"group_id,notnull"`
	IsPrimary        bool      `bun:"is_primary,notnull,default:false"`
	ValidFrom        string    `bun:"valid_from,notnull"`
	ValidUntil       *string   `bun:"valid_until"`
	CalendarPeriodID *int64    `bun:"calendar_period_id"`
	Weekday          *int      `bun:"weekday"`
}

func (s *Store) FindPlannedSupervisor(ctx context.Context, id int64) (domain.PlannedSupervisor, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlannedSupervisor{}, false, domain.OperationStats{}, err
	}
	row := plannedSupervisorRow{}
	found, stats, err := scanOne(ctx, plannedSupervisorSelect(db, &row, tenantID).Where(`"supervisor".id = ?`, id), "find planned supervisor")
	return plannedSupervisorToDomain(row), found, stats, err
}

func (s *Store) ListPlannedSupervisors(ctx context.Context, filter domain.PlannedSupervisorFilter) ([]domain.PlannedSupervisor, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []plannedSupervisorRow{}
	query := plannedSupervisorSelect(db, &rows, tenantID)
	if len(filter.GroupIDs) > 0 {
		query = query.Where(`"supervisor".group_id IN (?)`, bun.List(filter.GroupIDs))
	}
	if filter.StaffID != nil {
		query = query.Where(`"supervisor".staff_id = ?`, *filter.StaffID)
	}
	query = query.OrderExpr(`"supervisor".is_primary DESC, "supervisor".id ASC`)
	stats, err := scanAll(ctx, query, "list planned supervisors")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.PlannedSupervisor, 0, len(rows))
	for _, row := range rows {
		result = append(result, plannedSupervisorToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) ListPlannedSupervisionBlockers(ctx context.Context, staffID int64) ([]domain.PlannedSupervisionBlocker, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.PlannedSupervisionBlocker{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`activities.supervisors AS "supervisor"`).
		ColumnExpr(`"supervisor".id`).ColumnExpr(`"supervisor".group_id AS activity_id`).
		ColumnExpr(`COALESCE("group".name, 'Unbekannte Aktivität') AS activity_name`).
		ColumnExpr(`COALESCE("supervisor".is_primary, false) AS is_primary`).
		Join(`LEFT JOIN activities.groups AS "group" ON "group".id = "supervisor".group_id AND "group".tenant_id = "supervisor".tenant_id`).
		Where(`"supervisor".tenant_id = ?`, tenantID).Where(`"supervisor".staff_id = ?`, staffID).
		OrderExpr(`"group".name ASC`)
	stats, err := scanAll(ctx, query, "list planned supervision blockers")
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

func (s *Store) CreatePlannedSupervisor(ctx context.Context, fields domain.PlannedSupervisorFields) (domain.PlannedSupervisor, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlannedSupervisor{}, domain.OperationStats{}, err
	}
	row := plannedSupervisorRow{TenantID: tenantID}
	applyPlannedSupervisorFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`activities.supervisors`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.PlannedSupervisor{}, stats, classifyWriteError("create planned supervisor", err, &stats)
	}
	stats.Rows = 1
	return plannedSupervisorToDomain(row), stats, nil
}

func (s *Store) UpdatePlannedSupervisor(ctx context.Context, id int64, fields domain.PlannedSupervisorFields) (domain.PlannedSupervisor, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PlannedSupervisor{}, false, domain.OperationStats{}, err
	}
	row := plannedSupervisorRow{ID: id, TenantID: tenantID}
	applyPlannedSupervisorFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`activities.supervisors`).
		Column("staff_id", "group_id", "is_primary", "valid_from", "valid_until", "calendar_period_id", "weekday").
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PlannedSupervisor{}, false, stats, nil
	}
	if err != nil {
		return domain.PlannedSupervisor{}, false, stats, classifyWriteError("update planned supervisor", err, &stats)
	}
	stats.Rows = 1
	return plannedSupervisorToDomain(row), true, stats, nil
}

func (s *Store) DeletePlannedSupervisor(ctx context.Context, id int64) (domain.OperationStats, error) {
	return s.deletePlannedSupervisors(ctx, "delete planned supervisor", func(query *bun.DeleteQuery) *bun.DeleteQuery { return query.Where("id = ?", id) })
}

func (s *Store) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (int64, domain.OperationStats, error) {
	stats, err := s.deletePlannedSupervisors(ctx, "delete planned supervisors by staff", func(query *bun.DeleteQuery) *bun.DeleteQuery { return query.Where("staff_id = ?", staffID) })
	return stats.Rows, stats, err
}

func (s *Store) deletePlannedSupervisors(ctx context.Context, operation string, apply func(*bun.DeleteQuery) *bun.DeleteQuery) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := apply(db.NewDelete().Table("activities.supervisors").Where("tenant_id = ?", tenantID)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError(operation, err, &stats)
	}
	_, err = rowsAffected(result, operation, &stats)
	return stats, err
}

func (s *Store) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Table("activities.supervisors").Set("is_primary = true").
		Where("tenant_id = ?", tenantID).Where("id = ?", id).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, classifyWriteError("set primary planned supervisor", err, &stats)
	}
	rows, err := rowsAffected(result, "set primary planned supervisor", &stats)
	return rows == 1, stats, err
}

func (s *Store) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Table("activities.supervisors").Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", tenantID).Where("id = ?", id).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, classifyWriteError("set planned supervisor valid_until", err, &stats)
	}
	rows, err := rowsAffected(result, "set planned supervisor valid_until", &stats)
	return rows == 1, stats, err
}

func (s *Store) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Table("activities.supervisors").Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", tenantID).Where("group_id = ?", groupID).Where("valid_until IS NULL")
	if periodID == nil {
		query = query.Where("calendar_period_id IS NULL")
	} else {
		query = query.Where("calendar_period_id = ?", *periodID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError("close open planned supervisors", err, &stats)
	}
	_, err = rowsAffected(result, "closed open planned supervisors", &stats)
	return stats, err
}

func (s *Store) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	deleted, err := execPlannedSupervisorWrite(ctx, db.NewDelete().Table("activities.supervisors").
		Where("tenant_id = ?", tenantID).Where("group_id = ?", groupID).
		Where("valid_from >= ?", validUntil).Where("valid_until IS NULL"), "delete future planned supervisors", &stats)
	if err != nil {
		return deleted, stats, err
	}
	total := deleted
	for _, primary := range []bool{false, true} {
		rows, updateErr := execPlannedSupervisorWrite(ctx, db.NewUpdate().Table("activities.supervisors").Set("valid_until = ?", validUntil).
			Where("tenant_id = ?", tenantID).Where("group_id = ?", groupID).
			Where("valid_from < ?", validUntil).Where("valid_until IS NULL").Where("is_primary = ?", primary),
			"cap active planned supervisors", &stats)
		total += rows
		if updateErr != nil {
			return total, stats, updateErr
		}
	}
	return total, stats, nil
}

type executableQuery interface {
	Exec(context.Context, ...any) (sql.Result, error)
}

func execPlannedSupervisorWrite(ctx context.Context, query executableQuery, operation string, stats *domain.OperationStats) (int64, error) {
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return 0, classifyWriteError(operation, err, stats)
	}
	return rowsAffected(result, operation, stats)
}

func plannedSupervisorSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`activities.supervisors AS "supervisor"`).
		ColumnExpr(`"supervisor".*`).Where(`"supervisor".tenant_id = ?`, tenantID)
}

func applyPlannedSupervisorFields(row *plannedSupervisorRow, fields domain.PlannedSupervisorFields) {
	row.StaffID, row.GroupID, row.IsPrimary = fields.StaffID, fields.GroupID, fields.IsPrimary
	row.ValidFrom, row.ValidUntil = fields.ValidFrom, fields.ValidUntil
	row.CalendarPeriodID, row.Weekday = fields.CalendarPeriodID, fields.Weekday
}

func plannedSupervisorToDomain(row plannedSupervisorRow) domain.PlannedSupervisor {
	return domain.PlannedSupervisor{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StaffID: row.StaffID, GroupID: row.GroupID, IsPrimary: row.IsPrimary, ValidFrom: row.ValidFrom,
		ValidUntil: row.ValidUntil, CalendarPeriodID: row.CalendarPeriodID, Weekday: row.Weekday}
}
