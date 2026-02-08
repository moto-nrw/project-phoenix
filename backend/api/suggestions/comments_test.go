package suggestions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	apiSuggestions "github.com/moto-nrw/project-phoenix/api/suggestions"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Test helper functions (duplicated from api_test.go)

func setupCommentTestRouter(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()
	viper.Set("auth_jwt_secret", "test-jwt-secret-32-chars-minimum")
	db, serviceFactory := testutil.SetupAPITest(t)
	resource := apiSuggestions.NewResource(serviceFactory.Suggestions)
	router := chi.NewRouter()
	router.Mount("/suggestions", resource.Router())
	return db, router
}

func createCommentTestPost(t *testing.T, db *bun.DB, accountID int64, title, desc string) *suggestions.Post {
	t.Helper()
	post := &suggestions.Post{
		Title:       title,
		Description: desc,
		AuthorID:    accountID,
		Status:      suggestions.StatusOpen,
		Score:       0,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().
		Model(post).
		ModelTableExpr("suggestions.posts").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err)
	return post
}

func createTestComment(t *testing.T, db *bun.DB, postID, authorID int64, content string) *suggestions.Comment {
	t.Helper()
	comment := &suggestions.Comment{
		PostID:     postID,
		AuthorID:   authorID,
		AuthorType: suggestions.AuthorTypeUser,
		Content:    content,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().
		Model(comment).
		ModelTableExpr("suggestions.comments").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err)
	return comment
}

func cleanupComments(t *testing.T, db *bun.DB, commentIDs ...int64) {
	t.Helper()
	if len(commentIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("suggestions.comments").
		Where("id IN (?)", bun.In(commentIDs)).
		Exec(ctx)
}

func cleanupCommentPosts(t *testing.T, db *bun.DB, postIDs ...int64) {
	t.Helper()
	if len(postIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("suggestions.votes").
		Where("post_id IN (?)", bun.In(postIDs)).
		Exec(ctx)
	_, _ = db.NewDelete().
		TableExpr("suggestions.posts").
		Where("id IN (?)", bun.In(postIDs)).
		Exec(ctx)
}

func newCommentAuthRequest(t *testing.T, method, target string, body any, accountID int64, perms []string) *http.Request {
	t.Helper()
	var reader *bytes.Buffer
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewBuffer(jsonBytes)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	token := testpkg.CreateTestJWT(t, accountID, perms)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func execCommentRequest(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

var commentPerms = []string{
	"suggestions:list",
	"suggestions:read",
	"suggestions:create",
	"suggestions:update",
	"suggestions:delete",
}

// ============================================================================
// List Comments
// ============================================================================

func TestListComments_Success(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comments-list")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createCommentTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
	defer cleanupCommentPosts(t, db, post.ID)

	comment := createTestComment(t, db, post.ID, account.ID, "Test comment")
	defer cleanupComments(t, db, comment.ID)

	req := newCommentAuthRequest(t, "GET", fmt.Sprintf("/suggestions/%d/comments", post.ID), nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]any)
	assert.GreaterOrEqual(t, len(data), 1)
}

func TestListComments_InvalidPostID(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comments-invalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newCommentAuthRequest(t, "GET", "/suggestions/abc/comments", nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListComments_NoPermission(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comments-noperm")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newCommentAuthRequest(t, "GET", "/suggestions/1/comments", nil, account.ID, []string{})
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ============================================================================
// Create Comment
// ============================================================================

func TestCreateComment_Success(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-create")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createCommentTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
	defer cleanupCommentPosts(t, db, post.ID)

	body := map[string]string{
		"content": "This is a new comment",
	}
	req := newCommentAuthRequest(t, "POST", fmt.Sprintf("/suggestions/%d/comments", post.ID), body, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateComment_EmptyContent(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-empty")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{
		"content": "",
	}
	req := newCommentAuthRequest(t, "POST", "/suggestions/1/comments", body, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "content is required")
}

func TestCreateComment_ContentTooLong(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-long")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{
		"content": strings.Repeat("a", 5001),
	}
	req := newCommentAuthRequest(t, "POST", "/suggestions/1/comments", body, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "must not exceed 5000 characters")
}

func TestCreateComment_InvalidPostID(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-invalidpost")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{
		"content": "Comment",
	}
	req := newCommentAuthRequest(t, "POST", "/suggestions/abc/comments", body, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateComment_PostNotFound(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-postnotfound")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{
		"content": "Comment on non-existent post",
	}
	req := newCommentAuthRequest(t, "POST", "/suggestions/999999999/comments", body, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ============================================================================
// Delete Comment
// ============================================================================

func TestDeleteComment_Success(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-delete")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createCommentTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
	defer cleanupCommentPosts(t, db, post.ID)

	comment := createTestComment(t, db, post.ID, account.ID, "To be deleted")

	req := newCommentAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d/comments/%d", post.ID, comment.ID), nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteComment_Forbidden(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	author := testpkg.CreateTestAccount(t, db, "api-comment-author")
	other := testpkg.CreateTestAccount(t, db, "api-comment-other")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, other.ID)

	post := createCommentTestPost(t, db, author.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
	defer cleanupCommentPosts(t, db, post.ID)

	comment := createTestComment(t, db, post.ID, author.ID, "Author's comment")
	defer cleanupComments(t, db, comment.ID)

	// Try to delete as different user
	req := newCommentAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d/comments/%d", post.ID, comment.ID), nil, other.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDeleteComment_InvalidID(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-delinvalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newCommentAuthRequest(t, "DELETE", "/suggestions/1/comments/abc", nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDeleteComment_NotFound(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-del404")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newCommentAuthRequest(t, "DELETE", "/suggestions/1/comments/999999999", nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ============================================================================
// Mark Comments Read
// ============================================================================

func TestMarkCommentsRead_Success(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-markread")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createCommentTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
	defer cleanupCommentPosts(t, db, post.ID)

	comment := createTestComment(t, db, post.ID, account.ID, "Unread comment")
	defer cleanupComments(t, db, comment.ID)

	req := newCommentAuthRequest(t, "POST", fmt.Sprintf("/suggestions/%d/comments/read", post.ID), nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMarkCommentsRead_InvalidPostID(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-readinvalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newCommentAuthRequest(t, "POST", "/suggestions/abc/comments/read", nil, account.ID, commentPerms)
	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Request Binding Tests
// ============================================================================

func TestCreateCommentRequest_Bind_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", nil)
	createReq := &apiSuggestions.CreateCommentRequest{Content: "Valid comment"}
	err := createReq.Bind(req)

	assert.NoError(t, err)
}

func TestCreateCommentRequest_Bind_EmptyContent(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", nil)
	createReq := &apiSuggestions.CreateCommentRequest{Content: ""}
	err := createReq.Bind(req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestCreateCommentRequest_Bind_ContentTooLong(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", nil)
	createReq := &apiSuggestions.CreateCommentRequest{Content: strings.Repeat("a", 5001)}
	err := createReq.Bind(req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not exceed 5000 characters")
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestCreateComment_InvalidJSON(t *testing.T) {
	db, router := setupCommentTestRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-comment-badjson")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	token := testpkg.CreateTestJWT(t, account.ID, commentPerms)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := execCommentRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
