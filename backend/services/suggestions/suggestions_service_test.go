package suggestions_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	suggestionsService "github.com/moto-nrw/project-phoenix/services/suggestions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
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
	findByPostIDFn func(ctx context.Context, postID int64) ([]*suggestions.Comment, error)
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

type capturingMailer struct {
	messages chan email.Message
}

func newCapturingMailer(buffer int) *capturingMailer {
	return &capturingMailer{
		messages: make(chan email.Message, buffer),
	}
}

func (m *capturingMailer) Send(message email.Message) error {
	m.messages <- message
	return nil
}

func (m *mockCommentReadRepo) CountTotalUnread(ctx context.Context, accountID int64, readerType string) (int, error) {
	if m.countTotalUnreadFn != nil {
		return m.countTotalUnreadFn(ctx, accountID, readerType)
	}
	return 0, nil
}

// newTestService creates a service with mock repos for unit testing (no email notifications)
func newTestService(postRepo suggestions.PostRepository, voteRepo suggestions.VoteRepository, commentRepo suggestions.CommentRepository, commentReadRepo suggestions.CommentReadRepository) suggestionsService.Service {
	return suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: commentReadRepo,
	})
}

func waitForDispatchedMessage(t *testing.T, mailer *capturingMailer) email.Message {
	t.Helper()

	select {
	case message := <-mailer.messages:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatched email")
		return email.Message{}
	}
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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	err := svc.CreatePost(ctx, nil)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.InvalidDataError{}, err)
}

func TestCreatePost_ValidationError(t *testing.T) {
	ctx := context.Background()

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post := &suggestions.Post{
		Title:       "Test Post",
		Description: "Test Description",
		AuthorID:    123,
	}

	err := svc.CreatePost(ctx, post)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCreatePost_DispatchesNotificationToTrimmedRecipients(t *testing.T) {
	ctx := context.Background()
	mailer := newCapturingMailer(2)
	dispatcher := email.NewDispatcher(mailer, nil)

	postRepo := &mockPostRepo{
		createFn: func(ctx context.Context, post *suggestions.Post) error {
			post.ID = 77
			return nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			require.Equal(t, int64(77), id)
			require.Equal(t, int64(0), accountID)
			require.Equal(t, suggestions.ReaderTypeUser, readerType)
			return &suggestions.Post{
				Model:       base.Model{ID: id},
				Title:       "Test Post",
				Description: strings.Repeat("x", 501),
				AuthorName:  "Alice Example",
			}, nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        &mockVoteRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		Dispatcher:      dispatcher,
		DefaultFrom:     email.NewEmail("Phoenix", "noreply@example.com"),
		NotifyEmail:     " ops1@example.com, ,ops2@example.com ",
		FrontendURL:     "https://frontend.test",
	})

	post := &suggestions.Post{
		Title:       "Test Post",
		Description: "Original description",
		AuthorID:    123,
	}

	require.NoError(t, svc.CreatePost(ctx, post))

	first := waitForDispatchedMessage(t, mailer)
	second := waitForDispatchedMessage(t, mailer)

	recipients := []string{first.To.Address, second.To.Address}
	assert.ElementsMatch(t, []string{"ops1@example.com", "ops2@example.com"}, recipients)

	for _, message := range []email.Message{first, second} {
		assert.Equal(t, "Neuer Vorschlag: Test Post", message.Subject)
		assert.Equal(t, "suggestion-notification.html", message.Template)

		content, ok := message.Content.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "new_post", content["Type"])
		assert.Equal(t, "Alice Example", content["AuthorName"])
		assert.Equal(t, "https://frontend.test/operator/suggestions?post=77", content["SuggestionURL"])
		assert.Equal(t, "https://frontend.test/images/moto_transparent.png", content["LogoURL"])
		assert.Len(t, content["Description"], 503)
		assert.True(t, strings.HasSuffix(content["Description"], "\u2026"))
	}
}

