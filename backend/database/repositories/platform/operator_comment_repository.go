package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tableSuggestionsOperatorComments      = "suggestions.operator_comments"
	tableSuggestionsOperatorCommentsAlias = `suggestions.operator_comments AS "comment"`
)

// OperatorCommentRepository implements platform.OperatorCommentRepository interface
type OperatorCommentRepository struct {
	*base.Repository[*platform.OperatorComment]
	db *bun.DB
}

// NewOperatorCommentRepository creates a new OperatorCommentRepository
func NewOperatorCommentRepository(db *bun.DB) platform.OperatorCommentRepository {
	return &OperatorCommentRepository{
		Repository: base.NewRepository[*platform.OperatorComment](db, tableSuggestionsOperatorComments, "OperatorComment"),
		db:         db,
	}
}

// Create inserts a new operator comment
func (r *OperatorCommentRepository) Create(ctx context.Context, comment *platform.OperatorComment) error {
	if comment == nil {
		return fmt.Errorf("comment cannot be nil")
	}

	if err := comment.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, comment)
}

// FindByID retrieves a comment by ID
func (r *OperatorCommentRepository) FindByID(ctx context.Context, id int64) (*platform.OperatorComment, error) {
	comment := new(platform.OperatorComment)
	err := r.db.NewSelect().
		Model(comment).
		ModelTableExpr(tableSuggestionsOperatorCommentsAlias).
		Where(`"comment".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find operator comment by id",
			Err: err,
		}
	}

	return comment, nil
}

// Update updates a comment
func (r *OperatorCommentRepository) Update(ctx context.Context, comment *platform.OperatorComment) error {
	if comment == nil {
		return fmt.Errorf("comment cannot be nil")
	}

	if err := comment.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, comment)
}

// Delete removes a comment by ID
func (r *OperatorCommentRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// FindByPostID retrieves all comments for a post
func (r *OperatorCommentRepository) FindByPostID(ctx context.Context, postID int64, includeInternal bool) ([]*platform.OperatorComment, error) {
	var comments []*platform.OperatorComment
	query := r.db.NewSelect().
		Model(&comments).
		ModelTableExpr(tableSuggestionsOperatorCommentsAlias).
		Relation("Operator").
		Where(`"comment".post_id = ?`, postID)

	if !includeInternal {
		query = query.Where(`"comment".is_internal = false`)
	}

	err := query.
		Order(`"comment".created_at ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find operator comments by post id",
			Err: err,
		}
	}

	return comments, nil
}

// CountByPostID counts comments for a post
func (r *OperatorCommentRepository) CountByPostID(ctx context.Context, postID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*platform.OperatorComment)(nil)).
		ModelTableExpr(tableSuggestionsOperatorCommentsAlias).
		Where(`"comment".post_id = ?`, postID).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count operator comments by post id",
			Err: err,
		}
	}

	return count, nil
}
