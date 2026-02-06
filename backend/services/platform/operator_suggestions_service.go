package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

// OperatorSuggestionsService handles operator actions on suggestions
type OperatorSuggestionsService interface {
	// List all suggestions (cross-tenant for operators)
	ListAllPosts(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error)

	// Get a single post with comments
	GetPost(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error)

	// Mark comments as read for the operator
	MarkCommentsRead(ctx context.Context, operatorAccountID, postID int64) error

	// Get total unread comment count for the operator
	GetTotalUnreadCount(ctx context.Context, operatorAccountID int64) (int, error)

	// Mark a post as viewed by the operator
	MarkPostViewed(ctx context.Context, operatorAccountID, postID int64) error

	// Get count of unviewed posts for the operator
	GetUnviewedPostCount(ctx context.Context, operatorAccountID int64) (int, error)

	// Update post status (only operators can change status)
	UpdatePostStatus(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error

	// Comments (operators can see internal, create internal/public, delete any)
	AddComment(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error
	GetComments(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error)
	DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error

	// Get public comments for user-facing display
	GetPublicComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error)
}

type operatorSuggestionsService struct {
	postRepo        suggestions.PostRepository
	commentRepo     suggestions.CommentRepository
	commentReadRepo suggestions.CommentReadRepository
	postReadRepo    suggestions.PostReadRepository
	auditLogRepo    platform.OperatorAuditLogRepository
	db              *bun.DB
	logger          *slog.Logger
}

// OperatorSuggestionsServiceConfig holds configuration for the service
type OperatorSuggestionsServiceConfig struct {
	PostRepo        suggestions.PostRepository
	CommentRepo     suggestions.CommentRepository
	CommentReadRepo suggestions.CommentReadRepository
	PostReadRepo    suggestions.PostReadRepository
	AuditLogRepo    platform.OperatorAuditLogRepository
	DB              *bun.DB
	Logger          *slog.Logger
}

// NewOperatorSuggestionsService creates a new operator suggestions service
func NewOperatorSuggestionsService(cfg OperatorSuggestionsServiceConfig) OperatorSuggestionsService {
	return &operatorSuggestionsService{
		postRepo:        cfg.PostRepo,
		commentRepo:     cfg.CommentRepo,
		commentReadRepo: cfg.CommentReadRepo,
		postReadRepo:    cfg.PostReadRepo,
		auditLogRepo:    cfg.AuditLogRepo,
		db:              cfg.DB,
		logger:          cfg.Logger,
	}
}

func (s *operatorSuggestionsService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// ListAllPosts returns all suggestion posts (for operators)
func (s *operatorSuggestionsService) ListAllPosts(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
	return s.postRepo.List(ctx, operatorAccountID, sortBy)
}

// GetPost retrieves a single post with its comments (including internal)
func (s *operatorSuggestionsService) GetPost(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error) {
	post, err := s.postRepo.FindByIDWithVote(ctx, postID, operatorAccountID)
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

// MarkCommentsRead marks all comments on a post as read for the operator
func (s *operatorSuggestionsService) MarkCommentsRead(ctx context.Context, operatorAccountID, postID int64) error {
	// Verify post exists
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: postID}
	}

	return s.commentReadRepo.Upsert(ctx, operatorAccountID, postID)
}

// GetTotalUnreadCount returns the total number of unread comments across all posts
func (s *operatorSuggestionsService) GetTotalUnreadCount(ctx context.Context, operatorAccountID int64) (int, error) {
	return s.commentReadRepo.CountTotalUnread(ctx, operatorAccountID)
}

// MarkPostViewed marks a post as viewed by the operator
func (s *operatorSuggestionsService) MarkPostViewed(ctx context.Context, operatorAccountID, postID int64) error {
	// Verify post exists
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: postID}
	}

	return s.postReadRepo.MarkViewed(ctx, operatorAccountID, postID)
}

// GetUnviewedPostCount returns the count of posts the operator hasn't viewed yet
func (s *operatorSuggestionsService) GetUnviewedPostCount(ctx context.Context, operatorAccountID int64) (int, error) {
	return s.postReadRepo.CountUnviewed(ctx, operatorAccountID)
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

	// Mark post as viewed when operator changes status (they've interacted with it)
	if s.postReadRepo != nil {
		_ = s.postReadRepo.MarkViewed(ctx, operatorID, postID)
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
func (s *operatorSuggestionsService) AddComment(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error {
	if comment == nil {
		return &InvalidDataError{Err: fmt.Errorf("comment cannot be nil")}
	}

	// Force operator author type
	comment.AuthorType = suggestions.AuthorTypeOperator

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
	s.logAction(ctx, comment.AuthorID, platform.ActionAddComment, platform.ResourceComment, &comment.ID, clientIP, changes)

	return nil
}

// GetComments retrieves comments for a post
func (s *operatorSuggestionsService) GetComments(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error) {
	return s.commentRepo.FindByPostID(ctx, postID, includeInternal)
}

// DeleteComment deletes a comment (operators can delete any comment)
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
func (s *operatorSuggestionsService) GetPublicComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
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
			s.getLogger().Error("failed to set audit log changes",
				"operator_id", operatorID,
				"action", action,
				"error", err,
			)
		}
	}

	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		s.getLogger().Error("failed to create audit log",
			"operator_id", operatorID,
			"action", action,
			"resource_type", resourceType,
			"error", err,
		)
	}
}
