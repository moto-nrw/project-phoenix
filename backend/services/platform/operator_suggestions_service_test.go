package platform_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Mock implementations
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
	t.Parallel()

	ctx := context.Background()
	expectedPosts := []*suggestions.Post{{}, {}}

	postRepo := &testpkg.SuggestionsPostRepoMock{
		ListFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			assert.Equal(t, "score", sortBy)
			assert.Equal(t, "open", status)
			return expectedPosts, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	require.NoError(t, err)
	assert.Equal(t, expectedPosts, posts)
}

func TestListAllPosts_RepoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		ListFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, posts)
}

func TestGetPost_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedPost := &suggestions.Post{Title: "Test Post"}
	expectedComments := []*suggestions.Comment{{Content: "Test Comment"}}

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return expectedPost, nil
		},
	}

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			assert.Equal(t, int64(456), postID)
			return expectedComments, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("comment repo error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			return &suggestions.Post{}, nil
		},
	}

	commentReadRepo := &testpkg.SuggestionsCommentReadRepoMock{
		UpsertFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, int64(456), postID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: commentReadRepo,
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	require.NoError(t, err)
}

func TestMarkCommentsRead_PostNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestMarkCommentsRead_RepoErrorOnFindByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestMarkCommentsRead_RepoErrorOnUpsert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("upsert error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentReadRepo := &testpkg.SuggestionsCommentReadRepoMock{
		UpsertFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: commentReadRepo,
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkCommentsRead(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetTotalUnreadCount_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	commentReadRepo := &testpkg.SuggestionsCommentReadRepoMock{
		CountTotalUnreadFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 42, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	commentReadRepo := &testpkg.SuggestionsCommentReadRepoMock{
		CountTotalUnreadFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 0, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
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
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
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
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	require.NoError(t, err)
}

func TestMarkPostViewed_PostNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestMarkPostViewed_RepoErrorOnFindByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestMarkPostViewed_RepoErrorOnMarkViewed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("mark viewed error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
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
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.MarkPostViewed(ctx, 123, 456)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetUnviewedPostCount_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postReadRepo := &mockPostReadRepo{
		countUnviewedFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 7, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	count, err := svc.GetUnviewedPostCount(ctx, 123)
	require.NoError(t, err)
	assert.Equal(t, 7, count)
}

func TestGetUnviewedPostCount_RepoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postReadRepo := &mockPostReadRepo{
		countUnviewedFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, suggestions.ReaderTypeOperator, readerType)
			return 0, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	count, err := svc.GetUnviewedPostCount(ctx, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, count)
}

func TestUpdatePostStatus_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		UpdateStatusFn: func(ctx context.Context, postID int64, status string) error {
			assert.Equal(t, int64(456), postID)
			assert.Equal(t, suggestions.StatusDone, status)
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
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    auditLogRepo,
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	require.NoError(t, err)
}

func TestUpdatePostStatus_InvalidStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, "invalid_status", 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.InvalidDataError{}, err)
}

func TestUpdatePostStatus_PostNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestUpdatePostStatus_RepoErrorOnUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("update error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		UpdateStatusFn: func(ctx context.Context, postID int64, status string) error {
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUpdatePostStatus_NilPostReadRepo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		UpdateFn: func(ctx context.Context, post *suggestions.Post) error {
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    nil, // Nil postReadRepo
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	require.NoError(t, err)
}

func TestAddComment_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		CreateFn: func(ctx context.Context, comment *suggestions.Comment) error {
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
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.AddComment(ctx, nil, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.InvalidDataError{}, err)
}

func TestAddComment_PostNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("create error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		CreateFn: func(ctx context.Context, comment *suggestions.Comment) error {
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
	t.Parallel()

	ctx := context.Background()
	expectedComments := []*suggestions.Comment{{Content: "Test"}}

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			assert.Equal(t, int64(456), postID)
			return expectedComments, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comments, err := svc.GetComments(ctx, 456)
	require.NoError(t, err)
	assert.Equal(t, expectedComments, comments)
}

func TestGetComments_RepoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("repo error")

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	comments, err := svc.GetComments(ctx, 456)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, comments)
}

