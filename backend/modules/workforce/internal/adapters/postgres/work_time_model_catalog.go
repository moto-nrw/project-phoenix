package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// SetWorkTimeModelArchived marks a template as archived or offers it again
// (#6891). Archiving twice keeps the first instant. found is false when the
// template does not exist in the caller's school.
func (s *Store) SetWorkTimeModelArchived(ctx context.Context, id int64, archived bool) (domain.WorkTimeModel, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkTimeModel{}, false, domain.OperationStats{}, err
	}
	row := &workTimeModelRow{ID: id, TenantID: scope.TenantID}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableWorkTimeModels).
		Set("updated_at = NOW()").
		Where("id = ?", id)
	if archived {
		query = query.Set("archived_at = COALESCE(archived_at, NOW())")
	} else {
		query = query.Set("archived_at = NULL")
	}
	query = scopeQuery(scope, query, "tenant_id = ?")

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.WorkTimeModel{}, false, stats, fmt.Errorf("workforce postgres: archive work-time model: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.WorkTimeModel{}, false, stats, fmt.Errorf("workforce postgres: archive work-time model rows affected: %w", err)
	}
	if affected != 1 {
		return domain.WorkTimeModel{}, false, stats, nil
	}
	stats.Rows = affected
	model, stats, err := s.reload(ctx, db, row, stats)
	return model, err == nil, stats, err
}

// StaffByWorkTimeModel lists, per template of the caller's school, every
// membership bound to it, live or retired, in one read.
func (s *Store) StaffByWorkTimeModel(ctx context.Context) (map[int64][]int64, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := []staffEmploymentRow{}
	if err := staffEmploymentTenant(db.NewSelect().Model(&rows).
		Column("membership_id", "work_time_model_id").
		Where(`"profile".work_time_model_id IS NOT NULL`).
		OrderExpr(`"profile".membership_id ASC`), scope).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("workforce postgres: list staff by work time model: %w", err)
	}
	byModel := make(map[int64][]int64)
	for _, row := range rows {
		if row.WorkTimeModelID != nil {
			byModel[*row.WorkTimeModelID] = append(byModel[*row.WorkTimeModelID], row.MembershipID)
		}
	}
	return byModel, nil
}
