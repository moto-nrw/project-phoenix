package suggestions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
	suggestionsService "github.com/moto-nrw/project-phoenix/services/suggestions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Mock implementations
type mockPostRepo struct {
	createFn           func(ctx context.Context, post *suggestions.Post) error
	findByIDFn         func(ctx context.Context, id int64) (*suggestions.Post, error)
	findByIDWithVoteFn func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error)
	updateFn           func(ctx context.Context, post *suggestions.Post) error
	deleteFn           func(ctx context.Context, id int64) error
	listFn             func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error)
	recalculateScoreFn func(ctx context.Context, postID int64) error
}

func (m *mockPostRepo) Create(ctx context.Context, post *suggestions.Post) error {
	if m.createFn != nil {
		return m.createFn(ctx, post)
	}
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
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
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
	if m.recalculateScoreFn != nil {
		return m.recalculateScoreFn(ctx, postID)
	}
	return nil
}

type mockVoteRepo struct {
	upsertFn               func(ctx context.Context, vote *suggestions.Vote) error
	deleteByPostAndVoterFn func(ctx context.Context, postID, voterID int64) error
	findByPostAndVoterFn   func(ctx context.Context, postID, voterID int64) (*suggestions.Vote, error)
}

func (m *mockVoteRepo) Upsert(ctx context.Context, vote *suggestions.Vote) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, vote)
	}
	return nil
}

func (m *mockVoteRepo) DeleteByPostAndVoter(ctx context.Context, postID, voterID int64) error {
	if m.deleteByPostAndVoterFn != nil {
		return m.deleteByPostAndVoterFn(ctx, postID, voterID)
	}
	return nil
}

func (m *mockVoteRepo) FindByPostAndVoter(ctx context.Context, postID, voterID int64) (*suggestions.Vote, error) {
	if m.findByPostAndVoterFn != nil {
		return m.findByPostAndVoterFn(ctx, postID, voterID)
	}
	return nil, nil
}

type mockCommentRepo struct {
	createFn       func(ctx context.Context, comment *suggestions.Comment) error
	findByIDFn     func(ctx context.Context, id int64) (*suggestions.Comment, error)
	findByPostIDFn func(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error)
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

func (m *mockCommentRepo) FindByPostID(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error) {
	if m.findByPostIDFn != nil {
		return m.findByPostIDFn(ctx, postID, includeInternal)
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
	upsertFn            func(ctx context.Context, accountID, postID int64, readerType string) error
	getLastReadAtFn     func(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error)
	countUnreadByPostFn func(ctx context.Context, accountID, postID int64, readerType string) (int, error)
	countTotalUnreadFn  func(ctx context.Context, accountID int64, readerType string) (int, error)
}

func (m *mockCommentReadRepo) Upsert(ctx context.Context, accountID, postID int64, readerType string) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, accountID, postID, readerType)
	}
	return nil
}

func (m *mockCommentReadRepo) GetLastReadAt(ctx context.Context, accountID, postID int64, readerType string) (*time.Time, error) {
	if m.getLastReadAtFn != nil {
		return m.getLastReadAtFn(ctx, accountID, postID, readerType)
	}
	return nil, nil
}

func (m *mockCommentReadRepo) CountUnreadByPost(ctx context.Context, accountID, postID int64, readerType string) (int, error) {
	if m.countUnreadByPostFn != nil {
		return m.countUnreadByPostFn(ctx, accountID, postID, readerType)
	}
	return 0, nil
}

func (m *mockCommentReadRepo) CountTotalUnread(ctx context.Context, accountID int64, readerType string) (int, error) {
	if m.countTotalUnreadFn != nil {
		return m.countTotalUnreadFn(ctx, accountID, readerType)
	}
	return 0, nil
}

// Helper to create a test DB for transaction testing
func createTestDB(t *testing.T) *bun.DB {
	// Create an in-memory database for testing
	// This is a minimal setup - in real tests you'd use a proper test database
	return nil // TxHandler will handle nil DB gracefully in tests
}

func TestCreatePost_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		createFn: func(ctx context.Context, post *suggestions.Post) error {
			assert.Equal(t, suggestions.StatusOpen, post.Status)
			assert.Equal(t, 0, post.Score)
			assert.Equal(t, "Test Post", post.Title)
			return nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "Test Post",
		Description: "Test Description",
		AuthorID:    123,
	}

	err := svc.CreatePost(ctx, post)
	require.NoError(t, err)
	assert.Equal(t, suggestions.StatusOpen, post.Status)
	assert.Equal(t, 0, post.Score)
}

