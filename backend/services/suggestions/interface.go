package suggestions

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
)

// Service defines the suggestions service operations
type Service interface {
	// Post CRUD
	CreatePost(ctx context.Context, post *suggestions.Post) error
	GetPost(ctx context.Context, id int64, accountID int64) (*suggestions.Post, error)
	UpdatePost(ctx context.Context, post *suggestions.Post, accountID int64) error
	DeletePost(ctx context.Context, id int64, accountID int64) error
	ListPosts(ctx context.Context, accountID int64, sortBy string) ([]*suggestions.Post, error)

	// Voting
	Vote(ctx context.Context, postID int64, accountID int64, direction string) (*suggestions.Post, error)
	RemoveVote(ctx context.Context, postID int64, accountID int64) (*suggestions.Post, error)
}
