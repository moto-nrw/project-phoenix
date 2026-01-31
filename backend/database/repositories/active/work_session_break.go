package active

import (
	"context"
	"database/sql"
	"fmt"
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
	return &WorkSessionBreakRepository{
		Repository: base.NewRepository[*active.WorkSessionBreak](db, tableActiveWorkSessionBreaks, "WorkSessionBreak"),
		db:         db,
	}
}

// Create overrides base Create to handle validation
func (r *WorkSessionBreakRepository) Create(ctx context.Context, brk *active.WorkSessionBreak) error {
	if brk == nil {
		return fmt.Errorf("work session break cannot be nil")
	}

	if err := brk.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, brk)
}

// List overrides base List to use QueryOptions
func (r *WorkSessionBreakRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.WorkSessionBreak, error) {
	var breaks []*active.WorkSessionBreak
	query := r.db.NewSelect().
		Model(&breaks).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak)

	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return breaks, nil
}

// GetBySessionID returns all breaks for a given session ordered by started_at
func (r *WorkSessionBreakRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*active.WorkSessionBreak, error) {
	var breaks []*active.WorkSessionBreak
	err := r.db.NewSelect().
		Model(&breaks).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak).
		Where(`"work_session_break".session_id = ?`, sessionID).
		OrderExpr(`"work_session_break".started_at ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get breaks by session ID",
			Err: err,
		}
	}

	return breaks, nil
}

// GetActiveBySessionID returns the currently active break for a session, or nil if none
func (r *WorkSessionBreakRepository) GetActiveBySessionID(ctx context.Context, sessionID int64) (*active.WorkSessionBreak, error) {
	brk := new(active.WorkSessionBreak)
	err := r.db.NewSelect().
		Model(brk).
		ModelTableExpr(tableExprActiveWorkSessionBreaksAsWorkSessionBreak).
		Where(`"work_session_break".session_id = ?`, sessionID).
		Where(`"work_session_break".ended_at IS NULL`).
		Scan(ctx)

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
	_, err := r.db.NewUpdate().
		Table(tableActiveWorkSessionBreaks).
		Set("ended_at = ?", endedAt).
		Set("duration_minutes = ?", durationMinutes).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND ended_at IS NULL", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end break",
			Err: err,
		}
	}

	return nil
}
