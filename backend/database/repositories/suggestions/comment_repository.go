package suggestions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tableSuggestionsComments      = "suggestions.comments"
	tableSuggestionsCommentsAlias = `suggestions.comments AS "comment"`
)

// CommentRepository implements suggestions.CommentRepository interface
type CommentRepository struct {
	db *bun.DB
}

// NewCommentRepository creates a new CommentRepository
func NewCommentRepository(db *bun.DB) suggestions.CommentRepository {
	return &CommentRepository{db: db}
}

// Create inserts a new comment
func (r *CommentRepository) Create(ctx context.Context, comment *suggestions.Comment) error {
	if comment == nil {
		return fmt.Errorf("comment cannot be nil")
	}

	if err := comment.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(comment).
		ModelTableExpr(tableSuggestionsComments).
		Returning("id, created_at, updated_at").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create comment",
			Err: err,
		}
	}

	return nil
}

// FindByID retrieves a comment by ID
func (r *CommentRepository) FindByID(ctx context.Context, id int64) (*suggestions.Comment, error) {
	comment := new(suggestions.Comment)
	err := r.db.NewSelect().
		Model(comment).
		ModelTableExpr(tableSuggestionsCommentsAlias).
		Where(`"comment".id = ?`, id).
		Where(`"comment".deleted_at IS NULL`).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find comment by id",
			Err: err,
		}
	}

	return comment, nil
}

// FindByPostID retrieves all comments for a post with author names resolved via polymorphic joins.
func (r *CommentRepository) FindByPostID(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	var comments []*suggestions.Comment

	err := r.db.NewSelect().
		Model(&comments).
		ModelTableExpr(tableSuggestionsCommentsAlias).
		ColumnExpr(`"comment".*`).
		ColumnExpr(`CASE
			WHEN "comment".author_type = 'operator' THEN "op".display_name
			WHEN "comment".author_type = 'user' THEN CONCAT("person".first_name, ' ', "person".last_name)
		END AS author_name`).
		Join(`LEFT JOIN platform.operators AS "op" ON "comment".author_type = 'operator' AND "op".id = "comment".author_id`).
		Join(`LEFT JOIN users.persons AS "person" ON "comment".author_type = 'user' AND "person".account_id = "comment".author_id`).
		Where(`"comment".post_id = ?`, postID).
		Where(`"comment".deleted_at IS NULL`).
		OrderExpr(`"comment".created_at ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find comments by post id",
			Err: err,
		}
	}

	return comments, nil
}

// Delete soft-deletes a comment by setting deleted_at
func (r *CommentRepository) Delete(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		ModelTableExpr(tableSuggestionsComments).
		Set("deleted_at = ?", now).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete comment",
			Err: err,
		}
	}

	return nil
}

// CountByPostID counts non-deleted comments for a post
func (r *CommentRepository) CountByPostID(ctx context.Context, postID int64) (int, error) {
	count, err := r.db.NewSelect().
		ModelTableExpr(tableSuggestionsCommentsAlias).
		Where(`"comment".post_id = ?`, postID).
		Where(`"comment".deleted_at IS NULL`).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count comments by post id",
			Err: err,
		}
	}

	return count, nil
}
