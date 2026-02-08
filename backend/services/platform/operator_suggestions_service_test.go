package platform_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Mock implementations
type mockPostRepo struct {
	listFn             func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error)
	findByIDWithVoteFn func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error)
	findByIDFn         func(ctx context.Context, id int64) (*suggestions.Post, error)
	updateFn           func(ctx context.Context, post *suggestions.Post) error
}

func (m *mockPostRepo) Create(ctx context.Context, post *suggestions.Post) error {
	return nil
}

func (m *mockPostRepo) FindByID(ctx context.Context, id int64) (*suggestions.Post, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return &suggestions.Post{}, nil
}

func (m *mockPostRepo) Update(ctx context.Context, post *suggestions.Post) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, post)
	}
	return nil
}

func (m *mockPostRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockPostRepo) List(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
	if m.listFn != nil {
		return m.listFn(ctx, accountID, readerType, sortBy, status)
	}
	return nil, nil
}

func (m *mockPostRepo) FindByIDWithVote(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
	if m.findByIDWithVoteFn != nil {
		return m.findByIDWithVoteFn(ctx, id, accountID, readerType)
	}
	return nil, nil
}

func (m *mockPostRepo) RecalculateScore(ctx context.Context, postID int64) error {
	return nil
}

type mockCommentRepo struct {
	findByPostIDFn func(ctx context.Context, postID int64) ([]*suggestions.Comment, error)
	findByIDFn     func(ctx context.Context, id int64) (*suggestions.Comment, error)
	createFn       func(ctx context.Context, comment *suggestions.Comment) error
	deleteFn       func(ctx context.Context, id int64) error
}

func (m *mockCommentRepo) Create(ctx context.Context, comment *suggestions.Comment) error {
	if m.createFn != nil {
		return m.createFn(ctx, comment)
	}
	return nil
}

func (m *mockCommentRepo) FindByID(ctx context.Context, id int64) (*suggestions.Comment, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockCommentRepo) FindByPostID(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	if m.findByPostIDFn != nil {
		return m.findByPostIDFn(ctx, postID)
	}
	return nil, nil
}

func (m *mockCommentRepo) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockCommentRepo) CountByPostID(ctx context.Context, postID int64) (int, error) {
	return 0, nil
}

type mockCommentReadRepo struct {
	upsertFn           func(ctx context.Context, accountID, postID int64, readerType string) error
	countTotalUnreadFn func(ctx context.Context, accountID int64, readerType string) (int, error)
}

func (m *mockCommentReadRepo) Upsert(ctx context.Context, accountID, postID int64, readerType string) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, accountID, postID, readerType)
	}
	return nil
}

func (m *mockCommentReadRepo) GetLastReadAt(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error) {
	return nil, nil
}

func (m *mockCommentReadRepo) CountUnreadByPost(ctx context.Context, accountID, postID int64, readerType string) (int, error) {
	return 0, nil
}

func (m *mockCommentReadRepo) CountTotalUnread(ctx context.Context, accountID int64, readerType string) (int, error) {
	if m.countTotalUnreadFn != nil {
		return m.countTotalUnreadFn(ctx, accountID, readerType)
	}
	return 0, nil
}

type mockPostReadRepo struct {
	markViewedFn    func(ctx context.Context, accountID, postID int64, readerType string) error
	countUnviewedFn func(ctx context.Context, accountID int64, readerType string) (int, error)
}

func (m *mockPostReadRepo) MarkViewed(ctx context.Context, accountID, postID int64, readerType string) error {
	if m.markViewedFn != nil {
		return m.markViewedFn(ctx, accountID, postID, readerType)
	}
	return nil
}

func (m *mockPostReadRepo) IsViewed(ctx context.Context, accountID, postID int64, readerType string) (bool, error) {
	return false, nil
}

func (m *mockPostReadRepo) CountUnviewed(ctx context.Context, accountID int64, readerType string) (int, error) {
	if m.countUnviewedFn != nil {
		return m.countUnviewedFn(ctx, accountID, readerType)
	}
	return 0, nil
}

func TestListAllPosts_Success(t *testing.T) {
	ctx := context.Background()
	expectedPosts := []*suggestions.Post{{}, {}}

	postRepo := &mockPostRepo{
		listFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			assert.Equal(t, "score", sortBy)
			assert.Equal(t, "open", status)
			return expectedPosts, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	require.NoError(t, err)
	assert.Equal(t, expectedPosts, posts)
}

func TestListAllPosts_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &mockPostRepo{
		listFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, posts)
}

func TestGetPost_Success(t *testing.T) {
	ctx := context.Background()
	expectedPost := &suggestions.Post{Title: "Test Post"}
	expectedComments := []*suggestions.Comment{{Content: "Test Comment"}}

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return expectedPost, nil
		},
	}

	commentRepo := &mockCommentRepo{
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			assert.Equal(t, int64(456), postID)
			return expectedComments, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	post, comments, err := svc.GetPost(ctx, 456, 123)
	require.NoError(t, err)
	assert.Equal(t, expectedPost, post)
	assert.Equal(t, expectedComments, comments)
}

func TestGetPost_NotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	post, comments, err := svc.GetPost(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
	assert.Nil(t, post)
	assert.Nil(t, comments)
}