func TestCreatePost_DispatchesNotificationWithInjectedLogger(t *testing.T) {
	ctx := context.Background()
	mailer := newCapturingMailer(1)
	dispatcher := email.NewDispatcher(mailer, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	postRepo := &mockPostRepo{
		createFn: func(ctx context.Context, post *suggestions.Post) error {
			post.ID = 99
			return nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{
				Model:       base.Model{ID: id},
				Title:       "Logger Post",
				Description: "desc",
				AuthorName:  "Alice Example",
			}, nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        &mockVoteRepo{},
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		Dispatcher:      dispatcher,
		DefaultFrom:     email.NewEmail("Phoenix", "noreply@example.com"),
		NotifyEmail:     "ops@example.com",
		FrontendURL:     "https://frontend.test",
		Logger:          logger,
	})

	require.NoError(t, svc.CreatePost(ctx, &suggestions.Post{
		Title:       "Logger Post",
		Description: "desc",
		AuthorID:    123,
	}))

	message := waitForDispatchedMessage(t, mailer)
	assert.Equal(t, "ops@example.com", message.To.Address)
}

func TestCreatePost_IgnoresNotificationLookupFailure(t *testing.T) {
	ctx := context.Background()
	createCalls := 0
	lookupCalls := 0

	postRepo := &mockPostRepo{
		createFn: func(ctx context.Context, post *suggestions.Post) error {
			createCalls++
			post.ID = 55
			return nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			lookupCalls++
			return nil, errors.New("lookup failed")
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	err := svc.CreatePost(ctx, &suggestions.Post{
		Title:       "Test Post",
		Description: "Test Description",
		AuthorID:    123,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, createCalls)
	assert.Equal(t, 1, lookupCalls)
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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post := &suggestions.Post{
		Title:       "New Title",
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestUpdatePost_FindByIDError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("find error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post := &suggestions.Post{
		Title:       "New Title",
		Description: "New Description",
	}
	post.ID = 456

	err := svc.UpdatePost(ctx, post, 123)
	assert.ErrorIs(t, err, expectedErr)
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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	err := svc.DeletePost(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestDeletePost_FindByIDError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("find error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	err := svc.DeletePost(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	posts, err := svc.ListPosts(ctx, 123, "score")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, posts)
}

func TestVote_InvalidDirection(t *testing.T) {
	ctx := context.Background()

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post, err := svc.Vote(ctx, 456, 123, suggestions.DirectionUp)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
	assert.Nil(t, post)
}

func TestVote_Success(t *testing.T) {
	ctx := context.Background()

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectCommit()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			assert.Equal(t, int64(456), id)
			return &suggestions.Post{Model: suggestions.Post{}.Model, AuthorID: 123}, nil
		},
		recalculateScoreFn: func(ctx context.Context, postID int64) error {
			assert.Equal(t, int64(456), postID)
			return nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			assert.Equal(t, suggestions.ReaderTypeUser, readerType)
			return &suggestions.Post{Score: 1}, nil
		},
	}

	voteRepo := &mockVoteRepo{
		upsertFn: func(ctx context.Context, vote *suggestions.Vote) error {
			assert.Equal(t, int64(456), vote.PostID)
			assert.Equal(t, int64(123), vote.VoterID)
			assert.Equal(t, suggestions.DirectionUp, vote.Direction)
			return nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		DB:              bunDB,
	})

	post, err := svc.Vote(ctx, 456, 123, suggestions.DirectionUp)
	require.NoError(t, err)
	require.NotNil(t, post)
	assert.Equal(t, 1, post.Score)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVote_FindByIDError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("find failed")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post, err := svc.Vote(ctx, 456, 123, suggestions.DirectionUp)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
}

func TestVote_UpsertErrorRollsBack(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("upsert failed")

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectRollback()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{AuthorID: 123}, nil
		},
	}

	voteRepo := &mockVoteRepo{
		upsertFn: func(ctx context.Context, vote *suggestions.Vote) error {
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		DB:              bunDB,
	})

	post, err := svc.Vote(ctx, 456, 123, suggestions.DirectionUp)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVote_RecalculateErrorRollsBack(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("recalculate failed")

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectRollback()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{AuthorID: 123}, nil
		},
		recalculateScoreFn: func(ctx context.Context, postID int64) error {
			return expectedErr
		},
	}

	voteRepo := &mockVoteRepo{
		upsertFn: func(ctx context.Context, vote *suggestions.Vote) error {
			return nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		DB:              bunDB,
	})

	post, err := svc.Vote(ctx, 456, 123, suggestions.DirectionUp)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveVote_PostNotFound(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, nil
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post, err := svc.RemoveVote(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
	assert.Nil(t, post)
}

func TestRemoveVote_Success(t *testing.T) {
	ctx := context.Background()

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectCommit()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{AuthorID: 123}, nil
		},
		recalculateScoreFn: func(ctx context.Context, postID int64) error {
			assert.Equal(t, int64(456), postID)
			return nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Score: 0}, nil
		},
	}

	voteRepo := &mockVoteRepo{
		deleteByPostAndVoterFn: func(ctx context.Context, postID, voterID int64) error {
			assert.Equal(t, int64(456), postID)
			assert.Equal(t, int64(123), voterID)
			return nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		DB:              bunDB,
	})

	post, err := svc.RemoveVote(ctx, 456, 123)
	require.NoError(t, err)
	require.NotNil(t, post)
	assert.Equal(t, 0, post.Score)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveVote_FindByIDError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("find failed")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	post, err := svc.RemoveVote(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
}

func TestRemoveVote_DeleteErrorRollsBack(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("delete failed")

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectRollback()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{AuthorID: 123}, nil
		},
	}

	voteRepo := &mockVoteRepo{
		deleteByPostAndVoterFn: func(ctx context.Context, postID, voterID int64) error {
			return expectedErr
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		DB:              bunDB,
	})

	post, err := svc.RemoveVote(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveVote_RecalculateErrorRollsBack(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("recalculate failed")

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectRollback()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{AuthorID: 123}, nil
		},
		recalculateScoreFn: func(ctx context.Context, postID int64) error {
			return expectedErr
		},
	}

	voteRepo := &mockVoteRepo{
		deleteByPostAndVoterFn: func(ctx context.Context, postID, voterID int64) error {
			return nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     &mockCommentRepo{},
		CommentReadRepo: &mockCommentReadRepo{},
		DB:              bunDB,
	})

	post, err := svc.RemoveVote(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, post)
	assert.NoError(t, mock.ExpectationsWereMet())
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
			assert.Equal(t, "Test comment", comment.Content)
			return nil
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.CreateComment(ctx, comment)
	require.NoError(t, err)
	assert.Equal(t, suggestions.AuthorTypeUser, comment.AuthorType)
}