func TestCreatePost_NilPost(t *testing.T) {
	ctx := context.Background()

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.CreatePost(ctx, nil)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestCreatePost_ValidationError(t *testing.T) {
	ctx := context.Background()

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "", // Invalid: empty title
		Description: "Test Description",
		AuthorID:    123,
	}

	err := svc.CreatePost(ctx, post)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestCreatePost_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &mockPostRepo{
		createFn: func(ctx context.Context, post *suggestions.Post) error {
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "Test Post",
		Description: "Test Description",
		AuthorID:    123,
	}

	err := svc.CreatePost(ctx, post)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetPost_Success(t *testing.T) {
	ctx := context.Background()
	expectedPost := &suggestions.Post{Title: "Test Post"}

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return expectedPost, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post, err := svc.GetPost(ctx, 456, 123)
	require.NoError(t, err)
	assert.Equal(t, expectedPost, post)
}

func TestGetPost_NotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post, err := svc.GetPost(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
	assert.Nil(t, post)
}

func TestGetPost_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	postRepo := &mockPostRepo{
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return nil, expectedErr
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post, err := svc.GetPost(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
}

func TestUpdatePost_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 123,
				Title:    "Old Title",
				Status:   suggestions.StatusOpen,
			}, nil
		},
		updateFn: func(ctx context.Context, post *suggestions.Post) error {
			assert.Equal(t, "New Title", post.Title)
			assert.Equal(t, "New Description", post.Description)
			return nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "New Title",
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	require.NoError(t, err)
}

func TestUpdatePost_NilPost(t *testing.T) {
	ctx := context.Background()

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.UpdatePost(ctx, nil, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestUpdatePost_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "New Title",
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestUpdatePost_Forbidden(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 999, // Different author
			}, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "New Title",
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.ForbiddenError{}, err)
}

func TestUpdatePost_ValidationError(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 123,
				Status:   suggestions.StatusOpen,
			}, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "", // Invalid: empty title
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestUpdatePost_RepoErrorOnUpdate(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("update error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 123,
				Status:   suggestions.StatusOpen,
			}, nil
		},
		updateFn: func(ctx context.Context, post *suggestions.Post) error {
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post := &suggestions.Post{
		Title:       "New Title",
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	assert.ErrorIs(t, err, expectedErr)
}

func TestDeletePost_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 123,
			}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			assert.Equal(t, int64(456), id)
			return nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeletePost(ctx, 456, 123)
	require.NoError(t, err)
}

func TestDeletePost_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeletePost(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestDeletePost_Forbidden(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 999, // Different author
			}, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeletePost(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.ForbiddenError{}, err)
}

func TestDeletePost_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("delete error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{
				AuthorID: 123,
			}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeletePost(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
}

func TestListPosts_Success(t *testing.T) {
	ctx := context.Background()
	expectedPosts := []*suggestions.Post{{}, {}}

	postRepo := &mockPostRepo{
		listFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			assert.Equal(t, "score", sortBy)
			return expectedPosts, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	posts, err := svc.ListPosts(ctx, 123, "score")
	require.NoError(t, err)
	assert.Equal(t, expectedPosts, posts)
}

func TestListPosts_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("list error")

	postRepo := &mockPostRepo{
		listFn: func(ctx context.Context, accountID int64, readerType string, sortBy string, status string) ([]*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return nil, expectedErr
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	posts, err := svc.ListPosts(ctx, 123, "score")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, posts)
}

func TestVote_Success(t *testing.T) {
	t.Skip("Skipping transaction test - requires real database")
	// Transaction tests require a real database
	// This test would verify Vote() -> Upsert() + RecalculateScore() in transaction
}

func TestVote_InvalidDirection(t *testing.T) {
	ctx := context.Background()

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post, err := svc.Vote(ctx, 456, 123, "invalid")
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
	assert.Nil(t, post)
}

func TestVote_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post, err := svc.Vote(ctx, 456, 123, suggestions.DirectionUp)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
	assert.Nil(t, post)
}

func TestVote_TransactionErrorOnUpsert(t *testing.T) {
	t.Skip("Skipping transaction test - requires real database")
	// Transaction tests require a real database
}

func TestVote_TransactionErrorOnRecalculateScore(t *testing.T) {
	t.Skip("Skipping transaction test - requires real database")
	// Transaction tests require a real database
}

func TestRemoveVote_Success(t *testing.T) {
	t.Skip("Skipping transaction test - requires real database")
	// Transaction tests require a real database
}

func TestRemoveVote_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	post, err := svc.RemoveVote(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
	assert.Nil(t, post)
}

func TestRemoveVote_TransactionErrorOnDelete(t *testing.T) {
	t.Skip("Skipping transaction test - requires real database")
	// Transaction tests require a real database
}

