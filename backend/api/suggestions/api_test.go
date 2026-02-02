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

// suggestionsPermissions is the standard set of permissions for test users.
var suggestionsPermissions = []string{
	"suggestions:list",
	"suggestions:read",
	"suggestions:create",
	"suggestions:update",
	"suggestions:delete",
}

// setupRouter creates a chi router with the suggestions resource mounted.
func setupRouter(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()

	// Match the JWT secret used by testpkg.CreateTestJWT
	viper.Set("auth_jwt_secret", "test-jwt-secret-32-chars-minimum")

	db, serviceFactory := testutil.SetupAPITest(t)

	resource := apiSuggestions.NewResource(serviceFactory.Suggestions)
	router := chi.NewRouter()
	router.Mount("/suggestions", resource.Router())

	return db, router
}

// createTestPost inserts a post directly for test setup.
func createTestPost(t *testing.T, db *bun.DB, accountID int64, title, desc string) *suggestions.Post {
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

// cleanupPosts removes test posts and their votes.
func cleanupPosts(t *testing.T, db *bun.DB, postIDs ...int64) {
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

// newAuthRequest creates an HTTP request with a valid JWT bearer token.
func newAuthRequest(t *testing.T, method, target string, body any, accountID int64, perms []string) *http.Request {
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

// exec executes a request against the router and returns the recorder.
func exec(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ============================================================================
// List Posts
// ============================================================================

func TestListPosts_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-list")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("APIList %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	req := newAuthRequest(t, "GET", "/suggestions", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListPosts_WithSortParam(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-list-sort")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "GET", "/suggestions?sort=newest", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListPosts_NoPermission(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-list-noperm")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "GET", "/suggestions", nil, account.ID, []string{}) // no perms
	rr := exec(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ============================================================================
// Get Post
// ============================================================================

func TestGetPost_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-get")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("APIGet %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	req := newAuthRequest(t, "GET", fmt.Sprintf("/suggestions/%d", post.ID), nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetPost_NotFound(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-get-404")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "GET", "/suggestions/999999999", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetPost_InvalidID(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-get-invalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "GET", "/suggestions/abc", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Create Post
// ============================================================================

func TestCreatePost_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-create")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{
		"title":       fmt.Sprintf("API Create %d", time.Now().UnixNano()),
		"description": "Created via API test",
	}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	// Cleanup created post
	var resp map[string]any
	if json.Unmarshal(rr.Body.Bytes(), &resp) == nil {
		if data, ok := resp["data"].(map[string]any); ok {
			if id, ok := data["id"].(float64); ok {
				defer cleanupPosts(t, db, int64(id))
			}
		}
	}
}

func TestCreatePost_EmptyTitle(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-create-empty")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"title": "", "description": "Missing title"}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreatePost_EmptyDescription(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-create-nodesc")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"title": "Has Title", "description": ""}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreatePost_TitleTooLong(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-create-long")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{
		"title":       strings.Repeat("a", 201),
		"description": "Some description",
	}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Update Post
// ============================================================================

func TestUpdatePost_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-update")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("APIUpdate %d", time.Now().UnixNano()), "Original")
	defer cleanupPosts(t, db, post.ID)

	body := map[string]string{"title": "Updated via API", "description": "Updated description"}
	req := newAuthRequest(t, "PUT", fmt.Sprintf("/suggestions/%d", post.ID), body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdatePost_Forbidden(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	author := testpkg.CreateTestAccount(t, db, "api-upd-author")
	other := testpkg.CreateTestAccount(t, db, "api-upd-other")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, other.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("APIForbid %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	body := map[string]string{"title": "Hacked", "description": "Should fail"}
	req := newAuthRequest(t, "PUT", fmt.Sprintf("/suggestions/%d", post.ID), body, other.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestUpdatePost_NotFound(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-upd-404")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"title": "Title", "description": "Desc"}
	req := newAuthRequest(t, "PUT", "/suggestions/999999999", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdatePost_InvalidID(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-upd-invalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"title": "Title", "description": "Desc"}
	req := newAuthRequest(t, "PUT", "/suggestions/abc", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdatePost_DescriptionTooLong(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-upd-longdesc")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("DescLong %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	body := map[string]string{
		"title":       "Valid Title",
		"description": strings.Repeat("a", 5001),
	}
	req := newAuthRequest(t, "PUT", fmt.Sprintf("/suggestions/%d", post.ID), body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Delete Post
// ============================================================================

func TestDeletePost_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-delete")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("APIDelete %d", time.Now().UnixNano()), "Desc")

	req := newAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d", post.ID), nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeletePost_Forbidden(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	author := testpkg.CreateTestAccount(t, db, "api-del-author")
	other := testpkg.CreateTestAccount(t, db, "api-del-other")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, other.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("APIDelForbid %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	req := newAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d", post.ID), nil, other.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDeletePost_NotFound(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-del-404")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "DELETE", "/suggestions/999999999", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeletePost_InvalidID(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-del-invalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "DELETE", "/suggestions/abc", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Vote
// ============================================================================

func TestVote_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-vote")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("APIVote %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	body := map[string]string{"direction": "up"}
	req := newAuthRequest(t, "POST", fmt.Sprintf("/suggestions/%d/vote", post.ID), body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestVote_InvalidDirection(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-vote-invalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"direction": "sideways"}
	req := newAuthRequest(t, "POST", "/suggestions/1/vote", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVote_InvalidID(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-vote-badid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"direction": "up"}
	req := newAuthRequest(t, "POST", "/suggestions/abc/vote", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVote_PostNotFound(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-vote-404")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	body := map[string]string{"direction": "up"}
	req := newAuthRequest(t, "POST", "/suggestions/999999999/vote", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ============================================================================
// Remove Vote
// ============================================================================

func TestRemoveVote_Success(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-rmvote")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("APIRemoveVote %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	// Vote first
	voteBody := map[string]string{"direction": "up"}
	voteReq := newAuthRequest(t, "POST", fmt.Sprintf("/suggestions/%d/vote", post.ID), voteBody, account.ID, suggestionsPermissions)
	voteRR := exec(router, voteReq)
	require.Equal(t, http.StatusOK, voteRR.Code)

	// Remove vote
	req := newAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d/vote", post.ID), nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRemoveVote_PostNotFound(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-rmvote-404")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "DELETE", "/suggestions/999999999/vote", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRemoveVote_InvalidID(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-rmvote-badid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "DELETE", "/suggestions/abc/vote", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Error Renderer coverage
// ============================================================================

func TestGetPost_NegativeID(t *testing.T) {
	db, router := setupRouter(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "api-negid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	req := newAuthRequest(t, "GET", "/suggestions/-1", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
