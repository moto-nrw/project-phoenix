package suggestions

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

const tableCommentReadsAlias = `suggestions.comment_reads AS "cr"`

// ownParentCommentExclusion keeps a guardian's own comments out of their
// unread badge. Staff and operators retain the existing unread semantics.
const ownParentCommentExclusion = `( ? <> 'parent' OR c.author_type <> 'parent' OR c.author_id <> ?)`

// CommentReadRepository implements suggestions.CommentReadRepository
type CommentReadRepository struct {
	db *bun.DB
}

// NewCommentReadRepository creates a new CommentReadRepository
func NewCommentReadRepository(db *bun.DB) suggestions.CommentReadRepository {
	return &CommentReadRepository{db: db}
}

// Upsert creates or updates the last_read_at timestamp for a reader on a post
func (r *CommentReadRepository) Upsert(ctx context.Context, accountID, postID int64, readerType string) error {
	cr := &suggestions.CommentRead{
		AccountID:  accountID,
		PostID:     postID,
		ReaderType: readerType,
		LastReadAt: time.Now(),
	}

	base.EnsureTenantID(ctx, cr)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(cr).
		On("CONFLICT (account_id, post_id, reader_type) DO UPDATE").
		Set("last_read_at = EXCLUDED.last_read_at").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert comment read", Err: err}
	}
	return nil
}

// GetLastReadAt returns when a reader last read comments on a post (nil if never)
func (r *CommentReadRepository) GetLastReadAt(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error) {
	cr := new(suggestions.CommentRead)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(cr).
		ModelTableExpr(tableCommentReadsAlias).
		Where(`"cr".account_id = ?`, accountID).
		Where(`"cr".post_id = ?`, postID).
		Where(`"cr".reader_type = ?`, readerType).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get last read at", Err: err}
	}
	return &cr.LastReadAt, nil
}

// CountUnreadByPost counts comments on a visible post created after the reader's last read time.
func (r *CommentReadRepository) CountUnreadByPost(ctx context.Context, accountID, postID int64, readerType string) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("suggestions.comments AS c").
		Join("JOIN suggestions.posts AS p ON p.id = c.post_id").
		Where("c.post_id = ?", postID).
		Where("p.is_hidden = FALSE").
		Where("c.deleted_at IS NULL").
		Where(ownParentCommentExclusion, readerType, accountID).
		Where(`c.created_at > COALESCE(
			(SELECT cr.last_read_at FROM suggestions.comment_reads cr
			 WHERE cr.account_id = ? AND cr.post_id = ? AND cr.reader_type = ?),
			'1970-01-01'::timestamptz
		)`, accountID, postID, readerType)

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread comments", Err: err}
	}
	return count, nil
}

// CountTotalUnread counts all unread comments across visible posts for a reader.
func (r *CommentReadRepository) CountTotalUnread(ctx context.Context, accountID int64, readerType string) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("suggestions.comments AS c").
		Join("JOIN suggestions.posts AS p ON p.id = c.post_id").
		Where("p.is_hidden = FALSE").
		Where("c.deleted_at IS NULL").
		Where(ownParentCommentExclusion, readerType, accountID).
		Where(`c.created_at > COALESCE(
			(SELECT cr.last_read_at FROM suggestions.comment_reads cr
			 WHERE cr.account_id = ? AND cr.post_id = c.post_id AND cr.reader_type = ?),
			'1970-01-01'::timestamptz
		)`, accountID, readerType)

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count total unread comments", Err: err}
	}
	return count, nil
}
