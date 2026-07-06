package platform

import (
	"cmp"
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

	// Moderation
	HidePost(ctx context.Context, postID int64, hidden bool, operatorID int64, clientIP net.IP) error
	DeletePost(ctx context.Context, postID int64, operatorID int64, clientIP net.IP) error
}

type operatorSuggestionsService struct {
	OperatorSuggestionsServiceConfig
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
	return &operatorSuggestionsService{OperatorSuggestionsServiceConfig: cfg}
}

func (s *operatorSuggestionsService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// ListAllPosts returns all suggestion posts (for operators).
// Uses WithAdminTx to bypass RLS so operators see posts across all tenants.
func (s *operatorSuggestionsService) ListAllPosts(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
	var posts []*suggestions.Post
	err := tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		var txErr error
		posts, txErr = s.PostRepo.List(ctx, operatorAccountID, suggestions.ReaderTypeOperator, sortBy, status)
		return txErr
	})
	return posts, err
}

// GetPost retrieves a single post with its comments (including internal).
// Uses WithAdminTx to bypass RLS so operators can access posts from any tenant.
func (s *operatorSuggestionsService) GetPost(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error) {
	var post *suggestions.Post
	var comments []*suggestions.Comment
	err := tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		var txErr error
		post, txErr = s.PostRepo.FindByIDWithVote(ctx, postID, operatorAccountID, suggestions.ReaderTypeOperator)
		if txErr != nil {
			return txErr
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		comments, txErr = s.CommentRepo.FindByPostID(ctx, postID)
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
	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		post, err := s.PostRepo.FindByID(ctx, postID, suggestions.ReaderTypeOperator)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		// Inject tenant so EnsureTenantID in the repository can set it on the upserted record
		ctx = tenant.WithTenantID(ctx, post.TenantID)
		return s.CommentReadRepo.Upsert(ctx, operatorAccountID, postID, suggestions.ReaderTypeOperator)
	})
}

// GetTotalUnreadCount returns the total number of unread comments across all posts.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) GetTotalUnreadCount(ctx context.Context, operatorAccountID int64) (int, error) {
	var count int
	err := tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		var txErr error
		count, txErr = s.CommentReadRepo.CountTotalUnread(ctx, operatorAccountID, suggestions.ReaderTypeOperator)
		return txErr
	})
	return count, err
}

// MarkPostViewed marks a post as viewed by the operator.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) MarkPostViewed(ctx context.Context, operatorAccountID, postID int64) error {
	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		post, err := s.PostRepo.FindByID(ctx, postID, suggestions.ReaderTypeOperator)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		// Inject tenant so EnsureTenantID in the repository can set it on the upserted record
		ctx = tenant.WithTenantID(ctx, post.TenantID)
		return s.PostReadRepo.MarkViewed(ctx, operatorAccountID, postID, suggestions.ReaderTypeOperator)
	})
}

// GetUnviewedPostCount returns the count of posts the operator hasn't viewed yet.
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) GetUnviewedPostCount(ctx context.Context, operatorAccountID int64) (int, error) {
	var count int
	err := tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		var txErr error
		count, txErr = s.PostReadRepo.CountUnviewed(ctx, operatorAccountID, suggestions.ReaderTypeOperator)
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

	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		post, err := s.PostRepo.FindByID(ctx, postID, suggestions.ReaderTypeOperator)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		oldStatus := post.Status
		if err := s.PostRepo.UpdateStatus(ctx, postID, status); err != nil {
			return err
		}

		// Inject tenant so EnsureTenantID can set it on upserted records
		ctx = tenant.WithTenantID(ctx, post.TenantID)

		// Mark post as viewed when operator changes status (they've interacted with it).
		// Wrapped in savepoint so a failure here doesn't roll back the status change.
		if s.PostReadRepo != nil {
			s.bestEffort(ctx, "mark_viewed", func() error {
				return s.PostReadRepo.MarkViewed(ctx, operatorID, postID, suggestions.ReaderTypeOperator)
			})
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

	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		// Verify post exists (operators can comment on hidden posts too)
		post, err := s.PostRepo.FindByID(ctx, comment.PostID, suggestions.ReaderTypeOperator)
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

		if err := s.CommentRepo.Create(ctx, comment); err != nil {
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
	err := tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		var txErr error
		comments, txErr = s.CommentRepo.FindByPostID(ctx, postID)
		return txErr
	})
	return comments, err
}