func TestRemoveVote_TransactionErrorOnRecalculateScore(t *testing.T) {
	t.Skip("Skipping transaction test - requires real database")
	// Transaction tests require a real database
}

func TestCreateComment_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentRepo := &mockCommentRepo{
		createFn: func(ctx context.Context, comment *suggestions.Comment) error {
			assert.Equal(t, suggestions.AuthorTypeUser, comment.AuthorType)
			assert.False(t, comment.IsInternal)
			assert.Equal(t, "Test comment", comment.Content)
			return nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.CreateComment(ctx, comment)
	require.NoError(t, err)
	assert.Equal(t, suggestions.AuthorTypeUser, comment.AuthorType)
	assert.False(t, comment.IsInternal)
}

func TestCreateComment_NilComment(t *testing.T) {
	ctx := context.Background()

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.CreateComment(ctx, nil)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestCreateComment_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.CreateComment(ctx, comment)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestCreateComment_ValidationError(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "", // Invalid: empty content
	}

	err := svc.CreateComment(ctx, comment)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestCreateComment_RepoError(t *testing.T) {
	ctx := context.Background()
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

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.CreateComment(ctx, comment)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetComments_Success(t *testing.T) {
	ctx := context.Background()
	expectedComments := []*suggestions.Comment{{Content: "Test"}}

	commentRepo := &mockCommentRepo{
		findByPostIDFn: func(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error) {
			assert.Equal(t, int64(456), postID)
			assert.False(t, includeInternal)
			return expectedComments, nil
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	comments, err := svc.GetComments(ctx, 456)
	require.NoError(t, err)
	assert.Equal(t, expectedComments, comments)
}

func TestGetComments_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("repo error")

	commentRepo := &mockCommentRepo{
		findByPostIDFn: func(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	comments, err := svc.GetComments(ctx, 456)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, comments)
}

func TestDeleteComment_Success(t *testing.T) {
	ctx := context.Background()

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{
				AuthorType: suggestions.AuthorTypeUser,
				AuthorID:   123,
			}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			assert.Equal(t, int64(789), id)
			return nil
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeleteComment(ctx, 789, 123)
	require.NoError(t, err)
}

func TestDeleteComment_CommentNotFound(t *testing.T) {
	ctx := context.Background()

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeleteComment(ctx, 789, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.CommentNotFoundError{}, err)
}

func TestDeleteComment_ForbiddenWrongAuthorType(t *testing.T) {
	ctx := context.Background()

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{
				AuthorType: suggestions.AuthorTypeOperator, // Operator comment
				AuthorID:   123,
			}, nil
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeleteComment(ctx, 789, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.ForbiddenError{}, err)
}

func TestDeleteComment_ForbiddenWrongAuthorID(t *testing.T) {
	ctx := context.Background()

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{
				AuthorType: suggestions.AuthorTypeUser,
				AuthorID:   999, // Different author
			}, nil
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeleteComment(ctx, 789, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.ForbiddenError{}, err)
}

func TestDeleteComment_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("delete error")

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return &suggestions.Comment{
				AuthorType: suggestions.AuthorTypeUser,
				AuthorID:   123,
			}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.DeleteComment(ctx, 789, 123)
	assert.ErrorIs(t, err, expectedErr)
}

func TestMarkCommentsRead_Success(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentReadRepo := &mockCommentReadRepo{
		upsertFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, int64(456), postID)
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo, createTestDB(t))

	err := svc.MarkCommentsRead(ctx, 456, 123)
	require.NoError(t, err)
}

func TestMarkCommentsRead_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{}, createTestDB(t))

	err := svc.MarkCommentsRead(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestMarkCommentsRead_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("upsert error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{}, nil
		},
	}

	commentReadRepo := &mockCommentReadRepo{
		upsertFn: func(ctx context.Context, accountID, postID int64, readerType string) error {
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo, createTestDB(t))

	err := svc.MarkCommentsRead(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetTotalUnreadCount_Success(t *testing.T) {
	ctx := context.Background()

	commentReadRepo := &mockCommentReadRepo{
		countTotalUnreadFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, int64(123), accountID)
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return 42, nil
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo, createTestDB(t))

	count, err := svc.GetTotalUnreadCount(ctx, 123)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestGetTotalUnreadCount_RepoError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("count error")

	commentReadRepo := &mockCommentReadRepo{
		countTotalUnreadFn: func(ctx context.Context, accountID int64, readerType string) (int, error) {
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return 0, expectedErr
		},
	}

	svc := suggestionsService.NewService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo, createTestDB(t))

	count, err := svc.GetTotalUnreadCount(ctx, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, count)
}