func TestDeleteComment_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{PostID: 456}, nil
		},
		DeleteFn: func(ctx context.Context, id int64) error {
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
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	require.NoError(t, err)
}

func TestDeleteComment_CommentNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return nil, nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.CommentNotFoundError{}, err)
}

func TestDeleteComment_RepoErrorOnFindByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("repo error")

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUpdatePostStatus_RepoErrorOnFindByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("find error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUpdatePostStatus_MarkViewedFails_StillSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		UpdateFn: func(ctx context.Context, post *suggestions.Post) error {
			return nil
		},
	}

	postReadRepo := &mockPostReadRepo{
		markViewedFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			return errors.New("mark viewed failed")
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    postReadRepo,
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	// bestEffort catches the markViewed error — overall operation should succeed
	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	require.NoError(t, err)
}

func TestUpdatePostStatus_AuditLogFails_StillSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Status: suggestions.StatusOpen}, nil
		},
		UpdateFn: func(ctx context.Context, post *suggestions.Post) error {
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			return errors.New("audit log failed")
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		Logger:          slog.Default(),
	})

	// bestEffort catches the audit log error — overall operation should succeed
	err := svc.UpdatePostStatus(ctx, 456, suggestions.StatusDone, 123, clientIP)
	require.NoError(t, err)
}

func TestAddComment_RepoErrorOnFindByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("find error")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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

func TestAddComment_AuditLogFails_StillSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		CreateFn: func(ctx context.Context, comment *suggestions.Comment) error {
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			return errors.New("audit log failed")
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
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
}

func TestDeleteComment_AuditLogFails_StillSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{PostID: 456}, nil
		},
		DeleteFn: func(ctx context.Context, id int64) error {
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			return errors.New("audit log failed")
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	require.NoError(t, err)
}

func TestNewOperatorSuggestionsService_NilLogger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		ListFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			return []*suggestions.Post{}, nil
		},
	}

	// Create service with nil logger — getLogger() should fall back to slog.Default()
	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          nil,
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	require.NoError(t, err)
	assert.NotNil(t, posts)
}

func TestHasDB_WithNilDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		ListFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			return []*suggestions.Post{}, nil
		},
	}

	// No DB — withAdminTx should detect hasDB()=false and call fn directly
	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		DB:              nil,
		Logger:          slog.Default(),
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	require.NoError(t, err)
	assert.NotNil(t, posts)
}

func TestHasDB_WithZeroValueDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postRepo := &testpkg.SuggestionsPostRepoMock{
		ListFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			return []*suggestions.Post{}, nil
		},
	}

	// Zero-value bun.DB panics on DBStats() — hasDB() should recover and return false
	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	posts, err := svc.ListAllPosts(ctx, 123, "open", "score")
	require.NoError(t, err)
	assert.NotNil(t, posts)
}