func TestCreateComment_NilComment(t *testing.T) {
	ctx := context.Background()

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	}

	err := svc.CreateComment(ctx, comment)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCreateComment_DispatchesNotificationForCreatedComment(t *testing.T) {
	ctx := context.Background()
	mailer := newCapturingMailer(2)
	dispatcher := email.NewDispatcher(mailer, nil)

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{Model: base.Model{ID: id}}, nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{
				Model:      base.Model{ID: id},
				Title:      "Test Post",
				AuthorName: "Original Poster",
			}, nil
		},
	}

	commentRepo := &mockCommentRepo{
		createFn: func(ctx context.Context, comment *suggestions.Comment) error {
			comment.ID = 88
			return nil
		},
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			return []*suggestions.Comment{
				{Model: base.Model{ID: 77}, AuthorName: "Someone Else", Content: "older"},
				{Model: base.Model{ID: 88}, AuthorName: "Comment Author", Content: strings.Repeat("c", 501)},
			}, nil
		},
	}

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        &mockVoteRepo{},
		CommentRepo:     commentRepo,
		CommentReadRepo: &mockCommentReadRepo{},
		Dispatcher:      dispatcher,
		DefaultFrom:     email.NewEmail("Phoenix", "noreply@example.com"),
		NotifyEmail:     "ops1@example.com, ops2@example.com",
		FrontendURL:     "https://frontend.test",
	})

	comment := &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "new comment",
	}

	require.NoError(t, svc.CreateComment(ctx, comment))

	first := waitForDispatchedMessage(t, mailer)
	second := waitForDispatchedMessage(t, mailer)

	for _, message := range []email.Message{first, second} {
		content, ok := message.Content.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "Neuer Kommentar: Test Post", message.Subject)
		assert.Equal(t, "new_comment", content["Type"])
		assert.Equal(t, "Comment Author", content["AuthorName"])
		assert.Equal(t, "Test Post", content["Title"])
		assert.Equal(t, "https://frontend.test/operator/suggestions?post=456", content["SuggestionURL"])
		assert.Len(t, content["CommentContent"], 503)
		assert.True(t, strings.HasSuffix(content["CommentContent"], "\u2026"))
	}
}

