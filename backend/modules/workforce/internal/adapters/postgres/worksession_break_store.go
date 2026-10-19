package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// --- work session breaks ---

func (s *Store) FindWorkSessionBreak(ctx context.Context, id int64) (domain.WorkSessionBreak, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSessionBreak{}, false, domain.OperationStats{}, err
	}
	row := &workSessionBreakRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`).
		Where(`"work_session_break".id = ?`, id), aliasWorkSessionBreak, scope)
	found, stats, err := scanOne(ctx, query, "find work session break")
	if err != nil || !found {
		return domain.WorkSessionBreak{}, found, stats, err
	}
	return workSessionBreakToDomain(*row), true, stats, nil
}

func (s *Store) ListWorkSessionBreaks(ctx context.Context, filter domain.WorkSessionBreakFilter) ([]domain.WorkSessionBreak, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []workSessionBreakRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`), aliasWorkSessionBreak, scope)
	if filter.SessionID > 0 {
		query = query.Where(`"work_session_break".session_id = ?`, filter.SessionID)
	}
	if filter.SessionIDs != nil {
		if len(filter.SessionIDs) == 0 {
			query = query.Where("FALSE")
		} else {
			query = query.Where(`"work_session_break".session_id IN (?)`, bun.List(filter.SessionIDs))
		}
	}
	if filter.Active != nil {
		if *filter.Active {
			query = query.Where(`"work_session_break".ended_at IS NULL`)
		} else {
			query = query.Where(`"work_session_break".ended_at IS NOT NULL`)
		}
	}
	query = limitOffset(query.OrderExpr(`"work_session_break".session_id ASC, "work_session_break".started_at ASC, "work_session_break".id ASC`), filter.Limit, filter.Offset)
	stats, err := scanAll(ctx, query, "list work session breaks")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return workSessionBreaksToDomain(rows), stats, nil
}

func (s *Store) ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]domain.WorkSessionBreak, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []workSessionBreakRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`).
		Where(`"work_session_break".ended_at IS NULL`).
		Where(`"work_session_break".planned_end_time IS NOT NULL`).
		Where(`"work_session_break".planned_end_time <= ?`, before), aliasWorkSessionBreak, scope).
		OrderExpr(`"work_session_break".id ASC`)
	stats, err := scanAll(ctx, query, "list expired work session breaks")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return workSessionBreaksToDomain(rows), stats, nil
}

func (s *Store) CreateWorkSessionBreak(ctx context.Context, value domain.WorkSessionBreak) (domain.WorkSessionBreak, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSessionBreak{}, domain.OperationStats{}, err
	}
	row := workSessionBreakFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableWorkSessionBreaks).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.WorkSessionBreak{}, stats, fmt.Errorf("workforce postgres: insert work session break: %w", err)
	}
	stats.Rows = 1
	return workSessionBreakToDomain(*row), stats, nil
}

func (s *Store) UpdateWorkSessionBreak(ctx context.Context, value domain.WorkSessionBreak) (domain.WorkSessionBreak, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSessionBreak{}, false, domain.OperationStats{}, err
	}
	row := workSessionBreakFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasWorkSessionBreak, scope)
	stats, err := execAffected(ctx, query, "update work session break")
	if err != nil {
		return domain.WorkSessionBreak{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.WorkSessionBreak{}, false, stats, nil
	}
	return workSessionBreakToDomain(*row), true, stats, nil
}

func (s *Store) DeleteWorkSessionBreak(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*workSessionBreakRow)(nil)).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`).
		Where(`"work_session_break".id = ?`, id), aliasWorkSessionBreak, scope)
	return execAffected(ctx, query, "delete work session break")
}

func (s *Store) EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*workSessionBreakRow)(nil)).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`).
		Set("ended_at = ?", endedAt).
		Set("duration_minutes = ?", durationMinutes).
		Set("updated_at = NOW()").
		Where(`"work_session_break".id = ?`, id).
		Where(`"work_session_break".ended_at IS NULL`), aliasWorkSessionBreak, scope)
	stats, err := execAffected(ctx, query, "end work session break")
	return stats.Rows, stats, err
}

func (s *Store) SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*workSessionBreakRow)(nil)).
		ModelTableExpr(tableWorkSessionBreaks+` AS "work_session_break"`).
		Set("duration_minutes = ?", durationMinutes).
		Set("ended_at = ?", endedAt).
		Set("updated_at = NOW()").
		Where(`"work_session_break".id = ?`, id), aliasWorkSessionBreak, scope)
	stats, err := execAffected(ctx, query, "update work session break duration")
	return stats.Rows, stats, err
}
