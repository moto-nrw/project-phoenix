package platform

import (
	"context"
	"fmt"
	"net"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

// OperatorSuggestionsService handles operator actions on suggestions
type OperatorSuggestionsService interface {
	// List all suggestions (cross-tenant for operators)
	ListAllPosts(ctx context.Context, status string, sortBy string) ([]*suggestions.Post, error)

	// Get a single post with operator comments
	GetPost(ctx context.Context, postID int64) (*suggestions.Post, []*platform.OperatorComment, error)

	// Update post status (only operators can change status)
	UpdatePostStatus(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error

	// Operator comments
	AddComment(ctx context.Context, comment *platform.OperatorComment, clientIP net.IP) error
	GetComments(ctx context.Context, postID int64, includeInternal bool) ([]*platform.OperatorComment, error)
	DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error

	// Get public comments for user-facing display
	GetPublicComments(ctx context.Context, postID int64) ([]*platform.OperatorComment, error)
}

type operatorSuggestionsService struct {
	postRepo     suggestions.PostRepository
	commentRepo  platform.OperatorCommentRepository
	auditLogRepo platform.OperatorAuditLogRepository
	db           *bun.DB
}

// OperatorSuggestionsServiceConfig holds configuration for the service
type OperatorSuggestionsServiceConfig struct {
	PostRepo     suggestions.PostRepository
	CommentRepo  platform.OperatorCommentRepository
	AuditLogRepo platform.OperatorAuditLogRepository
	DB           *bun.DB
}

// NewOperatorSuggestionsService creates a new operator suggestions service
func NewOperatorSuggestionsService(cfg OperatorSuggestionsServiceConfig) OperatorSuggestionsService {
	return &operatorSuggestionsService{
		postRepo:     cfg.PostRepo,
		commentRepo:  cfg.CommentRepo,
		auditLogRepo: cfg.AuditLogRepo,
		db:           cfg.DB,
	}
}

// ListAllPosts returns all suggestion posts (for operators)
func (s *operatorSuggestionsService) ListAllPosts(ctx context.Context, status string, sortBy string) ([]*suggestions.Post, error) {
	// Use account ID 0 since operators don't have personal votes
	return s.postRepo.List(ctx, 0, sortBy)
}

// GetPost retrieves a single post with its operator comments
func (s *operatorSuggestionsService) GetPost(ctx context.Context, postID int64) (*suggestions.Post, []*platform.OperatorComment, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, nil, err
	}
	if post == nil {
		return nil, nil, &PostNotFoundError{PostID: postID}
	}

	comments, err := s.commentRepo.FindByPostID(ctx, postID, true) // Include internal for operators
	if err != nil {
		return nil, nil, err
	}

	return post, comments, nil
}

// UpdatePostStatus updates the status of a suggestion post
func (s *operatorSuggestionsService) UpdatePostStatus(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error {
	if !suggestions.IsValidStatus(status) {
		return &InvalidDataError{Err: fmt.Errorf("invalid status: %s", status)}
	}

	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: postID}
	}

	oldStatus := post.Status
	post.Status = status

	if err := s.postRepo.Update(ctx, post); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{
		"old_status": oldStatus,
		"new_status": status,
	}
	s.logAction(ctx, operatorID, platform.ActionStatusChange, platform.ResourceSuggestion, &postID, clientIP, changes)

	return nil
}

// AddComment adds an operator comment to a suggestion
func (s *operatorSuggestionsService) AddComment(ctx context.Context, comment *platform.OperatorComment, clientIP net.IP) error {
	if comment == nil {
		return &InvalidDataError{Err: fmt.Errorf("comment cannot be nil")}
	}

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

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{
		"post_id":     comment.PostID,
		"is_internal": comment.IsInternal,
	}
	s.logAction(ctx, comment.OperatorID, platform.ActionAddComment, platform.ResourceComment, &comment.ID, clientIP, changes)

	return nil
}

// GetComments retrieves comments for a post
func (s *operatorSuggestionsService) GetComments(ctx context.Context, postID int64, includeInternal bool) ([]*platform.OperatorComment, error) {
	return s.commentRepo.FindByPostID(ctx, postID, includeInternal)
}

// DeleteComment deletes an operator comment
func (s *operatorSuggestionsService) DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
	comment, err := s.commentRepo.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return &CommentNotFoundError{CommentID: commentID}
	}

	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		return err
	}

	// Audit log
	changes := map[string]any{
		"post_id": comment.PostID,
	}
	s.logAction(ctx, operatorID, platform.ActionDeleteComment, platform.ResourceComment, &commentID, clientIP, changes)

	return nil
}

// GetPublicComments retrieves only public comments for user-facing display
func (s *operatorSuggestionsService) GetPublicComments(ctx context.Context, postID int64) ([]*platform.OperatorComment, error) {
	return s.commentRepo.FindByPostID(ctx, postID, false) // Exclude internal comments
}

// logAction logs an audit entry
func (s *operatorSuggestionsService) logAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) {
	entry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestIP:    clientIP,
	}

	if changes != nil {
		if err := entry.SetChanges(changes); err != nil {
			fmt.Printf("failed to set audit log changes: %v\n", err)
		}
	}

	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}
}