// DeleteComment deletes a comment (operators can delete any comment).
// Uses WithAdminTx to bypass RLS for cross-tenant access.
func (s *operatorSuggestionsService) DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		comment, err := s.CommentRepo.FindByID(ctx, commentID)
		if err != nil {
			return err
		}
		if comment == nil {
			return &CommentNotFoundError{CommentID: commentID}
		}

		if err := s.CommentRepo.Delete(ctx, commentID); err != nil {
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

// HidePost toggles the visibility of a suggestion post.
// Idempotent: if the post already has the requested visibility, it's a no-op.
func (s *operatorSuggestionsService) HidePost(ctx context.Context, postID int64, hidden bool, operatorID int64, clientIP net.IP) error {
	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		post, err := s.PostRepo.FindByID(ctx, postID, suggestions.ReaderTypeOperator)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		// Idempotent: no-op if already in the requested state
		if post.IsHidden == hidden {
			return nil
		}

		if err := s.PostRepo.UpdateHidden(ctx, postID, hidden); err != nil {
			return err
		}

		// Inject tenant so EnsureTenantID can set it on upserted records
		ctx = tenant.WithTenantID(ctx, post.TenantID)

		// Mark post as viewed when operator hides/unhides (they've interacted with it).
		if s.PostReadRepo != nil {
			s.bestEffort(ctx, "mark_viewed", func() error {
				return s.PostReadRepo.MarkViewed(ctx, operatorID, postID, suggestions.ReaderTypeOperator)
			})
		}

		action := platform.ActionHidePost
		if !hidden {
			action = platform.ActionUnhidePost
		}

		changes := map[string]any{
			"post_id": postID,
			"hidden":  hidden,
		}
		s.logAction(ctx, operatorID, action, platform.ResourceSuggestion, &postID, clientIP, changes)

		return nil
	})
}

// DeletePost permanently removes a suggestion post and all associated data (votes, comments, reads).
// Child tables have ON DELETE CASCADE, so cleanup is automatic.
func (s *operatorSuggestionsService) DeletePost(ctx context.Context, postID int64, operatorID int64, clientIP net.IP) error {
	return tenant.WithAdminTxOrDirect(ctx, s.DB, func(ctx context.Context) error {
		post, err := s.PostRepo.FindByID(ctx, postID, suggestions.ReaderTypeOperator)
		if err != nil {
			return err
		}
		if post == nil {
			return &PostNotFoundError{PostID: postID}
		}

		// Snapshot title before delete — cascade wipes the row
		title := post.Title

		if err := s.PostRepo.Delete(ctx, postID); err != nil {
			return err
		}

		changes := map[string]any{
			"post_id": postID,
			"title":   title,
		}
		s.logAction(ctx, operatorID, platform.ActionDeletePost, platform.ResourceSuggestion, &postID, clientIP, changes)

		return nil
	})
}

// bestEffort runs fn inside a PostgreSQL SAVEPOINT so that if fn fails, the
// surrounding transaction stays healthy. This is used for side-effects that
// should not abort the main business operation (e.g. marking viewed, audit logging).
func (s *operatorSuggestionsService) bestEffort(ctx context.Context, label string, fn func() error) {
	tx, ok := modelBase.TxFromContext(ctx)
	if !ok || tx == nil {
		// No transaction — just run directly; failure is isolated by default.
		if err := fn(); err != nil {
			s.getLogger().Warn("best-effort operation failed (no tx)",
				"operation", label,
				"error", err,
			)
		}
		return
	}

	savepointName := "sp_" + label
	if _, err := (*tx).ExecContext(ctx, "SAVEPOINT "+savepointName); err != nil {
		s.getLogger().Warn("failed to create savepoint for best-effort operation",
			"operation", label,
			"error", err,
		)
		return
	}

	if err := fn(); err != nil {
		s.getLogger().Warn("best-effort operation failed, rolling back savepoint",
			"operation", label,
			"error", err,
		)
		_, _ = (*tx).ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepointName)
		return
	}

	_, _ = (*tx).ExecContext(ctx, "RELEASE SAVEPOINT "+savepointName)
}

// logAction logs an audit entry. Runs inside a savepoint so that audit
// failures do not abort the surrounding business transaction.
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

	s.bestEffort(ctx, "audit_log", func() error {
		return s.AuditLogRepo.Create(ctx, entry)
	})
}
