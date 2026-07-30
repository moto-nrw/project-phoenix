package suggestions

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ServiceConfig holds configuration for the suggestions service
type ServiceConfig struct {
	PostRepo        suggestions.PostRepository
	VoteRepo        suggestions.VoteRepository
	CommentRepo     suggestions.CommentRepository
	CommentReadRepo suggestions.CommentReadRepository
	DB              *bun.DB
	Dispatcher      *email.Dispatcher
	DefaultFrom     email.Email
	NotifyEmail     string
	FrontendURL     string
	Logger          *slog.Logger
}

type suggestionsService struct {
	ServiceConfig
	txHandler    *base.TxHandler
	notifyEmails []string
}

func (s *suggestionsService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// readerTypeFor maps the request's actor to the read-tracking namespace. The
// two boards track "seen" state independently, so a dual-role account (staff at
// a school AND guardian of a child there) never carries read state across.
func readerTypeFor(ctx context.Context) string {
	if tenant.IsParentActor(ctx) {
		return suggestions.ReaderTypeParent
	}
	return suggestions.ReaderTypeUser
}

// postAuthorTypeFor maps the request's actor to the board a new post belongs to.
// RLS rejects a mismatch between this value and app.current_actor_type, so a
// parent post written outside a parent transaction fails loudly instead of
// landing on the school's board.
func postAuthorTypeFor(ctx context.Context) string {
	if tenant.IsParentActor(ctx) {
		return suggestions.PostAuthorParent
	}
	return suggestions.PostAuthorStaff
}

// commentAuthorTypeFor maps the request's actor to the comment author type.
func commentAuthorTypeFor(ctx context.Context) string {
	if tenant.IsParentActor(ctx) {
		return suggestions.AuthorTypeParent
	}
	return suggestions.AuthorTypeUser
}

func notificationContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// truncateRunes caps value to limit runes with an ellipsis suffix. It keeps the
// limit<=0 → "" guard (a zero/negative limit means "no room"); the rune-cutting
// core is strutil.TruncateRunes.
func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return strutil.TruncateRunes(value, limit, "…")
}

// NewService creates a new suggestions service
func NewService(cfg ServiceConfig) Service {
	// Parse comma-separated notify emails, trimming whitespace and filtering empties
	var emails []string
	for _, e := range strings.Split(cfg.NotifyEmail, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			emails = append(emails, e)
		}
	}

	return &suggestionsService{
		ServiceConfig: cfg,
		txHandler:     base.NewTxHandler(cfg.DB),
		notifyEmails:  emails,
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
	post.AuthorType = postAuthorTypeFor(ctx)
	post.SetTenantID(tenant.FromContext(ctx))

	if err := post.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	if err := s.PostRepo.Create(ctx, post); err != nil {
		return err
	}

	// Fetch post with author name for the notification email
	fullPost, err := s.PostRepo.FindByIDWithVote(notificationContext(ctx), post.ID, 0, readerTypeFor(ctx))
	if err == nil && fullPost != nil {
		tenant.RegisterAfterCommit(ctx, func() {
			s.notifyNewPost(fullPost)
		})
	}

	return nil
}

// GetPost retrieves a post by ID with author name and vote info
func (s *suggestionsService) GetPost(ctx context.Context, id int64, accountID int64) (*suggestions.Post, error) {
	post, err := s.PostRepo.FindByIDWithVote(ctx, id, accountID, readerTypeFor(ctx))
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

	existing, err := s.PostRepo.FindByID(ctx, post.ID, readerTypeFor(ctx))
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

	return s.PostRepo.Update(ctx, existing)
}

// DeletePost deletes a post. Only the author can delete their own posts.
func (s *suggestionsService) DeletePost(ctx context.Context, id int64, accountID int64) error {
	existing, err := s.PostRepo.FindByID(ctx, id, readerTypeFor(ctx))
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

	return s.PostRepo.Delete(ctx, id)
}

// ListPosts returns all posts sorted as requested
func (s *suggestionsService) ListPosts(ctx context.Context, accountID int64, sortBy string) ([]*suggestions.Post, error) {
	return s.PostRepo.List(ctx, accountID, readerTypeFor(ctx), sortBy, "")
}

