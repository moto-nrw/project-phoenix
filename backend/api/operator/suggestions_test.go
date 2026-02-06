package operator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
)

// Mock OperatorSuggestionsService
type mockOperatorSuggestionsService struct {
	listAllPostsFn         func(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error)
	getPostFn              func(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error)
	markCommentsReadFn     func(ctx context.Context, operatorAccountID, postID int64) error
	getTotalUnreadCountFn  func(ctx context.Context, operatorAccountID int64) (int, error)
	markPostViewedFn       func(ctx context.Context, operatorAccountID, postID int64) error
	getUnviewedPostCountFn func(ctx context.Context, operatorAccountID int64) (int, error)
	updatePostStatusFn     func(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error
	addCommentFn           func(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error
	deleteCommentFn        func(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error
}

func (m *mockOperatorSuggestionsService) ListAllPosts(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
	if m.listAllPostsFn != nil {
		return m.listAllPostsFn(ctx, operatorAccountID, status, sortBy)
	}
	return nil, nil
}

func (m *mockOperatorSuggestionsService) GetPost(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error) {
	if m.getPostFn != nil {
		return m.getPostFn(ctx, postID, operatorAccountID)
	}
	return nil, nil, nil
}

func (m *mockOperatorSuggestionsService) MarkCommentsRead(ctx context.Context, operatorAccountID, postID int64) error {
	if m.markCommentsReadFn != nil {
		return m.markCommentsReadFn(ctx, operatorAccountID, postID)
	}
	return nil
}

func (m *mockOperatorSuggestionsService) GetTotalUnreadCount(ctx context.Context, operatorAccountID int64) (int, error) {
	if m.getTotalUnreadCountFn != nil {
		return m.getTotalUnreadCountFn(ctx, operatorAccountID)
	}
	return 0, nil
}

func (m *mockOperatorSuggestionsService) MarkPostViewed(ctx context.Context, operatorAccountID, postID int64) error {
	if m.markPostViewedFn != nil {
		return m.markPostViewedFn(ctx, operatorAccountID, postID)
	}
	return nil
}

func (m *mockOperatorSuggestionsService) GetUnviewedPostCount(ctx context.Context, operatorAccountID int64) (int, error) {
	if m.getUnviewedPostCountFn != nil {
		return m.getUnviewedPostCountFn(ctx, operatorAccountID)
	}
	return 0, nil
}

func (m *mockOperatorSuggestionsService) UpdatePostStatus(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error {
	if m.updatePostStatusFn != nil {
		return m.updatePostStatusFn(ctx, postID, status, operatorID, clientIP)
	}
	return nil
}

func (m *mockOperatorSuggestionsService) AddComment(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error {
	if m.addCommentFn != nil {
		return m.addCommentFn(ctx, comment, clientIP)
	}
	return nil
}

func (m *mockOperatorSuggestionsService) DeleteComment(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
	if m.deleteCommentFn != nil {
		return m.deleteCommentFn(ctx, commentID, operatorID, clientIP)
	}
	return nil
}

func (m *mockOperatorSuggestionsService) GetComments(ctx context.Context, postID int64, includeInternal bool) ([]*suggestions.Comment, error) {
	return nil, nil
}

func (m *mockOperatorSuggestionsService) GetPublicComments(ctx context.Context, postID int64) ([]*suggestions.Comment, error) {
	return nil, nil
}

func TestListSuggestions_Success(t *testing.T) {
	now := time.Now()
	mockService := &mockOperatorSuggestionsService{
		listAllPostsFn: func(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
			assert.Equal(t, int64(1), operatorAccountID)
			assert.Equal(t, "", status)
			assert.Equal(t, "created_at", sortBy)
			post := &suggestions.Post{
				Title:        "Test Suggestion",
				Description:  "Test description",
				Status:       suggestions.StatusOpen,
				Score:        5,
				Upvotes:      5,
				Downvotes:    0,
				CommentCount: 2,
				UnreadCount:  1,
				IsNew:        true,
				AuthorName:   "Test User",
			}
			post.ID = 1
			post.CreatedAt = now
			post.UpdatedAt = now
			return []*suggestions.Post{post}, nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListSuggestions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]interface{})
	assert.Len(t, data, 1)
	post := data[0].(map[string]interface{})
	assert.Equal(t, "Test Suggestion", post["title"])
}

func TestListSuggestions_WithStatusFilter(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		listAllPostsFn: func(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
			assert.Equal(t, "open", status)
			return []*suggestions.Post{}, nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions?status=open", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListSuggestions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListSuggestions_WithSortParam(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		listAllPostsFn: func(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
			assert.Equal(t, "score", sortBy)
			return []*suggestions.Post{}, nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions?sort=score", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListSuggestions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListSuggestions_ServiceError(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		listAllPostsFn: func(ctx context.Context, operatorAccountID int64, status string, sortBy string) ([]*suggestions.Post, error) {
			return nil, errors.New("database error")
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListSuggestions(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetSuggestion_Success(t *testing.T) {
	now := time.Now()
	mockService := &mockOperatorSuggestionsService{
		getPostFn: func(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error) {
			assert.Equal(t, int64(1), postID)
			assert.Equal(t, int64(123), operatorAccountID)
			post := &suggestions.Post{
				Title:       "Test",
				Description: "Description",
				Status:      suggestions.StatusOpen,
				Score:       10,
				Upvotes:     10,
				Downvotes:   0,
				UnreadCount: 2,
				AuthorName:  "Author",
			}
			post.ID = 1
			post.CreatedAt = now
			post.UpdatedAt = now
			comment := &suggestions.Comment{
				PostID:     1,
				Content:    "Comment 1",
				AuthorName: "Commenter",
				AuthorType: "user",
				IsInternal: false,
			}
			comment.ID = 1
			comment.CreatedAt = now
			return post, []*suggestions.Comment{comment}, nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetSuggestion(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, "Test", data["title"])
	comments := data["operator_comments"].([]interface{})
	assert.Len(t, comments, 1)
}

func TestGetSuggestion_InvalidID(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{}
	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions/abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetSuggestion(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetSuggestion_NotFound(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		getPostFn: func(ctx context.Context, postID int64, operatorAccountID int64) (*suggestions.Post, []*suggestions.Comment, error) {
			return nil, nil, &platformSvc.PostNotFoundError{}
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetSuggestion(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateStatus_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		updatePostStatusFn: func(ctx context.Context, postID int64, status string, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, int64(1), postID)
			assert.Equal(t, "in-progress", status)
			assert.Equal(t, int64(123), operatorID)
			return nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	body := map[string]string{
		"status": "in-progress",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/suggestions/1/status", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateStatus(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateStatus_EmptyStatus(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{}
	resource := operator.NewSuggestionsResource(mockService)

	body := map[string]string{
		"status": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/suggestions/1/status", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateStatus(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "status is required")
}

func TestAddComment_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		addCommentFn: func(ctx context.Context, comment *suggestions.Comment, clientIP net.IP) error {
			assert.Equal(t, int64(1), comment.PostID)
			assert.Equal(t, int64(123), comment.AuthorID)
			assert.Equal(t, "Test comment", comment.Content)
			assert.True(t, comment.IsInternal)
			return nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	body := map[string]interface{}{
		"content":     "Test comment",
		"is_internal": true,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.AddComment(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestAddComment_EmptyContent(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{}
	resource := operator.NewSuggestionsResource(mockService)

	body := map[string]interface{}{
		"content": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.AddComment(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "content is required")
}

func TestDeleteComment_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		deleteCommentFn: func(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, int64(5), commentID)
			assert.Equal(t, int64(123), operatorID)
			return nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/suggestions/1/comments/5", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("commentId", "5")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteComment(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteComment_InvalidID(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{}
	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/suggestions/1/comments/abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("commentId", "abc")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteComment(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDeleteComment_NotFound(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		deleteCommentFn: func(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
			return &suggestionsSvc.CommentNotFoundError{}
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/suggestions/1/comments/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("commentId", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteComment(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteComment_Forbidden(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		deleteCommentFn: func(ctx context.Context, commentID int64, operatorID int64, clientIP net.IP) error {
			return &suggestionsSvc.ForbiddenError{}
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/suggestions/1/comments/5", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("commentId", "5")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteComment(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMarkCommentsRead_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		markCommentsReadFn: func(ctx context.Context, operatorAccountID, postID int64) error {
			assert.Equal(t, int64(123), operatorAccountID)
			assert.Equal(t, int64(1), postID)
			return nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments/read", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkCommentsRead(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestGetUnreadCount_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		getTotalUnreadCountFn: func(ctx context.Context, operatorAccountID int64) (int, error) {
			assert.Equal(t, int64(123), operatorAccountID)
			return 5, nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions/unread-count", nil)
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnreadCount(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(5), data["unread_count"])
}

func TestMarkPostViewed_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		markPostViewedFn: func(ctx context.Context, operatorAccountID, postID int64) error {
			assert.Equal(t, int64(123), operatorAccountID)
			assert.Equal(t, int64(1), postID)
			return nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/view", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkPostViewed(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestGetUnviewedCount_Success(t *testing.T) {
	mockService := &mockOperatorSuggestionsService{
		getUnviewedPostCountFn: func(ctx context.Context, operatorAccountID int64) (int, error) {
			assert.Equal(t, int64(123), operatorAccountID)
			return 3, nil
		},
	}

	resource := operator.NewSuggestionsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/suggestions/unviewed-count", nil)
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnviewedCount(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["unviewed_count"])
}

func TestUpdateStatusRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/suggestions/1/status", nil)
	updateReq := &operator.UpdateStatusRequest{}

	err := updateReq.Bind(req)
	assert.NoError(t, err)
}

func TestAddCommentRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/suggestions/1/comments", nil)
	addReq := &operator.AddCommentRequest{}

	err := addReq.Bind(req)
	assert.NoError(t, err)
}
