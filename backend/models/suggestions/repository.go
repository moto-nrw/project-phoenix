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
	// sortBy controls ordering: "score" (default), "newest", "status".
	List(ctx context.Context, accountID int64, sortBy string) ([]*Post, error)

	// FindByIDWithVote returns a single post with author name and current user's vote.
	FindByIDWithVote(ctx context.Context, id int64, accountID int64) (*Post, error)

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
	// If includeInternal is false, internal comments are excluded.
	FindByPostID(ctx context.Context, postID int64, includeInternal bool) ([]*Comment, error)

	// Delete soft-deletes a comment by ID
	Delete(ctx context.Context, id int64) error

	// CountByPostID counts non-deleted comments for a post
	CountByPostID(ctx context.Context, postID int64) (int, error)
}

// CommentReadRepository defines operations for tracking unread comments
type CommentReadRepository interface {
	// Upsert creates or updates the last_read_at timestamp for a user on a post
	Upsert(ctx context.Context, accountID, postID int64) error

	// GetLastReadAt returns when a user last read comments on a post (nil if never)
	GetLastReadAt(ctx context.Context, accountID, postID int64) (*time.Time, error)

	// CountUnreadByPost counts comments on a post created after the user's last read time
	CountUnreadByPost(ctx context.Context, accountID, postID int64) (int, error)

	// CountTotalUnread counts all unread comments across all posts for a user
	CountTotalUnread(ctx context.Context, accountID int64) (int, error)
}

// PostReadRepository defines operations for tracking viewed posts
type PostReadRepository interface {
	// MarkViewed marks a post as viewed by an operator
	MarkViewed(ctx context.Context, accountID, postID int64) error

	// IsViewed checks if an operator has viewed a post
	IsViewed(ctx context.Context, accountID, postID int64) (bool, error)

	// CountUnviewed counts posts that an operator has not yet viewed
	CountUnviewed(ctx context.Context, accountID int64) (int, error)
}
