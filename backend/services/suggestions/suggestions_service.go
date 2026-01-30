package suggestions

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

type suggestionsService struct {
	postRepo suggestions.PostRepository
	voteRepo suggestions.VoteRepository
	db       *bun.DB
}

// NewService creates a new suggestions service
func NewService(postRepo suggestions.PostRepository, voteRepo suggestions.VoteRepository, db *bun.DB) Service {
	return &suggestionsService{
		postRepo: postRepo,
		voteRepo: voteRepo,
		db:       db,
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

	if err := post.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	return s.postRepo.Create(ctx, post)
}

// GetPost retrieves a post by ID with author name and vote info
func (s *suggestionsService) GetPost(ctx context.Context, id int64, accountID int64) (*suggestions.Post, error) {
	post, err := s.postRepo.FindByIDWithVote(ctx, id, accountID)
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
	return s.postRepo.List(ctx, accountID, sortBy)
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

	// Run vote + score recalculation in a transaction
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	vote := &suggestions.Vote{
		PostID:    postID,
		VoterID:   int64(accountID),
		Direction: direction,
	}

	if err := s.voteRepo.Upsert(ctx, vote); err != nil {
		return nil, err
	}

	if err := s.postRepo.RecalculateScore(ctx, postID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Return updated post
	return s.postRepo.FindByIDWithVote(ctx, postID, accountID)
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

	// Run delete + score recalculation in a transaction
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	if err := s.voteRepo.DeleteByPostAndVoter(ctx, postID, int64(accountID)); err != nil {
		return nil, err
	}

	if err := s.postRepo.RecalculateScore(ctx, postID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Return updated post
	return s.postRepo.FindByIDWithVote(ctx, postID, accountID)
}
