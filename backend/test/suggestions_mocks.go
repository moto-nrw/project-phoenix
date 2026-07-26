package test

// Func-field mocks for suggestions.PostRepository, suggestions.CommentRepository,
// and suggestions.CommentReadRepository — replacing the copy-pasted
// mockPostRepo/mockCommentRepo/mockCommentReadRepo previously duplicated
// across suggestions service test files. mockVoteRepo and mockPostReadRepo
// are single-site and stay local to their test file.

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
)

// SuggestionsPostRepoMock is a func-field test double for suggestions.PostRepository.
type SuggestionsPostRepoMock struct {
	CreateFn           func(ctx context.Context, post *suggestions.Post) error
	FindByIDFn         func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error)
	FindByIDWithVoteFn func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error)
	UpdateFn           func(ctx context.Context, post *suggestions.Post) error
	UpdateStatusFn     func(ctx context.Context, postID int64, status string) error
	UpdateHiddenFn     func(ctx context.Context, postID int64, hidden bool) error
	DeleteFn           func(ctx context.Context, id int64) error
	ListFn             func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error)
	RecalculateScoreFn func(ctx context.Context, postID int64) error
}

var _ suggestions.PostRepository = (*SuggestionsPostRepoMock)(nil)

func (m *SuggestionsPostRepoMock) Create(ctx context.Context, post *suggestions.Post) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, post)
	}
	return nil
}

func (m *SuggestionsPostRepoMock) FindByID(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id, readerType)
	}
	return &suggestions.Post{}, nil
}

func (m *SuggestionsPostRepoMock) Update(ctx context.Context, post *suggestions.Post) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, post)
	}
	return nil
}

func (m *SuggestionsPostRepoMock) UpdateStatus(ctx context.Context, postID int64, status string) error {
	if m.UpdateStatusFn != nil {
		return m.UpdateStatusFn(ctx, postID, status)
	}
	return nil
}

func (m *SuggestionsPostRepoMock) UpdateHidden(ctx context.Context, postID int64, hidden bool) error {
	if m.UpdateHiddenFn != nil {
		return m.UpdateHiddenFn(ctx, postID, hidden)
	}
	return nil
}

func (m *SuggestionsPostRepoMock) Delete(ctx context.Context, id int64) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *SuggestionsPostRepoMock) List(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, accountID, readerType, sortBy, status)
	}
	return nil, nil
}

func (m *SuggestionsPostRepoMock) FindByIDWithVote(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
	if m.FindByIDWithVoteFn != nil {
		return m.FindByIDWithVoteFn(ctx, id, accountID, readerType)
	}
	return nil, nil
}

func (m *SuggestionsPostRepoMock) RecalculateScore(ctx context.Context, postID int64) error {
	if m.RecalculateScoreFn != nil {
		return m.RecalculateScoreFn(ctx, postID)
	}
	return nil
}

// SuggestionsCommentRepoMock is a func-field test double for suggestions.CommentRepository.
type SuggestionsCommentRepoMock struct {
	CreateFn             func(ctx context.Context, comment *suggestions.Comment) error
	FindByIDFn           func(ctx context.Context, id int64) (*suggestions.Comment, error)
	FindByIDWithAuthorFn func(ctx context.Context, id int64) (*suggestions.Comment, error)
	FindByPostIDFn       func(ctx context.Context, postID int64) ([]*suggestions.Comment, error)
	DeleteFn             func(ctx context.Context, id int64) error
}

var _ suggestions.CommentRepository = (*SuggestionsCommentRepoMock)(nil)

func (m *SuggestionsCommentRepoMock) Create(ctx context.Context, comment *suggestions.Comment) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, comment)
	}
	return nil
}

func (m *SuggestionsCommentRepoMock) FindByID(ctx context.Context, id int64) (*suggestions.Comment, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *SuggestionsCommentRepoMock) FindByIDWithAuthor(ctx context.Context, id int64) (*suggestions.Comment, error) {
	if m.FindByIDWithAuthorFn != nil {
		return m.FindByIDWithAuthorFn(ctx, id)
	}
	return nil, nil
}

func (m *SuggestionsCommentRepoMock) FindByPostID(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	if m.FindByPostIDFn != nil {
		return m.FindByPostIDFn(ctx, postID)
	}
	return nil, nil
}

func (m *SuggestionsCommentRepoMock) Delete(ctx context.Context, id int64) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

// SuggestionsCommentReadRepoMock is a func-field test double for suggestions.CommentReadRepository.
type SuggestionsCommentReadRepoMock struct {
	UpsertFn            func(ctx context.Context, accountID, postID int64, readerType string) error
	GetLastReadAtFn     func(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error)
	CountUnreadByPostFn func(ctx context.Context, accountID, postID int64, readerType string) (int, error)
	CountTotalUnreadFn  func(ctx context.Context, accountID int64, readerType string) (int, error)
}

var _ suggestions.CommentReadRepository = (*SuggestionsCommentReadRepoMock)(nil)

func (m *SuggestionsCommentReadRepoMock) Upsert(ctx context.Context, accountID, postID int64, readerType string) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, accountID, postID, readerType)
	}
	return nil
}

func (m *SuggestionsCommentReadRepoMock) GetLastReadAt(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error) {
	if m.GetLastReadAtFn != nil {
		return m.GetLastReadAtFn(ctx, accountID, postID, readerType)
	}
	return nil, nil
}

func (m *SuggestionsCommentReadRepoMock) CountUnreadByPost(ctx context.Context, accountID, postID int64, readerType string) (int, error) {
	if m.CountUnreadByPostFn != nil {
		return m.CountUnreadByPostFn(ctx, accountID, postID, readerType)
	}
	return 0, nil
}

func (m *SuggestionsCommentReadRepoMock) CountTotalUnread(ctx context.Context, accountID int64, readerType string) (int, error) {
	if m.CountTotalUnreadFn != nil {
		return m.CountTotalUnreadFn(ctx, accountID, readerType)
	}
	return 0, nil
}