func TestCreateComment_IgnoresNotificationLookupErrors(t *testing.T) {
	ctx := context.Background()
	findCommentsCalls := 0

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{Model: base.Model{ID: id}}, nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			return nil, errors.New("post lookup failed")
		},
	}

	commentRepo := &mockCommentRepo{
		createFn: func(ctx context.Context, comment *suggestions.Comment) error {
			comment.ID = 88
			return nil
		},
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			findCommentsCalls++
			return nil, nil
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

	err := svc.CreateComment(ctx, &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	})

	require.NoError(t, err)
	assert.Equal(t, 0, findCommentsCalls)
}

func TestCreateComment_IgnoresCommentListFailure(t *testing.T) {
	ctx := context.Background()

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return &suggestions.Post{Model: base.Model{ID: id}}, nil
		},
		findByIDWithVoteFn: func(ctx context.Context, id int64, accountID int64, readerType string) (*suggestions.Post, error) {
			return &suggestions.Post{Model: base.Model{ID: id}, Title: "Test Post"}, nil
		},
	}

	commentRepo := &mockCommentRepo{
		createFn: func(ctx context.Context, comment *suggestions.Comment) error {
			comment.ID = 88
			return nil
		},
		findByPostIDFn: func(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
			return nil, errors.New("comment lookup failed")
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

	err := svc.CreateComment(ctx, &suggestions.Comment{
		PostID:   456,
		AuthorID: 123,
		Content:  "Test comment",
	})

	require.NoError(t, err)
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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

	err := svc.DeleteComment(ctx, 789, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.CommentNotFoundError{}, err)
}

func TestDeleteComment_FindByIDError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("find error")

	commentRepo := &mockCommentRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Comment, error) {
			return nil, expectedErr
		},
	}

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

	err := svc.DeleteComment(ctx, 789, 123)
	assert.ErrorIs(t, err, expectedErr)
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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, commentRepo, &mockCommentReadRepo{})

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo)

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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	err := svc.MarkCommentsRead(ctx, 456, 123)
	assert.Error(t, err)
	assert.IsType(t, &suggestionsService.PostNotFoundError{}, err)
}

func TestMarkCommentsRead_FindByIDError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("find error")

	postRepo := &mockPostRepo{
		findByIDFn: func(ctx context.Context, id int64) (*suggestions.Post, error) {
			return nil, expectedErr
		},
	}

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, &mockCommentReadRepo{})

	err := svc.MarkCommentsRead(ctx, 456, 123)
	assert.ErrorIs(t, err, expectedErr)
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

	svc := newTestService(postRepo, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo)

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo)

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

	svc := newTestService(&mockPostRepo{}, &mockVoteRepo{}, &mockCommentRepo{}, commentReadRepo)

	count, err := svc.GetTotalUnreadCount(ctx, 123)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, count)
}
