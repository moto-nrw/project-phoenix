package parent

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/uptrace/bun"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	suggestionsModels "github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ErrFeedbackSchoolNotAllowed is returned when the guardian addresses a school
// they have no child at. Handler maps it to 403.
var ErrFeedbackSchoolNotAllowed = errors.New("parent: no child at this school")

// ErrFeedbackNotWired is returned when the suggestions service is missing from
// the wiring. It is a startup fault, not a user error.
var ErrFeedbackNotWired = errors.New("parent: feedback service not wired")

// FeedbackSchool is one entry of the school picker on the feedback board. A
// guardian with children at several schools posts to one board at a time.
type FeedbackSchool struct {
	TenantID   int64  `json:"tenant_id"`
	SchoolName string `json:"school_name"`
}

// ListFeedbackSchools returns the schools whose feedback board the guardian may
// use, derived from their children's schools. Sorted by name so the picker is
// stable.
func (s *service) ListFeedbackSchools(ctx context.Context, accountID int64) ([]*FeedbackSchool, error) {
	children, err := s.feedbackChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(children))
	out := make([]*FeedbackSchool, 0, len(children))
	for _, child := range children {
		if child == nil || seen[child.TenantID] {
			continue
		}
		seen[child.TenantID] = true
		out = append(out, &FeedbackSchool{TenantID: child.TenantID, SchoolName: child.SchoolName})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SchoolName < out[j].SchoolName })
	return out, nil
}

// ListFeedback returns the parent feedback board of one school.
func (s *service) ListFeedback(ctx context.Context, accountID, tenantID int64, sortBy string) ([]*suggestionsModels.Post, error) {
	var out []*suggestionsModels.Post
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		posts, err := s.Suggestions.ListPosts(txCtx, accountID, sortBy)
		out = posts
		return err
	})
	return out, err
}

// CreateFeedback posts new feedback to the product team.
func (s *service) CreateFeedback(ctx context.Context, accountID, tenantID int64, title, description string) (*suggestionsModels.Post, error) {
	post := &suggestionsModels.Post{
		Title:       title,
		Description: description,
		AuthorID:    accountID,
	}
	var out *suggestionsModels.Post
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		if err := s.Suggestions.CreatePost(txCtx, post); err != nil {
			return err
		}
		created, err := s.Suggestions.GetPost(txCtx, post.ID, accountID)
		out = created
		return err
	})
	return out, err
}

// GetFeedback returns a single board entry with vote and comment counts.
func (s *service) GetFeedback(ctx context.Context, accountID, tenantID, postID int64) (*suggestionsModels.Post, error) {
	var out *suggestionsModels.Post
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		post, err := s.Suggestions.GetPost(txCtx, postID, accountID)
		out = post
		return err
	})
	return out, err
}

// UpdateFeedback edits the guardian's own entry. Ownership is enforced by the
// suggestions service.
func (s *service) UpdateFeedback(ctx context.Context, accountID, tenantID, postID int64, title, description string) (*suggestionsModels.Post, error) {
	post := &suggestionsModels.Post{Title: title, Description: description}
	post.ID = postID
	var out *suggestionsModels.Post
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		if err := s.Suggestions.UpdatePost(txCtx, post, accountID); err != nil {
			return err
		}
		updated, err := s.Suggestions.GetPost(txCtx, postID, accountID)
		out = updated
		return err
	})
	return out, err
}

// DeleteFeedback removes the guardian's own entry.
func (s *service) DeleteFeedback(ctx context.Context, accountID, tenantID, postID int64) error {
	return s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		return s.Suggestions.DeletePost(txCtx, postID, accountID)
	})
}

// VoteFeedback casts or changes a vote.
func (s *service) VoteFeedback(ctx context.Context, accountID, tenantID, postID int64, direction string) (*suggestionsModels.Post, error) {
	var out *suggestionsModels.Post
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		post, err := s.Suggestions.Vote(txCtx, postID, accountID, direction)
		out = post
		return err
	})
	return out, err
}