func TestDeleteComment_RepoErrorOnDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("delete error")

	commentRepo := &testpkg.SuggestionsCommentRepoMock{
		FindByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{PostID: 456}, nil
		},
		DeleteFn: func(ctx context.Context, id int64) error {
			return expectedErr
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        &testpkg.SuggestionsPostRepoMock{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeleteComment(ctx, 789, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

// --- HidePost tests ---

func TestHidePost_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	updatedHidden := false
	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{IsHidden: false}, nil
		},
		UpdateHiddenFn: func(ctx context.Context, postID int64, hidden bool) error {
			assert.Equal(t, int64(456), postID)
			updatedHidden = hidden
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			assert.Equal(t, platform.ActionHidePost, entry.Action)
			assert.Equal(t, platform.ResourceSuggestion, entry.ResourceType)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	err := svc.HidePost(ctx, 456, true, 123, clientIP)
	require.NoError(t, err)
	assert.True(t, updatedHidden)
}

func TestHidePost_Unhide(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			assert.Equal(t, platform.ActionUnhidePost, entry.Action)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo: &testpkg.SuggestionsPostRepoMock{
			FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
				return &suggestions.Post{IsHidden: true}, nil
			},
		},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	err := svc.HidePost(ctx, 456, false, 123, clientIP)
	require.NoError(t, err)
}

func TestHidePost_Idempotent_AlreadyHidden(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	updateCalled := false
	auditCalled := false

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo: &testpkg.SuggestionsPostRepoMock{
			FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
				return &suggestions.Post{IsHidden: true}, nil
			},
			UpdateHiddenFn: func(ctx context.Context, postID int64, hidden bool) error {
				updateCalled = true
				return nil
			},
		},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{
			createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
				auditCalled = true
				return nil
			},
		},
		DB:     &bun.DB{},
		Logger: slog.Default(),
	})

	err := svc.HidePost(ctx, 456, true, 123, clientIP)
	require.NoError(t, err)
	assert.False(t, updateCalled, "should not call update when already in desired state")
	assert.False(t, auditCalled, "should not audit log when idempotent no-op")
}

func TestHidePost_PostNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo: &testpkg.SuggestionsPostRepoMock{
			FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
				return nil, nil
			},
		},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.HidePost(ctx, 999, true, 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestHidePost_RepoErrorOnFindByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("db connection failed")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo: &testpkg.SuggestionsPostRepoMock{
			FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
				return nil, expectedErr
			},
		},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.HidePost(ctx, 456, true, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}

// --- DeletePost tests ---

func TestDeletePost_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	deletedID := int64(0)
	postRepo := &testpkg.SuggestionsPostRepoMock{
		FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Title: "Abusive Post"}, nil
		},
		DeleteFn: func(ctx context.Context, id int64) error {
			deletedID = id
			return nil
		},
	}

	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			assert.Equal(t, platform.ActionDeletePost, entry.Action)
			assert.Equal(t, platform.ResourceSuggestion, entry.ResourceType)
			assert.Equal(t, int64(123), entry.OperatorID)
			return nil
		},
	}

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo:        postRepo,
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    auditLogRepo,
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	err := svc.DeletePost(ctx, 456, 123, clientIP)
	require.NoError(t, err)
	assert.Equal(t, int64(456), deletedID)
}

func TestDeletePost_PostNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo: &testpkg.SuggestionsPostRepoMock{
			FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
				return nil, nil
			},
		},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		Logger:          slog.Default(),
	})

	err := svc.DeletePost(ctx, 999, 123, clientIP)
	assert.Error(t, err)
	assert.IsType(t, &platformService.PostNotFoundError{}, err)
}

func TestDeletePost_RepoErrorOnDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientIP := net.ParseIP("192.168.1.1")
	expectedErr := errors.New("delete failed")

	svc := platformService.NewOperatorSuggestionsService(platformService.OperatorSuggestionsServiceConfig{
		PostRepo: &testpkg.SuggestionsPostRepoMock{
			FindByIDFn: func(ctx context.Context, id int64, readerType string) (*suggestions.Post, error) {
				return &suggestions.Post{Title: "Test"}, nil
			},
			DeleteFn: func(ctx context.Context, id int64) error {
				return expectedErr
			},
		},
		CommentRepo:     &testpkg.SuggestionsCommentRepoMock{},
		CommentReadRepo: &testpkg.SuggestionsCommentReadRepoMock{},
		PostReadRepo:    &mockPostReadRepo{},
		AuditLogRepo:    &mockAuditLogRepoShared{},
		DB:              &bun.DB{},
		Logger:          slog.Default(),
	})

	err := svc.DeletePost(ctx, 456, 123, clientIP)
	assert.ErrorIs(t, err, expectedErr)
}
