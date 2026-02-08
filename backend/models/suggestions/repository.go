package suggestions

import (
	"context"
	"time"
)

// PostRepository defines operations for managing suggestion posts
type PostRepository interface {
	Create(ctx context.Context, post *Post) error
	FindByID(ctx context.Context, id int64) (*Post, error)
	Update(ctx context.Context, post *Post) error
	Delete(ctx context.Context, id int64) error

	// List returns all posts with author name and current user's vote.
	// accountID is used to resolve the user_vote column.
	// readerType differentiates "operator" vs "user" in read-tracking subqueries.
	// sortBy controls ordering: "score" (default), "newest", "status".
	// status filters by post status when non-empty.
	List(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*Post, error)

	// FindByIDWithVote returns a single post with author name and current user's vote.
	// readerType differentiates "operator" vs "user" in read-tracking subqueries.
	FindByIDWithVote(ctx context.Context, id int64, accountID int64, readerType string) (*Post, error)

	// RecalculateScore updates the denormalized score on a post from votes.
	RecalculateScore(ctx context.Context, postID int64) error
}

// VoteRepository defines operations for managing suggestion votes
type VoteRepository interface {
	// Upsert creates or updates a vote (ON CONFLICT DO UPDATE).
	Upsert(ctx context.Context, vote *Vote) error

	// DeleteByPostAndVoter removes a vote for a specific post and voter.
	DeleteByPostAndVoter(ctx context.Context, postID, voterID int64) error

	// FindByPostAndVoter returns the vote for a specific post and voter, or nil.
	FindByPostAndVoter(ctx context.Context, postID, voterID int64) (*Vote, error)
}

// CommentRepository defines operations for managing suggestion comments
type CommentRepository interface {
	// Create inserts a new comment
	Create(ctx context.Context, comment *Comment) error

	// FindByID retrieves a comment by ID
	FindByID(ctx context.Context, id int64) (*Comment, error)

	// FindByPostID retrieves all comments for a post with author names resolved.
	FindByPostID(ctx context.Context, postID int64) ([]*Comment, error)

	// Delete soft-deletes a comment by ID
	Delete(ctx context.Context, id int64) error

	// CountByPostID counts non-deleted comments for a post
	CountByPostID(ctx context.Context, postID int64) (int, error)
}

// Reader type constants for namespace isolation between operators and users.
const (
	ReaderTypeUser     = "user"
	ReaderTypeOperator = "operator"
)

// CommentReadRepository defines operations for tracking unread comments
type CommentReadRepository interface {
	// Upsert creates or updates the last_read_at timestamp for a reader on a post.
	// readerType must be ReaderTypeUser or ReaderTypeOperator.
	Upsert(ctx context.Context, accountID, postID int64, readerType string) error

	// GetLastReadAt returns when a reader last read comments on a post (nil if never)
	GetLastReadAt(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error)

	// CountUnreadByPost counts comments on a post created after the reader's last read time
	CountUnreadByPost(ctx context.Context, accountID, postID int64, readerType string) (int, error)

	// CountTotalUnread counts all unread comments across all posts for a reader
	CountTotalUnread(ctx context.Context, accountID int64, readerType string) (int, error)
}

// PostReadRepository defines operations for tracking viewed posts
type PostReadRepository interface {
	// MarkViewed marks a post as viewed by a reader.
	// readerType must be ReaderTypeUser or ReaderTypeOperator.
	MarkViewed(ctx context.Context, accountID, postID int64, readerType string) error

	// IsViewed checks if a reader has viewed a post
	IsViewed(ctx context.Context, accountID, postID int64, readerType string) (bool, error)

	// CountUnviewed counts posts that a reader has not yet viewed
	CountUnviewed(ctx context.Context, accountID int64, readerType string) (int, error)
}
