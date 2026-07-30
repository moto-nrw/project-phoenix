package suggestions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tablePosts      = "suggestions.posts"
	tablePostsAlias = `suggestions.posts AS "post"`
	tableVotes      = "suggestions.votes"
)

// postAuthorNameExpr resolves the display name of a post author. Parent posts
// resolve to the pseudonym "Elternteil" in SQL, so no caller — not even the
// operator API — ever receives a guardian's name for a feedback post.
const postAuthorNameExpr = `CASE
			WHEN "post".author_type = 'parent' THEN 'Elternteil'
			ELSE COALESCE(CONCAT(p.first_name, ' ', LEFT(p.last_name, 1), '.'), 'Unbekannt')
		END AS author_name`

// PostRepository implements suggestions.PostRepository
type PostRepository struct {
	db *bun.DB
}

// NewPostRepository creates a new PostRepository
func NewPostRepository(db *bun.DB) suggestions.PostRepository {
	return &PostRepository{db: db}
}

// Create inserts a new suggestion post
func (r *PostRepository) Create(ctx context.Context, post *suggestions.Post) error {
	if post == nil {
		return fmt.Errorf("post cannot be nil")
	}
	if err := post.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, post)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(post).
		ModelTableExpr(tablePosts).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create post", Err: err}
	}
	return nil
}

