package suggestions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
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
	postRepo        suggestions.PostRepository
	voteRepo        suggestions.VoteRepository
	commentRepo     suggestions.CommentRepository
	commentReadRepo suggestions.CommentReadRepository
	txHandler       *base.TxHandler
	dispatcher      *email.Dispatcher
	defaultFrom     email.Email
	notifyEmails    []string
	frontendURL     string
	logger          *slog.Logger
}

func (s *suggestionsService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
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
		postRepo:        cfg.PostRepo,
		voteRepo:        cfg.VoteRepo,
		commentRepo:     cfg.CommentRepo,
		commentReadRepo: cfg.CommentReadRepo,
		txHandler:       base.NewTxHandler(cfg.DB),
		dispatcher:      cfg.Dispatcher,
		defaultFrom:     cfg.DefaultFrom,
		notifyEmails:    emails,
		frontendURL:     cfg.FrontendURL,
		logger:          cfg.Logger,
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

	if err := s.postRepo.Create(ctx, post); err != nil {
		return err
	}

	// Fetch post with author name for the notification email
	fullPost, err := s.postRepo.FindByIDWithVote(ctx, post.ID, 0, suggestions.ReaderTypeUser)
	if err == nil && fullPost != nil {
		s.notifyNewPost(fullPost)
	}

	return nil
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

	// Run vote upsert + score recalculation atomically
	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		vote := &suggestions.Vote{
			PostID:    postID,
			VoterID:   int64(accountID),
			Direction: direction,
		}

		if err := s.voteRepo.Upsert(txCtx, vote); err != nil {
			return err
		}

		return s.postRepo.RecalculateScore(txCtx, postID)
	}); err != nil {
		return nil, err
	}

	// Return updated post (outside transaction — read-only)
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

	// Run vote deletion + score recalculation atomically
	if err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.voteRepo.DeleteByPostAndVoter(txCtx, postID, int64(accountID)); err != nil {
			return err
		}

		return s.postRepo.RecalculateScore(txCtx, postID)
	}); err != nil {
		return nil, err
	}

	// Return updated post (outside transaction — read-only)
	return s.postRepo.FindByIDWithVote(ctx, postID, accountID, suggestions.ReaderTypeUser)
}

// CreateComment creates a new user-facing comment on a post
func (s *suggestionsService) CreateComment(ctx context.Context, comment *suggestions.Comment) error {
	if comment == nil {
		return &InvalidDataError{Err: fmt.Errorf("comment cannot be nil")}
	}

	// User-facing comments are always from type "user"
	comment.AuthorType = suggestions.AuthorTypeUser

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

	// Fetch full post for notification (with author name resolved)
	fullPost, fetchErr := s.postRepo.FindByIDWithVote(ctx, comment.PostID, 0, suggestions.ReaderTypeUser)
	if fetchErr == nil && fullPost != nil {
		// FindByPostID resolves author names via joins; find the just-created comment
		comments, commentsErr := s.commentRepo.FindByPostID(ctx, comment.PostID)
		if commentsErr == nil {
			for _, c := range comments {
				if c.ID == comment.ID {
					s.notifyNewComment(fullPost, c)
					break
				}
			}
		}
	}

	return nil
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

// notifyNewPost sends an email notification for a new suggestion post
func (s *suggestionsService) notifyNewPost(post *suggestions.Post) {
	if s.dispatcher == nil || len(s.notifyEmails) == 0 {
		return
	}

	// Truncate description for email preview
	description := post.Description
	if len(description) > 500 {
		description = description[:500] + "…"
	}

	suggestionURL := fmt.Sprintf("%s/operator/suggestions?post=%d", s.frontendURL, post.ID)

	for _, recipient := range s.notifyEmails {
		message := email.Message{
			From:     s.defaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  fmt.Sprintf("Neuer Vorschlag: %s", post.Title),
			Template: "suggestion-notification.html",
			Content: map[string]string{
				"LogoURL":       fmt.Sprintf("%s/images/moto_transparent.png", s.frontendURL),
				"Type":          "new_post",
				"AuthorName":    post.AuthorName,
				"Title":         post.Title,
				"Description":   description,
				"SuggestionURL": suggestionURL,
			},
		}

		s.dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
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
	if s.dispatcher == nil || len(s.notifyEmails) == 0 {
		return
	}

	// Truncate comment for email preview
	content := comment.Content
	if len(content) > 500 {
		content = content[:500] + "…"
	}

	suggestionURL := fmt.Sprintf("%s/operator/suggestions?post=%d", s.frontendURL, post.ID)

	for _, recipient := range s.notifyEmails {
		message := email.Message{
			From:     s.defaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  fmt.Sprintf("Neuer Kommentar: %s", post.Title),
			Template: "suggestion-notification.html",
			Content: map[string]string{
				"LogoURL":        fmt.Sprintf("%s/images/moto_transparent.png", s.frontendURL),
				"Type":           "new_comment",
				"AuthorName":     comment.AuthorName,
				"Title":          post.Title,
				"CommentContent": content,
				"SuggestionURL":  suggestionURL,
			},
		}

		s.dispatcher.Dispatch(context.Background(), email.DeliveryRequest{
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
