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
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Test constants used in mock assertions (not DB-dependent)
const (
	testAnnouncementID int64 = 1
	testOperatorID     int64 = 1
	testOperatorID123  int64 = 123
)

// Mock AnnouncementService
type mockAnnouncementService struct {
	createFn          func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error
	getAnnouncementFn func(ctx context.Context, id int64) (*platform.Announcement, error)
	updateFn          func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error
	deleteFn          func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error
	listFn            func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error)
	publishFn         func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error
	getStatsFn        func(ctx context.Context, id int64) (*platform.AnnouncementStats, error)
	getViewDetailsFn  func(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error)
}

func (m *mockAnnouncementService) CreateAnnouncement(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
	if m.createFn != nil {
		return m.createFn(ctx, announcement, operatorID, clientIP)
	}
	return nil
}

func (m *mockAnnouncementService) GetAnnouncement(ctx context.Context, id int64) (*platform.Announcement, error) {
	if m.getAnnouncementFn != nil {
		return m.getAnnouncementFn(ctx, id)
	}
	return nil, nil
}

func (m *mockAnnouncementService) UpdateAnnouncement(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, announcement, operatorID, clientIP)
	}
	return nil
}

func (m *mockAnnouncementService) DeleteAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id, operatorID, clientIP)
	}
	return nil
}

func (m *mockAnnouncementService) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
	if m.listFn != nil {
		return m.listFn(ctx, includeInactive)
	}
	return nil, nil
}

func (m *mockAnnouncementService) PublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, id, operatorID, clientIP)
	}
	return nil
}

func (m *mockAnnouncementService) UnpublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	return nil
}

func (m *mockAnnouncementService) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error) {
	return nil, nil
}

func (m *mockAnnouncementService) CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error) {
	return 0, nil
}

func (m *mockAnnouncementService) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	return nil
}

func (m *mockAnnouncementService) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	return nil
}

func (m *mockAnnouncementService) GetStats(ctx context.Context, id int64) (*platform.AnnouncementStats, error) {
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx, id)
	}
	return nil, nil
}

func (m *mockAnnouncementService) GetViewDetails(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error) {
	if m.getViewDetailsFn != nil {
		return m.getViewDetailsFn(ctx, id)
	}
	return nil, nil
}

func TestListAnnouncements_Success(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			assert.False(t, includeInactive)
			announcement := &platform.Announcement{
				Title:       "Test Announcement",
				Content:     "Test content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				TargetRoles: []string{"teacher"},
				CreatedBy:   testOperatorID,
			}
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return []*platform.Announcement{announcement}, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListAnnouncements(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]any)
	assert.Len(t, data, 1)
	announcement := data[0].(map[string]any)
	assert.Equal(t, "Test Announcement", announcement["title"])
}

