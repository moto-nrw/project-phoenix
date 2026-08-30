package active

import (
	"context"
	"database/sql"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	tableActiveWorkSessionBreaks                       = "active.work_session_breaks"
	tableExprActiveWorkSessionBreaksAsWorkSessionBreak = `active.work_session_breaks AS "work_session_break"`
)

// WorkSessionBreakRepository implements active.WorkSessionBreakRepository
type WorkSessionBreakRepository struct {
	*base.Repository[*active.WorkSessionBreak]
	db *bun.DB
}

// NewWorkSessionBreakRepository creates a new WorkSessionBreakRepository
func NewWorkSessionBreakRepository(db *bun.DB) active.WorkSessionBreakRepository {
	repo := base.NewRepository[*active.WorkSessionBreak](db, tableActiveWorkSessionBreaks, "WorkSessionBreak")
	repo.TenantScoped = true
	return &WorkSessionBreakRepository{
		Repository: repo,
		db:         db,
	}
}

// List overrides base List to use QueryOptions
func (r *WorkSessionBreakRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.WorkSessionBreak, error) {
	return r.ListWithOptions(ctx, options)
}

// GetBySessionID returns all breaks for a given session ordered by started_at
func (r *WorkSessionBreakRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*active.WorkSessionBreak, error) {
	var breaks []*active.WorkSessionBreak
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&breaks).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak).
		Where(`"work_session_break".session_id = ?`, sessionID).
		OrderExpr(`"work_session_break".started_at ASC`)

	query = base.WithTenantFilter(ctx, query, "work_session_break")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get breaks by session ID",
			Err: err,
		}
	}

	return breaks, nil
}

// GetBySessionIDs returns all breaks for multiple sessions in one round trip,
// grouped by session ID and ordered by start time within each group.
func (r *WorkSessionBreakRepository) GetBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64][]*active.WorkSessionBreak, error) {
	result := make(map[int64][]*active.WorkSessionBreak, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}

	var breaks []*active.WorkSessionBreak
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&breaks).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak).
		Where(`"work_session_break".session_id IN (?)`, bun.List(sessionIDs)).
		OrderExpr(`"work_session_break".session_id ASC, "work_session_break".started_at ASC`)
	query = base.WithTenantFilter(ctx, query, "work_session_break")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get breaks by session IDs", Err: err}
	}
	for _, brk := range breaks {
		result[brk.SessionID] = append(result[brk.SessionID], brk)
	}
	return result, nil
}

// GetActiveBySessionID returns the currently active break for a session, or nil if none
func (r *WorkSessionBreakRepository) GetActiveBySessionID(ctx context.Context, sessionID int64) (*active.WorkSessionBreak, error) {
	brk := new(active.WorkSessionBreak)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(brk).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak).
		Where(`"work_session_break".session_id = ?`, sessionID).
		Where(`"work_session_break".ended_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "work_session_break")

	err := query.Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "get active break by session ID",
			Err: err,
		}
	}

	return brk, nil
}

// EndBreak sets ended_at and duration_minutes on a break
func (r *WorkSessionBreakRepository) EndBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error {
	brk := &active.WorkSessionBreak{
		Model:           modelBase.Model{ID: id, UpdatedAt: time.Now()},
		EndedAt:         &endedAt,
		DurationMinutes: durationMinutes,
	}
	updated, err := r.UpdateColumnsIfNull(ctx, brk, "ended_at", "duration_minutes", "updated_at")
	if err != nil {
		return base.UpdateOperationError(err, "end break")
	}
	return base.AssertRowsAffectedCount(updated, 1, "end break")
}

// UpdateDuration updates the duration and ended_at of a completed break
func (r *WorkSessionBreakRepository) UpdateDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error {
	brk := &active.WorkSessionBreak{Model: modelBase.Model{ID: id, UpdatedAt: time.Now()}, DurationMinutes: durationMinutes, EndedAt: &endedAt}
	_, err := r.UpdateColumns(ctx, brk, "duration_minutes", "ended_at", "updated_at")
	return err
}

// GetExpiredBreaks returns all active breaks with planned_end_time <= before
func (r *WorkSessionBreakRepository) GetExpiredBreaks(ctx context.Context, before time.Time) ([]*active.WorkSessionBreak, error) {
	var breaks []*active.WorkSessionBreak
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&breaks).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak).
		Where(`"work_session_break".ended_at IS NULL`).
		Where(`"work_session_break".planned_end_time IS NOT NULL`).
		Where(`"work_session_break".planned_end_time <= ?`, before)

	query = base.WithTenantFilter(ctx, query, "work_session_break")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get expired breaks",
			Err: err,
		}
	}

	return breaks, nil
}
