package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
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

	// Comments
	AddComment(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error
	GetComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error)
	DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error
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

// withAdminTx wraps fn in a BYPASSRLS admin transaction so operator queries
// can access rows across all tenants. If the context already carries a tx
// (e.g. in tests) or no DB is configured, fn runs directly.
func (s *operatorSuggestionsService) withAdminTx(ctx context.Context, fn func(context.Context) error) error {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return fn(ctx)
	}
	if !s.hasDB() {
		return fn(ctx)
	}
	return tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		return fn(adminCtx)
	})
}

// hasDB returns true if s.db is a usable database connection.
// A zero-value &bun.DB{} (used in unit tests) panics on any operation,
// so we detect it by attempting a safe method call.
func (s *operatorSuggestionsService) hasDB() (ok bool) {
	if s.db == nil {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	_ = s.db.DBStats()
	return true
}

// ListAllPosts returns all suggestion posts (for operators).
// Uses WithAdminTx to bypass RLS so operators see posts across all tenants.
func (s *operatorSuggestionsService) ListAllPosts(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
	var posts []*suggestions.Post
	err := s.withAdminTx(ctx, func(ctx context.Context) error {
		var txErr error
		posts, txErr = s.postRepo.List(ctx, operatorAccountID, suggestions.ReaderTypeOperator, sortBy, status)
		return txErr
	})
	return posts, err
}

// GetPost retrieves a single post with its comments (including internal).
// Uses WithAdminTx to bypass RLS so operators can access posts from any tenant.
func (s *operatorSuggestionsService) GetPost(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error) {
	var post *suggestions.Post
	var comments []*suggestions.Comment
	err := s.withAdminTx(ctx, func(ctx context.Context) error {
		var txErr error
		post, txErr = s.postRepo.FindByIDWithVote(ctx, postID, operatorAccountID, suggestions.ReaderTypeOperator)
		if txErr != nil {
			return txErr
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		comments, txErr = s.commentRepo.FindByPostID(ctx, postID)
		return txErr
	})
	if err != nil {
		return nil, nil, err
	}
	return post, comments, nil
}

// MarkCommentsRead marks all comments on a post as read for the operator.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) MarkCommentsRead(ctx context.Context, operatorAccountID, postID int64) error {
	return s.withAdminTx(ctx, func(ctx context.Context) error {
		post, err := s.postRepo.FindByID(ctx, postID)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		// Inject tenant so EnsureTenantID in the repository can set it on the upserted record
		ctx = tenant.WithTenantID(ctx, post.TenantID)
		return s.commentReadRepo.Upsert(ctx, operatorAccountID, postID, suggestions.ReaderTypeOperator)
	})
}

// GetTotalUnreadCount returns the total number of unread comments across all posts.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) GetTotalUnreadCount(ctx context.Context, operatorAccountID int64) (int, error) {
	var count int
	err := s.withAdminTx(ctx, func(ctx context.Context) error {
		var txErr error
		count, txErr = s.commentReadRepo.CountTotalUnread(ctx, operatorAccountID, suggestions.ReaderTypeOperator)
		return txErr
	})
	return count, err
}

// MarkPostViewed marks a post as viewed by the operator.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) MarkPostViewed(ctx context.Context, operatorAccountID, postID int64) error {
	return s.withAdminTx(ctx, func(ctx context.Context) error {
		post, err := s.postRepo.FindByID(ctx, postID)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		// Inject tenant so EnsureTenantID in the repository can set it on the upserted record
		ctx = tenant.WithTenantID(ctx, post.TenantID)
		return s.postReadRepo.MarkViewed(ctx, operatorAccountID, postID, suggestions.ReaderTypeOperator)
	})
}

// GetUnviewedPostCount returns the count of posts the operator hasn't viewed yet.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) GetUnviewedPostCount(ctx context.Context, operatorAccountID int64) (int, error) {
	var count int
	err := s.withAdminTx(ctx, func(ctx context.Context) error {
		var txErr error
		count, txErr = s.postReadRepo.CountUnviewed(ctx, operatorAccountID, suggestions.ReaderTypeOperator)
		return txErr
	})
	return count, err
}

// UpdatePostStatus updates the status of a suggestion post.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) UpdatePostStatus(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error {
	if !suggestions.IsValidStatus(status) {
		return &InvalidDataError{Err: fmt.Errorf("invalid status: %s", status)}
	}

	return s.withAdminTx(ctx, func(ctx context.Context) error {
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

		// Inject tenant so EnsureTenantID can set it on upserted records
		ctx = tenant.WithTenantID(ctx, post.TenantID)

		// Mark post as viewed when operator changes status (they've interacted with it)
		if s.postReadRepo != nil {
			_ = s.postReadRepo.MarkViewed(ctx, operatorID, postID, suggestions.ReaderTypeOperator)
		}

		// Audit log
		changes := map[string]any{
			"old_status": oldStatus,
			"new_status": status,
		}
		s.logAction(ctx, operatorID, platform.ActionStatusChange, platform.ResourceSuggestion, &postID, clientIP, changes)

		return nil
	})
}

// AddComment adds an operator comment to a suggestion.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) AddComment(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error {
	if comment == nil {
		return &InvalidDataError{Err: fmt.Errorf("comment cannot be nil")}
	}

	// Force operator author type
	comment.AuthorType = suggestions.AuthorTypeOperator

	return s.withAdminTx(ctx, func(ctx context.Context) error {
		// Verify post exists
		post, err := s.postRepo.FindByID(ctx, comment.PostID)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: comment.PostID}
		}

		// Inherit tenant_id from the parent post (operator context has no tenant)
		comment.TenantID = post.TenantID

		if err := comment.Validate(); err != nil {
			return &InvalidDataError{Err: err}
		}

		if err := s.commentRepo.Create(ctx, comment); err != nil {
			return err
		}

		// Audit log
		changes := map[string]any{
			"post_id": comment.PostID,
		}
		s.logAction(ctx, comment.AuthorID, platform.ActionAddComment, platform.ResourceComment, &comment.ID, clientIP, changes)

		return nil
	})
}

// GetComments retrieves comments for a post.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) GetComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	var comments []*suggestions.Comment
	err := s.withAdminTx(ctx, func(ctx context.Context) error {
		var txErr error
		comments, txErr = s.commentRepo.FindByPostID(ctx, postID)
		return txErr
	})
	return comments, err
}

// DeleteComment deletes a comment (operators can delete any comment).
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(ctx context.Context) error {
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
	})
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