// Vote casts or changes a vote on a post, then recalculates score in a transaction
func (s *suggestionsService) Vote(ctx context.Context, postID int64, accountID int64, direction string) (*suggestions.Post, error) {
	if !suggestions.IsValidDirection(direction) {
		return nil, &InvalidDataError{Err: fmt.Errorf("direction must be 'up' or 'down'")}
	}

	// Verify post exists and is visible to users
	existing, err := s.PostRepo.FindByID(ctx, postID, readerTypeFor(ctx))
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

	if err := s.VoteRepo.Upsert(ctx, vote); err != nil {
		return nil, err
	}

	if err := s.PostRepo.RecalculateScore(ctx, postID); err != nil {
		return nil, err
	}

	return s.PostRepo.FindByIDWithVote(ctx, postID, accountID, readerTypeFor(ctx))
}

// RemoveVote removes a user's vote from a post, then recalculates score
func (s *suggestionsService) RemoveVote(ctx context.Context, postID int64, accountID int64) (*suggestions.Post, error) {
	// Verify post exists and is visible to users
	existing, err := s.PostRepo.FindByID(ctx, postID, readerTypeFor(ctx))
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, &PostNotFoundError{PostID: postID}
	}

	if err := s.VoteRepo.DeleteByPostAndVoter(ctx, postID, int64(accountID)); err != nil {
		return nil, err
	}

	if err := s.PostRepo.RecalculateScore(ctx, postID); err != nil {
		return nil, err
	}

	return s.PostRepo.FindByIDWithVote(ctx, postID, accountID, readerTypeFor(ctx))
}

// CreateComment creates a new user-facing comment on a post
func (s *suggestionsService) CreateComment(ctx context.Context, comment *suggestions.Comment) error {
	if comment == nil {
		return &InvalidDataError{Err: fmt.Errorf("comment cannot be nil")}
	}

	// User-facing comments carry the actor's author type: "user" on the staff
	// board, "parent" on the parent feedback board.
	comment.AuthorType = commentAuthorTypeFor(ctx)
	comment.SetTenantID(tenant.FromContext(ctx))

	// Verify post exists and is visible to users
	post, err := s.PostRepo.FindByID(ctx, comment.PostID, readerTypeFor(ctx))
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: comment.PostID}
	}

	if err := comment.Validate(); err != nil {
		return &InvalidDataError{Err: err}
	}

	if err := s.CommentRepo.Create(ctx, comment); err != nil {
		return err
	}

	notifyCtx := notificationContext(ctx)

	// Fetch full post for notification (with author name resolved)
	fullPost, fetchErr := s.PostRepo.FindByIDWithVote(notifyCtx, comment.PostID, 0, readerTypeFor(ctx))
	if fetchErr == nil && fullPost != nil {
		resolvedComment, commentErr := s.CommentRepo.FindByIDWithAuthor(notifyCtx, comment.ID)
		if commentErr == nil && resolvedComment != nil {
			tenant.RegisterAfterCommit(ctx, func() {
				s.notifyNewComment(fullPost, resolvedComment)
			})
		}
	}

	return nil
}

// GetComments retrieves comments for a post.
// Hidden posts return PostNotFoundError — comments on hidden posts are not accessible to users.
func (s *suggestionsService) GetComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	post, err := s.PostRepo.FindByID(ctx, postID, readerTypeFor(ctx))
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, &PostNotFoundError{PostID: postID}
	}

	return s.CommentRepo.FindByPostID(ctx, postID)
}

// DeleteComment deletes a user's own comment.
// Hidden posts return PostNotFoundError — users cannot interact with hidden posts.
func (s *suggestionsService) DeleteComment(ctx context.Context, commentID int64, accountID int64) error {
	comment, err := s.CommentRepo.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return &CommentNotFoundError{CommentID: commentID}
	}

	// Verify the parent post is visible to users
	post, err := s.PostRepo.FindByID(ctx, comment.PostID, readerTypeFor(ctx))
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: comment.PostID}
	}

	// Users can only delete their own comments, and only on their own board
	if comment.AuthorType != commentAuthorTypeFor(ctx) || comment.AuthorID != accountID {
		return &ForbiddenError{Reason: "you can only delete your own comments"}
	}

	return s.CommentRepo.Delete(ctx, commentID)
}