func TestGetPost_RepoErrorOnFindByIDWithVote(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	post, comments, err := svc.GetPost(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
	assert.Nil(t, comments)
}

func TestGetPost_RepoErrorOnFindByPostID(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("comment repo error")

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &mockCommentRepo{
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	post, comments, err := svc.GetPost(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
	assert.Nil(t, comments)
}

func TestMarkCommentsRead_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			return &suggestions.Post{}, nil
		},
	}

	commentReadRepo := &mockCommentReadRepo{
		upsertFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, int64(456), postID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: commentReadRepo,
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	require.NoError(t, err)
}

func TestMarkCommentsRead_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestMarkCommentsRead_RepoErrorOnFindByID(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestMarkCommentsRead_RepoErrorOnUpsert(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("upsert error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentReadRepo := &mockCommentReadRepo{
		upsertFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: commentReadRepo,
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetTotalUnreadCount_Success(t *testing.T) {
	ctx := context.Background()

	commentReadRepo := &mockCommentReadRepo{
		countTotalUnreadFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 42, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: commentReadRepo,
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	count, err := svc.GetTotalUnreadCount(ctx, 123)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestGetTotalUnreadCount_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	commentReadRepo := &mockCommentReadRepo{
		countTotalUnreadFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 0, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: commentReadRepo,
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	count, err := svc.GetTotalUnreadCount(ctx, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, count)
}

func TestMarkPostViewed_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			return &suggestions.Post{}, nil
		},
	}

	postReadRepo := &mockPostReadRepo{
		markViewedFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, int64(456), postID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	require.NoError(t, err)
}

func TestMarkPostViewed_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestMarkPostViewed_RepoErrorOnFindByID(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestMarkPostViewed_RepoErrorOnMarkViewed(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("mark viewed error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	postReadRepo := &mockPostReadRepo{
		markViewedFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetUnviewedPostCount_Success(t *testing.T) {
	ctx := context.Background()

	postReadRepo := &mockPostReadRepo{
		countUnviewedFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 7, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	count, err := svc.GetUnviewedPostCount(ctx, 123)
	require.NoError(t, err)
	assert.Equal(t, 7, count)
}

func TestGetUnviewedPostCount_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postReadRepo := &mockPostReadRepo{
		countUnviewedFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 0, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	count, err := svc.GetUnviewedPostCount(ctx, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, count)
}

func TestUpdatePostStatus_Success(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		updateFn: func(ctx context.Context, post *suggestions.Post) error {
			assert.Equal(t, suggestions.StatusDone, post.Status)
			return nil
		},
	}

	postReadRepo := &mockPostReadRepo{
		markViewedFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			assert.Equal(t, int64(123), entry.OperatorID)
			assert.Equal(t, platform.ActionStatusChange, entry.Action)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    auditLogRepo,
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	require.NoError(t, err)
}

func TestUpdatePostStatus_InvalidStatus(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, "invalid_status", 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.InvalidDataError{}, err)
}

func TestUpdatePostStatus_PostNotFound(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestUpdatePostStatus_RepoErrorOnUpdate(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("update error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		updateFn: func(ctx context.Context, post *suggestions.Post) error {
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUpdatePostStatus_NilPostReadRepo(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		updateFn: func(ctx context.Context, post *suggestions.Post) error {
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    nil, // Nil postReadRepo
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	require.NoError(t, err)
}

func TestAddComment_Success(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &mockCommentRepo{
		createFn: func(ctx context.Context, comment *suggestions.Comment) error {
			assert.Equal(t, suggestions.AuthorTypeOperator, comment.AuthorType)
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			assert.Equal(t, platform.ActionAddComment, entry.Action)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		Logger:          slog.Default(),
	})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.AddComment(ctx, comment, clientIP)
	require.NoError(t, err)
	assert.Equal(t, suggestions.AuthorTypeOperator, comment.AuthorType)
}

func TestAddComment_NilComment(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.AddComment(ctx, nil, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.InvalidDataError{}, err)
}

func TestAddComment_PostNotFound(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.AddComment(ctx, comment, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestAddComment_ValidationError(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "", // Invalid: empty content
	}

	err := svc.AddComment(ctx, comment, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.InvalidDataError{}, err)
}

func TestAddComment_RepoErrorOnCreate(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("create error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &mockCommentRepo{
		createFn: func(ctx context.Context, comment *suggestions.Comment) error {
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.AddComment(ctx, comment, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetComments_Success(t *testing.T) {
	ctx := context.Background()
	expectedComments := []*suggestions.Comment{{Content: "Test"}}

	commentRepo := &mockCommentRepo{
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			assert.Equal(t, int64(456), postID)
			return expectedComments, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comments, err := svc.GetComments(ctx, 456)
	require.NoError(t, err)
	assert.Equal(t, expectedComments, comments)
}

func TestGetComments_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	commentRepo := &mockCommentRepo{
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comments, err := svc.GetComments(ctx, 456)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, comments)
}

func TestDeleteComment_Success(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{PostID: 456}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			assert.Equal(t, int64(789), id)
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			assert.Equal(t, platform.ActionDeleteComment, entry.Action)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	require.NoError(t, err)
}

func TestDeleteComment_CommentNotFound(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.CommentNotFoundError{}, err)
}

func TestDeleteComment_RepoErrorOnFindByID(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("repo error")

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeleteComment_RepoErrorOnDelete(t *testing.T) {
	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("delete error")

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{PostID: 456}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &mockPostRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}
