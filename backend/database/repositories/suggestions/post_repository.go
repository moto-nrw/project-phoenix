package suggestions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tablePosts      = "suggestions.posts"
	tablePostsAlias = `suggestions.posts AS "post"`
	tableVotes      = "suggestions.votes"
)

// PostRepository implements suggestions.PostRepository
type PostRepository struct {
	db *bun.DB
}

// NewPostRepository creates a new PostRepository
func NewPostRepository(db *bun.DB) suggestions.PostRepository {
	return &PostRepository{db: db}
}

// conn returns the transaction from context if present, otherwise the base DB.
func (r *PostRepository) conn(ctx context.Context) bun.IDB {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return tx
	}
	return r.db
}

// Create inserts a new suggestion post
func (r *PostRepository) Create(ctx context.Context, post *suggestions.Post) error {
	if post == nil {
		return fmt.Errorf("post cannot be nil")
	}
	if err := post.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(post).
		ModelTableExpr(tablePosts).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create post", Err: err}
	}
	return nil
}

// FindByID retrieves a post by ID (without vote/author info)
func (r *PostRepository) FindByID(ctx context.Context, id int64) (*suggestions.Post, error) {
	post := new(suggestions.Post)
	err := r.db.NewSelect().
		Model(post).
		ModelTableExpr(tablePostsAlias).
		Where(`"post".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find post by id", Err: err}
	}
	return post, nil
}

// Update updates an existing post
func (r *PostRepository) Update(ctx context.Context, post *suggestions.Post) error {
	if post == nil {
		return fmt.Errorf("post cannot be nil")
	}

	_, err := r.db.NewUpdate().
		Model(post).
		ModelTableExpr(tablePostsAlias).
		Column("title", "description", "status", "updated_at").
		WherePK().
		Returning("*").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update post", Err: err}
	}
	return nil
}

// Delete removes a post by ID
func (r *PostRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().
		TableExpr(tablePosts).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete post", Err: err}
	}
	return nil
}

// List returns all posts with author name and the current user's vote direction.
func (r *PostRepository) List(ctx context.Context, accountID int64, sortBy string) ([]*suggestions.Post, error) {
	var posts []*suggestions.Post

	query := r.db.NewSelect().
		TableExpr(tablePostsAlias).
		ColumnExpr(`"post".*`).
		ColumnExpr(`COALESCE(CONCAT(p.first_name, ' ', LEFT(p.last_name, 1), '.'), 'Unbekannt') AS author_name`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.votes WHERE post_id = "post".id AND direction = 'up') AS upvotes`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.votes WHERE post_id = "post".id AND direction = 'down') AS downvotes`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.comments WHERE post_id = "post".id AND deleted_at IS NULL) AS comment_count`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.comments c
			LEFT JOIN suggestions.comment_reads cr ON cr.post_id = c.post_id AND cr.account_id = ?
			WHERE c.post_id = "post".id
			AND c.deleted_at IS NULL
			AND (cr.last_read_at IS NULL OR c.created_at > cr.last_read_at)
		) AS unread_count`, accountID).
		ColumnExpr(`NOT EXISTS (
			SELECT 1 FROM suggestions.post_reads pr
			WHERE pr.account_id = ? AND pr.post_id = "post".id
		) AS is_new`, accountID).
		ColumnExpr(`v.direction AS user_vote`).
		Join(`LEFT JOIN users.persons AS p ON p.account_id = "post".author_id`).
		Join(`LEFT JOIN suggestions.votes AS v ON v.post_id = "post".id AND v.voter_id = ?`, accountID)

	switch sortBy {
	case "newest":
		query = query.OrderExpr(`"post".created_at DESC`)
	case "status":
		// Order by status: open → planned → done → rejected
		query = query.OrderExpr(`CASE "post".status
			WHEN 'open' THEN 1
			WHEN 'planned' THEN 2
			WHEN 'done' THEN 3
			WHEN 'rejected' THEN 4
			ELSE 5 END`).
			OrderExpr(`"post".score DESC`)
	default: // "score"
		query = query.OrderExpr(`"post".score DESC, "post".created_at DESC`)
	}

	err := query.Scan(ctx, &posts)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list posts", Err: err}
	}
	return posts, nil
}

// FindByIDWithVote returns a single post with author name and user's vote.
func (r *PostRepository) FindByIDWithVote(ctx context.Context, id int64, accountID int64) (*suggestions.Post, error) {
	post := new(suggestions.Post)

	err := r.db.NewSelect().
		TableExpr(tablePostsAlias).
		ColumnExpr(`"post".*`).
		ColumnExpr(`COALESCE(CONCAT(p.first_name, ' ', LEFT(p.last_name, 1), '.'), 'Unbekannt') AS author_name`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.votes WHERE post_id = "post".id AND direction = 'up') AS upvotes`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.votes WHERE post_id = "post".id AND direction = 'down') AS downvotes`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.comments WHERE post_id = "post".id AND deleted_at IS NULL) AS comment_count`).
		ColumnExpr(`(SELECT COUNT(*) FROM suggestions.comments c
			LEFT JOIN suggestions.comment_reads cr ON cr.post_id = c.post_id AND cr.account_id = ?
			WHERE c.post_id = "post".id
			AND c.deleted_at IS NULL
			AND (cr.last_read_at IS NULL OR c.created_at > cr.last_read_at)
		) AS unread_count`, accountID).
		ColumnExpr(`v.direction AS user_vote`).
		Join(`LEFT JOIN users.persons AS p ON p.account_id = "post".author_id`).
		Join(`LEFT JOIN suggestions.votes AS v ON v.post_id = "post".id AND v.voter_id = ?`, accountID).
		Where(`"post".id = ?`, id).
		Scan(ctx, post)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find post by id with vote", Err: err}
	}
	return post, nil
}

// RecalculateScore updates the denormalized score on a post.
func (r *PostRepository) RecalculateScore(ctx context.Context, postID int64) error {
	_, err := r.conn(ctx).NewUpdate().
		TableExpr(tablePosts).
		Set(`score = (
			SELECT COALESCE(SUM(CASE WHEN direction = 'up' THEN 1 WHEN direction = 'down' THEN -1 ELSE 0 END), 0)
			FROM suggestions.votes WHERE post_id = ?
		)`, postID).
		Where("id = ?", postID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "recalculate score", Err: err}
	}
	return nil
}