// FindByID retrieves a post by ID (without vote/author info).
// readerType controls visibility: ReaderTypeUser excludes hidden posts,
// ReaderTypeOperator returns all posts including hidden ones.
func (r *PostRepository) FindByID(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
	post := new(suggestions.Post)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(post).
		ModelTableExpr(tablePostsAlias).
		Where(`"post".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "post")

	// Operators moderate and therefore see hidden posts; every other reader
	// (staff board, parent board) does not.
	if readerType != suggestions.ReaderTypeOperator {
		query = query.Where(`"post".is_hidden = FALSE`)
	}

	err := query.Scan(ctx)
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

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(post).
		ModelTableExpr(tablePosts).
		Column("title", "description", "updated_at").
		Where("id = ?", post.ID).
		Returning("*")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update post", Err: err}
	}
	return nil
}

// UpdateStatus updates only the status of an existing post.
func (r *PostRepository) UpdateStatus(ctx context.Context, postID int64, status string) error {
	post := &suggestions.Post{Model: modelBase.Model{ID: postID}, Status: status}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(post).
		ModelTableExpr(tablePosts).
		Column("status", "updated_at").
		Where("id = ?", postID).
		Returning("*")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update post status", Err: err}
	}
	return nil
}

// UpdateHidden updates only the hidden state of an existing post.
func (r *PostRepository) UpdateHidden(ctx context.Context, postID int64, hidden bool) error {
	post := &suggestions.Post{
		Model:    modelBase.Model{ID: postID},
		IsHidden: hidden,
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(post).
		ModelTableExpr(tablePosts).
		Column("is_hidden", "updated_at").
		Where("id = ?", postID).
		Returning("*")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update post hidden", Err: err}
	}
	return nil
}

// Delete removes a post by ID
func (r *PostRepository) Delete(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		TableExpr(tablePosts).
		Where("id = ?", id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete post", Err: err}
	}
	return nil
}

// List returns all posts with author name and the current user's vote direction.
// readerType differentiates "operator" vs "user" in read-tracking subqueries.
// status filters by post status when non-empty.
func (r *PostRepository) List(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
	var posts []*suggestions.Post

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tablePostsAlias).
		ColumnExpr(`"post".*`).
		ColumnExpr(postAuthorNameExpr).
		ColumnExpr(`COALESCE(vc.upvotes, 0) AS upvotes`).
		ColumnExpr(`COALESCE(vc.downvotes, 0) AS downvotes`).
		ColumnExpr(`COALESCE(cc.comment_count, 0) AS comment_count`).
		ColumnExpr(`COALESCE(cc.unread_count, 0) AS unread_count`).
		ColumnExpr(`NOT EXISTS (
			SELECT 1 FROM suggestions.post_reads pr
			WHERE pr.account_id = ? AND pr.post_id = "post".id AND pr.reader_type = ?
		) AS is_new`, accountID, readerType).
		ColumnExpr(`v.direction AS user_vote`).
		ColumnExpr(`COALESCE("sch".name, '') AS school_name`).
		Join(`LEFT JOIN users.persons AS p ON p.account_id = "post".author_id`).
		Join(`LEFT JOIN suggestions.votes AS v ON v.post_id = "post".id AND v.voter_id = ?`, accountID).
		Join(`LEFT JOIN platform.schools AS "sch" ON "sch".id = "post".tenant_id`).
		Join(`LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE direction = 'up') AS upvotes,
				COUNT(*) FILTER (WHERE direction = 'down') AS downvotes
			FROM suggestions.votes WHERE post_id = "post".id
		) vc ON true`).
		Join(`LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS comment_count,
				COUNT(*) FILTER (WHERE
					(cr.last_read_at IS NULL OR c.created_at > cr.last_read_at)
					AND (? <> 'parent' OR c.author_type <> 'parent' OR c.author_id <> ?)
				) AS unread_count
			FROM suggestions.comments c
			LEFT JOIN suggestions.comment_reads cr
				ON cr.post_id = c.post_id AND cr.account_id = ? AND cr.reader_type = ?
			WHERE c.post_id = "post".id AND c.deleted_at IS NULL
		) cc ON true`, readerType, accountID, accountID, readerType)

	query = base.WithTenantFilter(ctx, query, "post")

	// Operators moderate and therefore see hidden posts; every other reader
	// (staff board, parent board) does not.
	if readerType != suggestions.ReaderTypeOperator {
		query = query.Where(`"post".is_hidden = FALSE`)
	}

	if status != "" {
		query = query.Where(`"post".status = ?`, status)
	}

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
// readerType differentiates "operator" vs "user" in read-tracking subqueries.
func (r *PostRepository) FindByIDWithVote(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
	post := new(suggestions.Post)

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tablePostsAlias).
		ColumnExpr(`"post".*`).
		ColumnExpr(postAuthorNameExpr).
		ColumnExpr(`COALESCE(vc.upvotes, 0) AS upvotes`).
		ColumnExpr(`COALESCE(vc.downvotes, 0) AS downvotes`).
		ColumnExpr(`COALESCE(cc.comment_count, 0) AS comment_count`).
		ColumnExpr(`COALESCE(cc.unread_count, 0) AS unread_count`).
		ColumnExpr(`v.direction AS user_vote`).
		ColumnExpr(`COALESCE("sch".name, '') AS school_name`).
		Join(`LEFT JOIN users.persons AS p ON p.account_id = "post".author_id`).
		Join(`LEFT JOIN suggestions.votes AS v ON v.post_id = "post".id AND v.voter_id = ?`, accountID).
		Join(`LEFT JOIN platform.schools AS "sch" ON "sch".id = "post".tenant_id`).
		Join(`LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE direction = 'up') AS upvotes,
				COUNT(*) FILTER (WHERE direction = 'down') AS downvotes
			FROM suggestions.votes WHERE post_id = "post".id
		) vc ON true`).
		Join(`LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS comment_count,
				COUNT(*) FILTER (WHERE
					(cr.last_read_at IS NULL OR c.created_at > cr.last_read_at)
					AND (? <> 'parent' OR c.author_type <> 'parent' OR c.author_id <> ?)
				) AS unread_count
			FROM suggestions.comments c
			LEFT JOIN suggestions.comment_reads cr
				ON cr.post_id = c.post_id AND cr.account_id = ? AND cr.reader_type = ?
			WHERE c.post_id = "post".id AND c.deleted_at IS NULL
		) cc ON true`, readerType, accountID, accountID, readerType).
		Where(`"post".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "post")

	// Operators moderate and therefore see hidden posts; every other reader
	// (staff board, parent board) does not.
	if readerType != suggestions.ReaderTypeOperator {
		query = query.Where(`"post".is_hidden = FALSE`)
	}

	err := query.Scan(ctx, post)
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
	query := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr(tablePosts).
		Set(`score = (
			SELECT COALESCE(SUM(CASE WHEN direction = 'up' THEN 1 WHEN direction = 'down' THEN -1 ELSE 0 END), 0)
			FROM suggestions.votes WHERE post_id = ?
		)`, postID).
		Where("id = ?", postID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "recalculate score", Err: err}
	}
	return nil
}
