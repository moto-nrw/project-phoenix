package suggestions

import (
	"context"
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