func TestListAnnouncements_IncludeInactive(t *testing.T) {
	mockService := &mockAnnouncementService{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			assert.True(t, includeInactive)
			return []*platform.Announcement{}, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements?include_inactive=true", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListAnnouncements(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListAnnouncements_ServiceError(t *testing.T) {
	mockService := &mockAnnouncementService{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			return nil, errors.New("database error")
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ListAnnouncements(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAnnouncement_Success(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			assert.Equal(t, testAnnouncementID, id)
			announcement := &platform.Announcement{
				Title:       "Test",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				TargetRoles: []string{},
				CreatedBy:   testOperatorID,
			}
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return announcement, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetAnnouncement(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetAnnouncement_InvalidID(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetAnnouncement_NotFound(t *testing.T) {
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, &platformSvc.AnnouncementNotFoundError{}
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetAnnouncement(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCreateAnnouncement_Success(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, "Test Title", announcement.Title)
			assert.Equal(t, "Test Content", announcement.Content)
			assert.Equal(t, testOperatorID, operatorID)
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":   "Test Title",
		"content": "Test Content",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.CreateAnnouncement(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateAnnouncement_EmptyTitle(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":   "",
		"content": "Test Content",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.CreateAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "title is required")
}

func TestCreateAnnouncement_EmptyContent(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":   "Test Title",
		"content": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.CreateAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "content is required")
}

func TestCreateAnnouncement_InvalidExpiresAt(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	expiresAt := "invalid-date"
	body := map[string]any{
		"title":      "Test Title",
		"content":    "Test Content",
		"expires_at": &expiresAt,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.CreateAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid expires_at format")
}

func TestCreateAnnouncement_WithValidExpiresAt(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.NotNil(t, announcement.ExpiresAt)
			announcement.ID = 1
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	expiresAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	body := map[string]any{
		"title":      "Test Title",
		"content":    "Test Content",
		"expires_at": &expiresAt,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.CreateAnnouncement(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestUpdateAnnouncement_Success(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:       "Old Title",
				Content:     "Old Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				TargetRoles: []string{},
				CreatedBy:   testOperatorID,
			}
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return announcement, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, "New Title", announcement.Title)
			assert.Equal(t, "New Content", announcement.Content)
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":    "New Title",
		"content":  "New Content",
		"type":     platform.TypeAnnouncement,
		"severity": platform.SeverityInfo,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateAnnouncement(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateAnnouncement_NotFound(t *testing.T) {
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, &platformSvc.AnnouncementNotFoundError{}
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":    "Title",
		"content":  "Content",
		"type":     platform.TypeAnnouncement,
		"severity": platform.SeverityInfo,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/announcements/999", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateAnnouncement(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateAnnouncement_InvalidExpiresAt(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:    "Title",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return announcement, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	expiresAt := "invalid-date"
	body := map[string]any{
		"title":      "Title",
		"content":    "Content",
		"type":       platform.TypeAnnouncement,
		"severity":   platform.SeverityInfo,
		"expires_at": &expiresAt,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid expires_at format")
}

func TestDeleteAnnouncement_Success(t *testing.T) {
	mockService := &mockAnnouncementService{
		deleteFn: func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, testAnnouncementID, id)
			assert.Equal(t, testOperatorID123, operatorID)
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/announcements/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteAnnouncement(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteAnnouncement_NotFound(t *testing.T) {
	mockService := &mockAnnouncementService{
		deleteFn: func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
			return &platformSvc.AnnouncementNotFoundError{}
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/announcements/999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteAnnouncement(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPublishAnnouncement_Success(t *testing.T) {
	mockService := &mockAnnouncementService{
		publishFn: func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, testAnnouncementID, id)
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/1/publish", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.PublishAnnouncement(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetStats_Success(t *testing.T) {
	mockService := &mockAnnouncementService{
		getStatsFn: func(ctx context.Context, id int64) (*platform.AnnouncementStats, error) {
			return &platform.AnnouncementStats{
				AnnouncementID: id,
				TargetCount:    100,
				SeenCount:      50,
				DismissedCount: 10,
			}, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/1/stats", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetViewDetails_Success(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getViewDetailsFn: func(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error) {
			return []*platform.AnnouncementViewDetail{
				{
					UserID:    1,
					UserName:  "Test User",
					SeenAt:    now,
					Dismissed: false,
				},
			}, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/1/views", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetViewDetails(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]any)
	assert.Len(t, data, 1)
	detail := data[0].(map[string]any)
	assert.Equal(t, "Test User", detail["user_name"])
	assert.Equal(t, false, detail["dismissed"])
}

func TestCreateAnnouncementRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/announcements", nil)
	createReq := &operator.CreateAnnouncementRequest{}

	err := createReq.Bind(req)
	assert.NoError(t, err)
}

func TestUpdateAnnouncementRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", nil)
	updateReq := &operator.UpdateAnnouncementRequest{}

	err := updateReq.Bind(req)
	assert.NoError(t, err)
}

// --- Shared helpers for ID-based handler tests ---

// idHandlerFunc is a handler method that takes (http.ResponseWriter, *http.Request) with an "id" URL param.
type idHandlerFunc func(http.ResponseWriter, *http.Request)

// callIDHandler invokes a handler that reads chi URL param "id", with JWT claims in context.
func callIDHandler(handler idHandlerFunc, method, path, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// existingAnnouncement returns a mock getAnnouncementFn that always returns a valid announcement.
func existingAnnouncement() func(ctx context.Context, id int64) (*platform.Announcement, error) {
	now := time.Now()
	return func(ctx context.Context, id int64) (*platform.Announcement, error) {
		a := &platform.Announcement{
			Title: "Title", Content: "Content",
			Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo,
		}
		a.ID = testAnnouncementID
		a.CreatedAt = now
		a.UpdatedAt = now
		return a, nil
	}
}

// --- Table-driven: InvalidID returns 400 ---

func TestHandlers_InvalidID(t *testing.T) {
	resource := operator.NewAnnouncementsResource(&mockAnnouncementService{})

	tests := []struct {
		name    string
		handler idHandlerFunc
		method  string
		path    string
	}{
		{"Get", resource.GetAnnouncement, http.MethodGet, "/announcements/abc"},
		{"Delete", resource.DeleteAnnouncement, http.MethodDelete, "/announcements/abc"},
		{"Publish", resource.PublishAnnouncement, http.MethodPost, "/announcements/abc/publish"},
		{"Stats", resource.GetStats, http.MethodGet, "/announcements/abc/stats"},
		{"ViewDetails", resource.GetViewDetails, http.MethodGet, "/announcements/abc/views"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := callIDHandler(tc.handler, tc.method, tc.path, "abc")
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}

	// Update requires a JSON body to reach the ID parse
	t.Run("Update", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"title": "T", "content": "C"})
		req := httptest.NewRequest(http.MethodPut, "/announcements/abc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		claims := jwt.AppClaims{ID: 1}
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		resource.UpdateAnnouncement(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// --- Table-driven: ServiceError returns 500 ---

func TestHandlers_ServiceError(t *testing.T) {
	dbErr := errors.New("database error")

	tests := []struct {
		name    string
		mock    *mockAnnouncementService
		handler func(*operator.AnnouncementsResource) idHandlerFunc
		method  string
		path    string
	}{
		{
			"Get", &mockAnnouncementService{
				getAnnouncementFn: func(_ context.Context, _ int64) (*platform.Announcement, error) { return nil, dbErr },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.GetAnnouncement },
			http.MethodGet, "/announcements/1",
		},
		{
			"Delete", &mockAnnouncementService{
				deleteFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error { return dbErr },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.DeleteAnnouncement },
			http.MethodDelete, "/announcements/1",
		},
		{
			"Publish", &mockAnnouncementService{
				publishFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error { return dbErr },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.PublishAnnouncement },
			http.MethodPost, "/announcements/1/publish",
		},
		{
			"Stats", &mockAnnouncementService{
				getStatsFn: func(_ context.Context, _ int64) (*platform.AnnouncementStats, error) { return nil, dbErr },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.GetStats },
			http.MethodGet, "/announcements/1/stats",
		},
		{
			"ViewDetails", &mockAnnouncementService{
				getViewDetailsFn: func(_ context.Context, _ int64) ([]*platform.AnnouncementViewDetail, error) { return nil, dbErr },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.GetViewDetails },
			http.MethodGet, "/announcements/1/views",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resource := operator.NewAnnouncementsResource(tc.mock)
			rr := callIDHandler(tc.handler(resource), tc.method, tc.path, "1")
			assert.Equal(t, http.StatusInternalServerError, rr.Code)
		})
	}

	// Create and Update service errors
	t.Run("Create", func(t *testing.T) {
		mock := &mockAnnouncementService{
			createFn: func(_ context.Context, _ *platform.Announcement, _ int64, _ net.IP) error { return dbErr },
		}
		resource := operator.NewAnnouncementsResource(mock)
		body, _ := json.Marshal(map[string]any{"title": "T", "content": "C"})
		req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
		rr := httptest.NewRecorder()
		resource.CreateAnnouncement(rr, req.WithContext(ctx))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("Update", func(t *testing.T) {
		mock := &mockAnnouncementService{
			getAnnouncementFn: existingAnnouncement(),
			updateFn:          func(_ context.Context, _ *platform.Announcement, _ int64, _ net.IP) error { return dbErr },
		}
		resource := operator.NewAnnouncementsResource(mock)
		body, _ := json.Marshal(map[string]any{
			"title": "T", "content": "C",
			"type": platform.TypeAnnouncement, "severity": platform.SeverityInfo,
		})
		req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
		rr := httptest.NewRecorder()
		resource.UpdateAnnouncement(rr, req.WithContext(ctx))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// --- Table-driven: NotFound returns 404 ---

func TestHandlers_NotFound(t *testing.T) {
	notFound := &platformSvc.AnnouncementNotFoundError{}

	tests := []struct {
		name    string
		mock    *mockAnnouncementService
		handler func(*operator.AnnouncementsResource) idHandlerFunc
		method  string
		path    string
	}{
		{
			"Get", &mockAnnouncementService{
				getAnnouncementFn: func(_ context.Context, _ int64) (*platform.Announcement, error) { return nil, notFound },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.GetAnnouncement },
			http.MethodGet, "/announcements/999",
		},
		{
			"Delete", &mockAnnouncementService{
				deleteFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error { return notFound },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.DeleteAnnouncement },
			http.MethodDelete, "/announcements/999",
		},
		{
			"Publish", &mockAnnouncementService{
				publishFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error { return notFound },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.PublishAnnouncement },
			http.MethodPost, "/announcements/999/publish",
		},
		{
			"Stats", &mockAnnouncementService{
				getStatsFn: func(_ context.Context, _ int64) (*platform.AnnouncementStats, error) { return nil, notFound },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.GetStats },
			http.MethodGet, "/announcements/999/stats",
		},
		{
			"ViewDetails", &mockAnnouncementService{
				getViewDetailsFn: func(_ context.Context, _ int64) ([]*platform.AnnouncementViewDetail, error) { return nil, notFound },
			}, func(r *operator.AnnouncementsResource) idHandlerFunc { return r.GetViewDetails },
			http.MethodGet, "/announcements/999/views",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resource := operator.NewAnnouncementsResource(tc.mock)
			rr := callIDHandler(tc.handler(resource), tc.method, tc.path, "999")
			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
}

// --- Table-driven: Create with org/tenant targeting ---

func TestCreateAnnouncement_Targeting(t *testing.T) {
	tests := []struct {
		name            string
		orgIDs          []int64
		tenantIDs       []int64
		wantOrgIDs      []int64
		wantTenantIDs   []int64
		wantOrgEmpty    bool
		wantTenantEmpty bool
	}{
		{"BothOrgAndTenant", []int64{1, 2}, []int64{5}, []int64{1, 2}, []int64{5}, false, false},
		{"OrgOnly", []int64{5}, []int64{}, []int64{5}, nil, false, true},
		{"TenantOnly", []int64{}, []int64{12, 13}, nil, []int64{12, 13}, true, false},
		{"Empty", []int64{}, []int64{}, nil, nil, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockAnnouncementService{
				createFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
					if tc.wantOrgEmpty {
						assert.Empty(t, a.TargetOrgIDs)
					} else {
						assert.Equal(t, tc.wantOrgIDs, a.TargetOrgIDs)
					}
					if tc.wantTenantEmpty {
						assert.Empty(t, a.TargetTenantIDs)
					} else {
						assert.Equal(t, tc.wantTenantIDs, a.TargetTenantIDs)
					}
					a.ID = testAnnouncementID
					return nil
				},
			}
			resource := operator.NewAnnouncementsResource(mock)

			body, _ := json.Marshal(map[string]any{
				"title": "T", "content": "C",
				"target_org_ids": tc.orgIDs, "target_tenant_ids": tc.tenantIDs,
			})
			req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
			rr := httptest.NewRecorder()
			resource.CreateAnnouncement(rr, req.WithContext(ctx))
			assert.Equal(t, http.StatusCreated, rr.Code)
		})
	}
}

// --- Update with org/tenant targeting ---

func TestUpdateAnnouncement_WithOrgAndTenantTargeting(t *testing.T) {
	mock := &mockAnnouncementService{
		getAnnouncementFn: existingAnnouncement(),
		updateFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			assert.Equal(t, []int64{3, 4}, a.TargetOrgIDs)
			assert.Equal(t, []int64{7, 8, 9}, a.TargetTenantIDs)
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)

	body, _ := json.Marshal(map[string]any{
		"title": "Updated", "content": "Updated",
		"type": platform.TypeAnnouncement, "severity": platform.SeverityInfo,
		"target_org_ids": []int64{3, 4}, "target_tenant_ids": []int64{7, 8, 9},
	})
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.UpdateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- Create response includes targeting fields ---

func TestCreateAnnouncement_ResponseIncludesTargetingFields(t *testing.T) {
	now := time.Now()
	mock := &mockAnnouncementService{
		createFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			a.ID = testAnnouncementID
			a.CreatedAt = now
			a.UpdatedAt = now
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)

	body, _ := json.Marshal(map[string]any{
		"title": "T", "content": "C",
		"target_org_ids": []int64{1, 2}, "target_tenant_ids": []int64{5},
		"target_roles": []string{"admin"},
	})
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.CreateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusCreated, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	data := response["data"].(map[string]any)
	assert.Len(t, data["target_org_ids"].([]any), 2)
	assert.Len(t, data["target_tenant_ids"].([]any), 1)
	assert.Len(t, data["target_roles"].([]any), 1)
}

// --- Get response includes targeting fields ---

func TestGetAnnouncement_ResponseIncludesTargetingFields(t *testing.T) {
	now := time.Now()
	mock := &mockAnnouncementService{
		getAnnouncementFn: func(_ context.Context, _ int64) (*platform.Announcement, error) {
			a := &platform.Announcement{
				Title: "T", Content: "C",
				Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo, Active: true,
				TargetRoles: []string{"admin"}, TargetOrgIDs: []int64{3}, TargetTenantIDs: []int64{7, 8},
				CreatedBy: testOperatorID,
			}
			a.ID = testAnnouncementID
			a.CreatedAt = now
			a.UpdatedAt = now
			return a, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	rr := callIDHandler(resource.GetAnnouncement, http.MethodGet, "/announcements/1", "1")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	data := response["data"].(map[string]any)
	assert.Equal(t, float64(3), data["target_org_ids"].([]any)[0])
	tenantIDs := data["target_tenant_ids"].([]any)
	assert.Len(t, tenantIDs, 2)
	assert.Equal(t, float64(7), tenantIDs[0])
	assert.Equal(t, float64(8), tenantIDs[1])
}

// --- List includes org/tenant IDs ---

func TestListAnnouncements_IncludesOrgAndTenantIDs(t *testing.T) {
	now := time.Now()
	mock := &mockAnnouncementService{
		listFn: func(_ context.Context, _ bool) ([]*platform.Announcement, error) {
			a := &platform.Announcement{
				Title: "T", Content: "C",
				Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo, Active: true,
				TargetRoles: []string{"teacher"}, TargetOrgIDs: []int64{1}, TargetTenantIDs: []int64{5, 10},
				CreatedBy: testOperatorID,
			}
			a.ID = testAnnouncementID
			a.CreatedAt = now
			a.UpdatedAt = now
			return []*platform.Announcement{a}, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.ListAnnouncements(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	item := response["data"].([]any)[0].(map[string]any)
	orgIDs := item["target_org_ids"].([]any)
	assert.Equal(t, float64(1), orgIDs[0])
	tenantIDs := item["target_tenant_ids"].([]any)
	assert.Len(t, tenantIDs, 2)
}

// --- Nil org/tenant IDs default to empty arrays ---

func TestAnnouncementResponse_NilOrgAndTenantIDsDefaultToEmptyArrays(t *testing.T) {
	now := time.Now()
	mock := &mockAnnouncementService{
		listFn: func(_ context.Context, _ bool) ([]*platform.Announcement, error) {
			a := &platform.Announcement{
				Title: "Global", Content: "C",
				Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo, Active: true,
				TargetRoles: nil, TargetOrgIDs: nil, TargetTenantIDs: nil,
				CreatedBy: testOperatorID,
			}
			a.ID = testAnnouncementID
			a.CreatedAt = now
			a.UpdatedAt = now
			return []*platform.Announcement{a}, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.ListAnnouncements(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	item := response["data"].([]any)[0].(map[string]any)
	orgIDs, ok := item["target_org_ids"].([]any)
	require.True(t, ok, "target_org_ids should be an empty JSON array, not null")
	assert.Empty(t, orgIDs)
	tenantIDs, ok := item["target_tenant_ids"].([]any)
	require.True(t, ok, "target_tenant_ids should be an empty JSON array, not null")
	assert.Empty(t, tenantIDs)
}

// --- Empty list ---

func TestListAnnouncements_EmptyList(t *testing.T) {
	mock := &mockAnnouncementService{
		listFn: func(_ context.Context, _ bool) ([]*platform.Announcement, error) {
			return []*platform.Announcement{}, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)

	req := httptest.NewRequest(http.MethodGet, "/announcements", nil)
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.ListAnnouncements(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Empty(t, response["data"].([]any))
}

// --- Stats response fields ---

func TestGetStats_ResponseFields(t *testing.T) {
	mock := &mockAnnouncementService{
		getStatsFn: func(_ context.Context, id int64) (*platform.AnnouncementStats, error) {
			return &platform.AnnouncementStats{
				AnnouncementID: id, TargetCount: 200, SeenCount: 150, DismissedCount: 30,
			}, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	rr := callIDHandler(resource.GetStats, http.MethodGet, "/announcements/1/stats", "1")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	data := response["data"].(map[string]any)
	assert.Equal(t, float64(200), data["target_count"])
	assert.Equal(t, float64(150), data["seen_count"])
	assert.Equal(t, float64(30), data["dismissed_count"])
}

// --- ViewDetails: empty and multiple ---

func TestGetViewDetails_EmptyList(t *testing.T) {
	mock := &mockAnnouncementService{
		getViewDetailsFn: func(_ context.Context, _ int64) ([]*platform.AnnouncementViewDetail, error) {
			return []*platform.AnnouncementViewDetail{}, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	rr := callIDHandler(resource.GetViewDetails, http.MethodGet, "/announcements/1/views", "1")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Empty(t, response["data"].([]any))
}

func TestGetViewDetails_MultipleDetails(t *testing.T) {
	now := time.Now()
	mock := &mockAnnouncementService{
		getViewDetailsFn: func(_ context.Context, _ int64) ([]*platform.AnnouncementViewDetail, error) {
			return []*platform.AnnouncementViewDetail{
				{UserID: 10, UserName: "Alice", SeenAt: now, Dismissed: false},
				{UserID: 20, UserName: "Bob", SeenAt: now, Dismissed: true},
				{UserID: 30, UserName: "Charlie", SeenAt: now, Dismissed: false},
			}, nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	rr := callIDHandler(resource.GetViewDetails, http.MethodGet, "/announcements/1/views", "1")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	data := response["data"].([]any)
	assert.Len(t, data, 3)
	assert.Equal(t, "Alice", data[0].(map[string]any)["user_name"])
	assert.Equal(t, true, data[1].(map[string]any)["dismissed"])
}

// --- Table-driven: Response status (published, expired, draft) ---

func TestAnnouncementResponse_Status(t *testing.T) {
	now := time.Now()
	publishedAt := now.Add(-1 * time.Hour)
	publishedAtOld := now.Add(-48 * time.Hour)
	expiredAt := now.Add(-1 * time.Hour)

	tests := []struct {
		name       string
		published  *time.Time
		expires    *time.Time
		wantStatus string
	}{
		{"Published", &publishedAt, nil, "published"},
		{"Expired", &publishedAtOld, &expiredAt, "expired"},
		{"Draft", nil, nil, "draft"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockAnnouncementService{
				getAnnouncementFn: func(_ context.Context, _ int64) (*platform.Announcement, error) {
					a := &platform.Announcement{
						Title: tc.name, Content: "C",
						Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo, Active: true,
						PublishedAt: tc.published, ExpiresAt: tc.expires,
						TargetRoles: []string{}, CreatedBy: testOperatorID,
					}
					a.ID = testAnnouncementID
					a.CreatedAt = now
					a.UpdatedAt = now
					return a, nil
				},
			}
			resource := operator.NewAnnouncementsResource(mock)
			rr := callIDHandler(resource.GetAnnouncement, http.MethodGet, "/announcements/1", "1")
			assert.Equal(t, http.StatusOK, rr.Code)

			var response map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
			assert.Equal(t, tc.wantStatus, response["data"].(map[string]any)["status"])
		})
	}
}

// --- Create: default type/severity, version, invalid JSON, InvalidDataError ---

func TestCreateAnnouncement_DefaultTypeAndSeverity(t *testing.T) {
	mock := &mockAnnouncementService{
		createFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			assert.Equal(t, platform.TypeAnnouncement, a.Type)
			assert.Equal(t, platform.SeverityInfo, a.Severity)
			a.ID = testAnnouncementID
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	body, _ := json.Marshal(map[string]any{"title": "T", "content": "C"})
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.CreateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateAnnouncement_WithVersion(t *testing.T) {
	mock := &mockAnnouncementService{
		createFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			require.NotNil(t, a.Version)
			assert.Equal(t, "2.0.0", *a.Version)
			a.ID = testAnnouncementID
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	version := "2.0.0"
	body, _ := json.Marshal(map[string]any{
		"title": "Release", "content": "Notes", "type": platform.TypeRelease, "version": &version,
	})
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.CreateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateAnnouncement_InvalidJSONBody(t *testing.T) {
	resource := operator.NewAnnouncementsResource(&mockAnnouncementService{})
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.CreateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateAnnouncement_InvalidJSONBody(t *testing.T) {
	resource := operator.NewAnnouncementsResource(&mockAnnouncementService{})
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.UpdateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- InvalidDataError for Create and Update ---

func TestCreateAnnouncement_InvalidDataError(t *testing.T) {
	mock := &mockAnnouncementService{
		createFn: func(_ context.Context, _ *platform.Announcement, _ int64, _ net.IP) error {
			return &platformSvc.InvalidDataError{Err: errors.New("severity must be one of: info, warning, critical")}
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	body, _ := json.Marshal(map[string]any{"title": "T", "content": "C", "severity": "bad"})
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.CreateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Contains(t, response["message"], "severity must be one of")
}

func TestUpdateAnnouncement_InvalidDataError(t *testing.T) {
	mock := &mockAnnouncementService{
		getAnnouncementFn: existingAnnouncement(),
		updateFn: func(_ context.Context, _ *platform.Announcement, _ int64, _ net.IP) error {
			return &platformSvc.InvalidDataError{Err: errors.New("title exceeds maximum length")}
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	body, _ := json.Marshal(map[string]any{
		"title": "T", "content": "C",
		"type": platform.TypeAnnouncement, "severity": platform.SeverityInfo,
	})
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.UpdateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Contains(t, response["message"], "title exceeds maximum length")
}

// --- Update: clear/set expires_at, set active flag ---

func TestUpdateAnnouncement_ClearExpiresAt(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	now := time.Now()
	mock := &mockAnnouncementService{
		getAnnouncementFn: func(_ context.Context, _ int64) (*platform.Announcement, error) {
			a := &platform.Announcement{
				Title: "T", Content: "C",
				Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo, ExpiresAt: &expiresAt,
			}
			a.ID = testAnnouncementID
			a.CreatedAt = now
			a.UpdatedAt = now
			return a, nil
		},
		updateFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			assert.Nil(t, a.ExpiresAt, "expires_at should be cleared")
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	empty := ""
	body, _ := json.Marshal(map[string]any{
		"title": "T", "content": "C",
		"type": platform.TypeAnnouncement, "severity": platform.SeverityInfo, "expires_at": &empty,
	})
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.UpdateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateAnnouncement_SetValidExpiresAt(t *testing.T) {
	mock := &mockAnnouncementService{
		getAnnouncementFn: existingAnnouncement(),
		updateFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			assert.NotNil(t, a.ExpiresAt)
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	future := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{
		"title": "T", "content": "C",
		"type": platform.TypeAnnouncement, "severity": platform.SeverityInfo, "expires_at": &future,
	})
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.UpdateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateAnnouncement_SetActiveFlag(t *testing.T) {
	mock := &mockAnnouncementService{
		getAnnouncementFn: func(_ context.Context, _ int64) (*platform.Announcement, error) {
			now := time.Now()
			a := &platform.Announcement{
				Title: "T", Content: "C",
				Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo, Active: true,
			}
			a.ID = testAnnouncementID
			a.CreatedAt = now
			a.UpdatedAt = now
			return a, nil
		},
		updateFn: func(_ context.Context, a *platform.Announcement, _ int64, _ net.IP) error {
			assert.False(t, a.Active, "active flag should be false")
			return nil
		},
	}
	resource := operator.NewAnnouncementsResource(mock)
	active := false
	body, _ := json.Marshal(map[string]any{
		"title": "T", "content": "C",
		"type": platform.TypeAnnouncement, "severity": platform.SeverityInfo, "active": &active,
	})
	req := httptest.NewRequest(http.MethodPut, "/announcements/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 1})
	rr := httptest.NewRecorder()
	resource.UpdateAnnouncement(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusOK, rr.Code)
}
