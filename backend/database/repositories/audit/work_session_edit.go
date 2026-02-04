package audit

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	tableWorkSessionEdits        = "audit.work_session_edits"
	tableWorkSessionEditsAliased = `audit.work_session_edits AS "work_session_edit"`
	whereSessionIDEquals         = `"work_session_edit".session_id = ?`
)

// WorkSessionEditRepository implements audit.WorkSessionEditRepository interface
type WorkSessionEditRepository struct {
	db *bun.DB
}

// NewWorkSessionEditRepository creates a new WorkSessionEditRepository
func NewWorkSessionEditRepository(db *bun.DB) audit.WorkSessionEditRepository {
	return &WorkSessionEditRepository{db: db}
}

// CreateBatch inserts multiple edit audit records
func (r *WorkSessionEditRepository) CreateBatch(ctx context.Context, edits []*audit.WorkSessionEdit) error {
	if len(edits) == 0 {
		return nil
	}

	for _, edit := range edits {
		if edit == nil {
			return &modelBase.DatabaseError{
				Op:  "create batch",
				Err: errors.New("edit cannot be nil"),
			}
		}
		if err := edit.Validate(); err != nil {
			return &modelBase.DatabaseError{
				Op:  "validate",
				Err: err,
			}
		}
	}

	_, err := r.db.NewInsert().
		Model(&edits).
		ModelTableExpr(tableWorkSessionEdits).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create batch",
			Err: err,
		}
	}

	return nil
}

// GetBySessionID returns all edit records for a session, ordered by creation time descending
func (r *WorkSessionEditRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*audit.WorkSessionEdit, error) {
	var edits []*audit.WorkSessionEdit
	err := r.db.NewSelect().
		Model(&edits).
		ModelTableExpr(tableWorkSessionEditsAliased).
		Where(whereSessionIDEquals, sessionID).
		Order(orderByCreatedAtDesc).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get by session ID",
			Err: err,
		}
	}

	return edits, nil
}

// CountBySessionID returns the number of edit records for a session
func (r *WorkSessionEditRepository) CountBySessionID(ctx context.Context, sessionID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*audit.WorkSessionEdit)(nil)).
		ModelTableExpr(tableWorkSessionEditsAliased).
		Where(whereSessionIDEquals, sessionID).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by session ID",
			Err: err,
		}
	}

	return count, nil
}

// CountBySessionIDs returns a map of session ID → edit count for multiple sessions
func (r *WorkSessionEditRepository) CountBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64]int, error) {
	if len(sessionIDs) == 0 {
		return make(map[int64]int), nil
	}

	type countResult struct {
		SessionID int64 `bun:"session_id"`
		Count     int   `bun:"count"`
	}

	var results []countResult
	err := r.db.NewSelect().
		ModelTableExpr(tableWorkSessionEdits).
		ColumnExpr("session_id").
		ColumnExpr("COUNT(*) AS count").
		Where("session_id IN (?)", bun.In(sessionIDs)).
		GroupExpr("session_id").
		Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count by session IDs",
			Err: err,
		}
	}

	counts := make(map[int64]int, len(results))
	for _, r := range results {
		counts[r.SessionID] = r.Count
	}

	return counts, nil
}
