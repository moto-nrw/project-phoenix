package suggestions

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type suggestionsService struct {
	postRepo        suggestions.PostRepository
	voteRepo        suggestions.VoteRepository
	commentRepo     suggestions.CommentRepository
	commentReadRepo suggestions.CommentReadRepository
}

// NewService creates a new suggestions service
func NewService(postRepo suggestions.PostRepository, voteRepo suggestions.VoteRepository, commentRepo suggestions.CommentRepository, commentReadRepo suggestions.CommentReadRepository) Service {
	return &suggestionsService{
		postRepo:        postRepo,
		voteRepo:        voteRepo,
		commentRepo:     commentRepo,
		commentReadRepo: commentReadRepo,
	}
}

// CreatePost creates a new suggestion post
func (s *suggestionsService) CreatePost(ctx context.Context, post *suggestions.Post) error {
	if post == nil {
		return &InvalidDataError{Err: fmt.Errorf("post cannot be nil")}
	}

	// Force default status for new posts
	post.Status = suggestions.StatusOpen
	post.Score = 0
	post.SetTenantID(tenant.FromContext(ctx))

	if err := post.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	return s.postRepo.Create(ctx, post)
}

// GetPost retrieves a post by ID with author name and vote info
func (s *suggestionsService) GetPost(ctx context.Context, id int64, accountID int64) (*suggestions.Post, error) {
	post, err := s.postRepo.FindByIDWithVote(ctx, id, accountID, suggestions.ReaderTypeUser)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, &PostNotFoundError{PostID: id}
	}
	return post, nil
}

// UpdatePost updates a post. Only the author can update their own posts.
func (s *suggestionsService) UpdatePost(ctx context.Context, post *suggestions.Post, accountID int64) error {
	if post == nil {
		return &InvalidDataError{Err: fmt.Errorf("post cannot be nil")}
	}

	existing, err := s.postRepo.FindByID(ctx, post.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return &PostNotFoundError{PostID: post.ID}
	}

	// Ownership check
	if existing.AuthorID != int64(accountID) {
		return &ForbiddenError{}
	}

	// Only allow updating title and description
	existing.Title = post.Title
	existing.Description = post.Description

	if err := existing.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	return s.postRepo.Update(ctx, existing)
}

// DeletePost deletes a post. Only the author can delete their own posts.
func (s *suggestionsService) DeletePost(ctx context.Context, id int64, accountID int64) error {
	existing, err := s.postRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return &PostNotFoundError{PostID: id}
	}

	// Ownership check
	if existing.AuthorID != int64(accountID) {
		return &ForbiddenError{}
	}

	return s.postRepo.Delete(ctx, id)
}

// ListPosts returns all posts sorted as requested
func (s *suggestionsService) ListPosts(ctx context.Context, accountID int64, sortBy string) ([]*suggestions.Post, error) {
	return s.postRepo.List(ctx, accountID, suggestions.ReaderTypeUser, sortBy, "")
}

// Vote casts or changes a vote on a post, then recalculates score in a transaction
func (s *suggestionsService) Vote(ctx context.Context, postID int64, accountID int64, direction string) (*suggestions.Post, error) {
	if !suggestions.IsValidDirection(direction) {
		return nil, &InvalidDataError{Err: fmt.Errorf("direction must be 'up' or 'down'")}
	}

	// Verify post exists
	existing, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, &PostNotFoundError{PostID: postID}
	}

	vote := &suggestions.Vote{
		PostID:    postID,
		VoterID:   int64(accountID),
		Direction: direction,
	}
	vote.SetTenantID(tenant.FromContext(ctx))

	if err := s.voteRepo.Upsert(ctx, vote); err != nil {
		return nil, err
	}

	if err := s.postRepo.RecalculateScore(ctx, postID); err != nil {
		return nil, err
	}

	return s.postRepo.FindByIDWithVote(ctx, postID, accountID, suggestions.ReaderTypeUser)
}

// RemoveVote removes a user's vote from a post, then recalculates score
func (s *suggestionsService) RemoveVote(ctx context.Context, postID int64, accountID int64) (*suggestions.Post, error) {
	// Verify post exists
	existing, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, &PostNotFoundError{PostID: postID}
	}

	if err := s.voteRepo.DeleteByPostAndVoter(ctx, postID, int64(accountID)); err != nil {
		return nil, err
	}

	if err := s.postRepo.RecalculateScore(ctx, postID); err != nil {
		return nil, err
	}

	return s.postRepo.FindByIDWithVote(ctx, postID, accountID, suggestions.ReaderTypeUser)
}

// CreateComment creates a new user-facing comment on a post
func (s *suggestionsService) CreateComment(ctx context.Context, comment *suggestions.Comment) error {
	if comment == nil {
		return &InvalidDataError{Err: fmt.Errorf("comment cannot be nil")}
	}

	// User-facing comments are always from type "user"
	comment.AuthorType = suggestions.AuthorTypeUser
	comment.SetTenantID(tenant.FromContext(ctx))

	// Verify post exists
	post, err := s.postRepo.FindByID(ctx, comment.PostID)
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: comment.PostID}
	}

	if err := comment.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	return s.commentRepo.Create(ctx, comment)
}

// GetComments retrieves comments for a post
func (s *suggestionsService) GetComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	return s.commentRepo.FindByPostID(ctx, postID)
}

// DeleteComment deletes a user's own comment
func (s *suggestionsService) DeleteComment(ctx context.Context, commentID int64, accountID int64) error {
	comment, err := s.commentRepo.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return &CommentNotFoundError{CommentID: commentID}
	}

	// Users can only delete their own comments
	if comment.AuthorType != suggestions.AuthorTypeUser || comment.AuthorID != accountID {
		return &ForbiddenError{Reason: "you can only delete your own comments"}
	}

	return s.commentRepo.Delete(ctx, commentID)
}

// MarkCommentsRead marks all comments on a post as read for the user
func (s *suggestionsService) MarkCommentsRead(ctx context.Context, postID int64, accountID int64) error {
	// Verify post exists
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: postID}
	}

	return s.commentReadRepo.Upsert(ctx, accountID, postID, suggestions.ReaderTypeUser)
}

// GetTotalUnreadCount returns the total number of unread comments across all posts
func (s *suggestionsService) GetTotalUnreadCount(ctx context.Context, accountID int64) (int, error) {
	return s.commentReadRepo.CountTotalUnread(ctx, accountID, suggestions.ReaderTypeUser)
}