// MarkCommentsRead marks all comments on a post as read for the user
func (s *suggestionsService) MarkCommentsRead(ctx context.Context, postID int64, accountID int64) error {
	// Verify post exists and is visible to users
	post, err := s.PostRepo.FindByID(ctx, postID, readerTypeFor(ctx))
	if err != nil {
		return err
	}
	if post == nil {
		return &PostNotFoundError{PostID: postID}
	}

	return s.CommentReadRepo.Upsert(ctx, accountID, postID, readerTypeFor(ctx))
}

// GetTotalUnreadCount returns the total number of unread comments across all posts
func (s *suggestionsService) GetTotalUnreadCount(ctx context.Context, accountID int64) (int, error) {
	return s.CommentReadRepo.CountTotalUnread(ctx, accountID, readerTypeFor(ctx))
}

// newPostSubjectPrefix labels the notification by board so the product team can
// tell a school's suggestion from a guardian's feedback at a glance.
func newPostSubjectPrefix(post *suggestions.Post) string {
	if post.IsFromParent() {
		return "Neues Eltern-Feedback"
	}
	return "Neuer Vorschlag"
}

// notifyNewPost sends an email notification for a new suggestion post
func (s *suggestionsService) notifyNewPost(post *suggestions.Post) {
	if s.Dispatcher == nil || len(s.notifyEmails) == 0 {
		return
	}

	// Truncate description for email preview
	description := truncateRunes(post.Description, 500)

	suggestionURL := fmt.Sprintf("%s/operator/suggestions?post=%d", s.FrontendURL, post.ID)

	for _, recipient := range s.notifyEmails {
		message := email.Message{
			From:     s.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  fmt.Sprintf("%s: %s", newPostSubjectPrefix(post), post.Title),
			Template: "suggestion-notification.html",
			Content: map[string]string{
				"LogoURL":       fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.FrontendURL),
				"Type":          "new_post",
				"AuthorName":    post.AuthorName,
				"Title":         post.Title,
				"Description":   description,
				"SuggestionURL": suggestionURL,
			},
		}

		s.Dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
			Message: message,
			Metadata: email.DeliveryMetadata{
				Type:      "suggestion_notification",
				Recipient: recipient,
			},
		})

		s.getLogger().Info("suggestion notification dispatched",
			"type", "new_post",
			"post_id", post.ID,
			"recipient", recipient,
		)
	}
}

// notifyNewComment sends an email notification for a new user comment
func (s *suggestionsService) notifyNewComment(post *suggestions.Post, comment *suggestions.Comment) {
	if s.Dispatcher == nil || len(s.notifyEmails) == 0 {
		return
	}

	// Truncate comment for email preview
	content := truncateRunes(comment.Content, 500)

	suggestionURL := fmt.Sprintf("%s/operator/suggestions?post=%d", s.FrontendURL, post.ID)

	for _, recipient := range s.notifyEmails {
		message := email.Message{
			From:     s.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  fmt.Sprintf("Neuer Kommentar: %s", post.Title),
			Template: "suggestion-notification.html",
			Content: map[string]string{
				"LogoURL":        fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.FrontendURL),
				"Type":           "new_comment",
				"AuthorName":     comment.AuthorName,
				"Title":          post.Title,
				"CommentContent": content,
				"SuggestionURL":  suggestionURL,
			},
		}

		s.Dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
			Message: message,
			Metadata: email.DeliveryMetadata{
				Type:      "suggestion_notification",
				Recipient: recipient,
			},
		})

		s.getLogger().Info("suggestion notification dispatched",
			"type", "new_comment",
			"post_id", post.ID,
			"comment_id", comment.ID,
			"recipient", recipient,
		)
	}
}
