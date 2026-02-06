package suggestions

import (
	"context"
	"database/sql"
	"errors"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

const (
	tableCommentReads      = "suggestions.comment_reads"
	tableCommentReadsAlias = `suggestions.comment_reads AS "cr"`
)

// CommentReadRepository implements suggestions.CommentReadRepository
type CommentReadRepository struct {
	db *bun.DB
}

// NewCommentReadRepository creates a new CommentReadRepository
func NewCommentReadRepository(db *bun.DB) suggestions.CommentReadRepository {
	return &CommentReadRepository{db: db}
}

// Upsert creates or updates the last_read_at timestamp for a user on a post
func (r *CommentReadRepository) Upsert(ctx context.Context, accountID, postID int64) error {
	cr := &suggestions.CommentRead{
		AccountID:  accountID,
		PostID:     postID,
		LastReadAt: time.Now(),
	}

	_, err := r.db.NewInsert().
		Model(cr).
		On("CONFLICT (account_id, post_id) DO UPDATE").
		Set("last_read_at = EXCLUDED.last_read_at").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert comment read", Err: err}
	}
	return nil
}

// GetLastReadAt returns when a user last read comments on a post (nil if never)
func (r *CommentReadRepository) GetLastReadAt(ctx context.Context, accountID, postID int64) (*time.Time, error) {
	cr := new(suggestions.CommentRead)
	err := r.db.NewSelect().
		Model(cr).
		ModelTableExpr(tableCommentReadsAlias).
		Where(`"cr".account_id = ?`, accountID).
		Where(`"cr".post_id = ?`, postID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get last read at", Err: err}
	}
	return &cr.LastReadAt, nil
}

// CountUnreadByPost counts comments on a post created after the user's last read time
func (r *CommentReadRepository) CountUnreadByPost(ctx context.Context, accountID, postID int64) (int, error) {
	// Get the count of comments created after the user's last read time
	// If the user has never read, all comments are unread
	count, err := r.db.NewSelect().
		TableExpr("suggestions.comments AS c").
		Where("c.post_id = ?", postID).
		Where("c.deleted_at IS NULL").
		Where(`c.created_at > COALESCE(
			(SELECT cr.last_read_at FROM suggestions.comment_reads cr
			 WHERE cr.account_id = ? AND cr.post_id = ?),
			'1970-01-01'::timestamptz
		)`, accountID, postID).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread comments", Err: err}
	}
	return count, nil
}

// CountTotalUnread counts all unread comments across all posts for a user
func (r *CommentReadRepository) CountTotalUnread(ctx context.Context, accountID int64) (int, error) {
	count, err := r.db.NewSelect().
		TableExpr("suggestions.comments AS c").
		Where("c.deleted_at IS NULL").
		Where(`c.created_at > COALESCE(
			(SELECT cr.last_read_at FROM suggestions.comment_reads cr
			 WHERE cr.account_id = ? AND cr.post_id = c.post_id),
			'1970-01-01'::timestamptz
		)`, accountID).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count total unread comments", Err: err}
	}
	return count, nil
}
