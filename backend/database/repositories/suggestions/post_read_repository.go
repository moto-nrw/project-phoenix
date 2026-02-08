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

// PostReadRepository implements suggestions.PostReadRepository
type PostReadRepository struct {
	db *bun.DB
}

// NewPostReadRepository creates a new PostReadRepository
func NewPostReadRepository(db *bun.DB) suggestions.PostReadRepository {
	return &PostReadRepository{db: db}
}

// MarkViewed marks a post as viewed by a reader
func (r *PostReadRepository) MarkViewed(ctx context.Context, accountID, postID int64, readerType string) error {
	pr := &suggestions.PostRead{
		AccountID:  accountID,
		PostID:     postID,
		ReaderType: readerType,
		ViewedAt:   time.Now(),
	}

	_, err := r.db.NewInsert().
		Model(pr).
		On("CONFLICT (account_id, post_id, reader_type) DO UPDATE").
		Set("viewed_at = EXCLUDED.viewed_at").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark post viewed", Err: err}
	}
	return nil
}

// IsViewed checks if a reader has viewed a post
func (r *PostReadRepository) IsViewed(ctx context.Context, accountID, postID int64, readerType string) (bool, error) {
	exists, err := r.db.NewSelect().
		TableExpr("suggestions.post_reads").
		Where("account_id = ?", accountID).
		Where("post_id = ?", postID).
		Where("reader_type = ?", readerType).
		Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check post viewed", Err: err}
	}
	return exists, nil
}

// CountUnviewed counts posts that a reader has not yet viewed
func (r *PostReadRepository) CountUnviewed(ctx context.Context, accountID int64, readerType string) (int, error) {
	count, err := r.db.NewSelect().
		TableExpr("suggestions.posts AS p").
		Where(`NOT EXISTS (
			SELECT 1 FROM suggestions.post_reads pr
			WHERE pr.account_id = ? AND pr.post_id = p.id AND pr.reader_type = ?
		)`, accountID, readerType).
		Count(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, &modelBase.DatabaseError{Op: "count unviewed posts", Err: err}
	}
	return count, nil
}
