package suggestions_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	apiSuggestions "github.com/moto-nrw/project-phoenix/api/suggestions"
	"github.com/moto-nrw/project-phoenix/api/testutil"
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

	db, serviceFactory := testutil.SetupAPITest(t)

	resource := apiSuggestions.NewResource(serviceFactory.Suggestions, db)
	router := chi.NewRouter()
	router.Mount("/suggestions", resource.Router())

	return db, router
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
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-list")

	testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("APIList %d", time.Now().UnixNano()), "Desc")

	req := newAuthRequest(t, "GET", "/suggestions", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListPosts_WithSortParam(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-list-sort")

	req := newAuthRequest(t, "GET", "/suggestions?sort=newest", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListPosts_NoPermission(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-list-noperm")

	req := newAuthRequest(t, "GET", "/suggestions", nil, account.ID, []string{}) // no perms
	rr := exec(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ============================================================================
// Get Post
// ============================================================================

func TestGetPost_Success(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-get")

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("APIGet %d", time.Now().UnixNano()), "Desc")

	req := newAuthRequest(t, "GET", fmt.Sprintf("/suggestions/%d", post.ID), nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetPost_NotFound(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-get-404")

	req := newAuthRequest(t, "GET", "/suggestions/999999999", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetPost_InvalidID(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-get-invalid")

	req := newAuthRequest(t, "GET", "/suggestions/abc", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Create Post
// ============================================================================

func TestCreatePost_Success(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-create")

	body := map[string]string{
		"title":       fmt.Sprintf("API Create %d", time.Now().UnixNano()),
		"description": "Created via API test",
	}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

}

func TestCreatePost_EmptyTitle(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-create-empty")

	body := map[string]string{"title": "", "description": "Missing title"}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreatePost_EmptyDescription(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-create-nodesc")

	body := map[string]string{"title": "Has Title", "description": ""}
	req := newAuthRequest(t, "POST", "/suggestions", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreatePost_TitleTooLong(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-create-long")

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
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-update")

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("APIUpdate %d", time.Now().UnixNano()), "Original")

	body := map[string]string{"title": "Updated via API", "description": "Updated description"}
	req := newAuthRequest(t, "PUT", fmt.Sprintf("/suggestions/%d", post.ID), body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdatePost_Forbidden(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	author := testpkg.CreateTestAccount(t, db, "api-upd-author")
	other := testpkg.CreateTestAccount(t, db, "api-upd-other")

	post := testpkg.CreateTestPost(t, db, author.ID, fmt.Sprintf("APIForbid %d", time.Now().UnixNano()), "Desc")

	body := map[string]string{"title": "Hacked", "description": "Should fail"}
	req := newAuthRequest(t, "PUT", fmt.Sprintf("/suggestions/%d", post.ID), body, other.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestUpdatePost_NotFound(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-upd-404")

	body := map[string]string{"title": "Title", "description": "Desc"}
	req := newAuthRequest(t, "PUT", "/suggestions/999999999", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdatePost_InvalidID(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-upd-invalid")

	body := map[string]string{"title": "Title", "description": "Desc"}
	req := newAuthRequest(t, "PUT", "/suggestions/abc", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdatePost_DescriptionTooLong(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-upd-longdesc")

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("DescLong %d", time.Now().UnixNano()), "Desc")

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
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-delete")

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("APIDelete %d", time.Now().UnixNano()), "Desc")

	req := newAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d", post.ID), nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeletePost_Forbidden(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	author := testpkg.CreateTestAccount(t, db, "api-del-author")
	other := testpkg.CreateTestAccount(t, db, "api-del-other")

	post := testpkg.CreateTestPost(t, db, author.ID, fmt.Sprintf("APIDelForbid %d", time.Now().UnixNano()), "Desc")

	req := newAuthRequest(t, "DELETE", fmt.Sprintf("/suggestions/%d", post.ID), nil, other.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDeletePost_NotFound(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-del-404")

	req := newAuthRequest(t, "DELETE", "/suggestions/999999999", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeletePost_InvalidID(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-del-invalid")

	req := newAuthRequest(t, "DELETE", "/suggestions/abc", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Vote
// ============================================================================

func TestVote_Success(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-vote")

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("APIVote %d", time.Now().UnixNano()), "Desc")

	body := map[string]string{"direction": "up"}
	req := newAuthRequest(t, "POST", fmt.Sprintf("/suggestions/%d/vote", post.ID), body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestVote_InvalidDirection(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-vote-invalid")

	body := map[string]string{"direction": "sideways"}
	req := newAuthRequest(t, "POST", "/suggestions/1/vote", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVote_InvalidID(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-vote-badid")

	body := map[string]string{"direction": "up"}
	req := newAuthRequest(t, "POST", "/suggestions/abc/vote", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVote_PostNotFound(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-vote-404")

	body := map[string]string{"direction": "up"}
	req := newAuthRequest(t, "POST", "/suggestions/999999999/vote", body, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ============================================================================
// Remove Vote
// ============================================================================

func TestRemoveVote_Success(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-rmvote")

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("APIRemoveVote %d", time.Now().UnixNano()), "Desc")

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
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-rmvote-404")

	req := newAuthRequest(t, "DELETE", "/suggestions/999999999/vote", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRemoveVote_InvalidID(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-rmvote-badid")

	req := newAuthRequest(t, "DELETE", "/suggestions/abc/vote", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================================
// Error Renderer coverage
// ============================================================================

func TestGetPost_NegativeID(t *testing.T) {
	t.Parallel()
	db, router := setupRouter(t)

	account := testpkg.CreateTestAccount(t, db, "api-negid")

	req := newAuthRequest(t, "GET", "/suggestions/-1", nil, account.ID, suggestionsPermissions)
	rr := exec(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
