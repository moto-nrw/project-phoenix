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

func (m *mockAnnouncementService) GetUnreadForUser(ctx context.Context, userID int64, userRole string) ([]*platform.Announcement, error) {
	return nil, nil
}

func (m *mockAnnouncementService) CountUnread(ctx context.Context, userID int64, userRole string) (int, error) {
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
				CreatedBy:   1,
			}
			announcement.ID = 1
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

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]interface{})
	assert.Len(t, data, 1)
	announcement := data[0].(map[string]interface{})
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
			assert.Equal(t, int64(1), id)
			announcement := &platform.Announcement{
				Title:       "Test",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				TargetRoles: []string{},
				CreatedBy:   1,
			}
			announcement.ID = 1
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
			assert.Equal(t, int64(1), operatorID)
			announcement.ID = 1
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]interface{}{
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

	body := map[string]interface{}{
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

	body := map[string]interface{}{
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
	body := map[string]interface{}{
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
	body := map[string]interface{}{
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
				CreatedBy:   1,
			}
			announcement.ID = 1
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

	body := map[string]interface{}{
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

	body := map[string]interface{}{
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
			announcement.ID = 1
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return announcement, nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	expiresAt := "invalid-date"
	body := map[string]interface{}{
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
			assert.Equal(t, int64(1), id)
			assert.Equal(t, int64(123), operatorID)
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
			assert.Equal(t, int64(1), id)
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

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]interface{})
	assert.Len(t, data, 1)
	detail := data[0].(map[string]interface{})
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