// RemoveFeedbackVote withdraws the guardian's vote.
func (s *service) RemoveFeedbackVote(ctx context.Context, accountID, tenantID, postID int64) (*suggestionsModels.Post, error) {
	var out *suggestionsModels.Post
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		post, err := s.Suggestions.RemoveVote(txCtx, postID, accountID)
		out = post
		return err
	})
	return out, err
}

// ListFeedbackComments returns the thread of one board entry.
func (s *service) ListFeedbackComments(ctx context.Context, accountID, tenantID, postID int64) ([]*suggestionsModels.Comment, error) {
	var out []*suggestionsModels.Comment
	err := s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		comments, err := s.Suggestions.GetComments(txCtx, postID)
		out = comments
		return err
	})
	return out, err
}

// CreateFeedbackComment appends a comment to a thread.
func (s *service) CreateFeedbackComment(ctx context.Context, accountID, tenantID, postID int64, content string) error {
	comment := &suggestionsModels.Comment{
		PostID:   postID,
		AuthorID: accountID,
		Content:  content,
	}
	return s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		return s.Suggestions.CreateComment(txCtx, comment)
	})
}

// DeleteFeedbackComment removes the guardian's own comment.
func (s *service) DeleteFeedbackComment(ctx context.Context, accountID, tenantID, commentID int64) error {
	return s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		return s.Suggestions.DeleteComment(txCtx, commentID, accountID)
	})
}

// MarkFeedbackCommentsRead clears the unread marker of one thread.
func (s *service) MarkFeedbackCommentsRead(ctx context.Context, accountID, tenantID, postID int64) error {
	return s.withFeedbackTx(ctx, accountID, tenantID, func(txCtx context.Context) error {
		return s.Suggestions.MarkCommentsRead(txCtx, postID, accountID)
	})
}

// FeedbackUnreadCount sums unread replies across every school the guardian has
// a board at — the sidebar badge. Cross-tenant.
func (s *service) FeedbackUnreadCount(ctx context.Context, accountID int64) (int, error) {
	if s.Suggestions == nil {
		return 0, ErrFeedbackNotWired
	}
	children, err := s.feedbackChildren(ctx, accountID)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, tenantID := range distinctTenantIDs(children) {
		var count int
		if txErr := tenant.WithParentTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			n, err := s.Suggestions.GetTotalUnreadCount(txCtx, accountID)
			count = n
			return err
		}); txErr != nil {
			return 0, fmt.Errorf("parent: feedback unread count: %w", txErr)
		}
		total += count
	}
	return total, nil
}

// withFeedbackTx validates that the guardian may address this school, then runs
// fn inside a parent-actor transaction. WithParentTx is what makes the RLS
// policy resolve to the parent board — running this work through WithTenantTx
// would target the school's staff board instead, and the database would reject
// the write rather than leak it.
func (s *service) withFeedbackTx(ctx context.Context, accountID, tenantID int64, fn func(ctx context.Context) error) error {
	if s.Suggestions == nil {
		return ErrFeedbackNotWired
	}
	if accountID <= 0 {
		return fmt.Errorf("parent: account_id must be positive")
	}
	if tenantID <= 0 {
		return ErrFeedbackSchoolNotAllowed
	}
	allowed, err := s.feedbackTenantAllowed(ctx, accountID, tenantID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrFeedbackSchoolNotAllowed
	}
	return tenant.WithParentTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// feedbackTenantAllowed reports whether the guardian has a child at the school.
func (s *service) feedbackTenantAllowed(ctx context.Context, accountID, tenantID int64) (bool, error) {
	children, err := s.feedbackChildren(ctx, accountID)
	if err != nil {
		return false, err
	}
	for _, child := range children {
		if child != nil && child.TenantID == tenantID {
			return true, nil
		}
	}
	return false, nil
}

// feedbackChildren loads the guardian's children across all their schools.
func (s *service) feedbackChildren(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve feedback schools: %w", err)
	}
	return children, nil
}
