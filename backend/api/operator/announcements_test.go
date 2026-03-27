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

func (m *mockAnnouncementService) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) ([]*platform.Announcement, error) {
	return nil, nil
}

func (m *mockAnnouncementService) CountUnread(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) (int, error) {
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

func TestCreateAnnouncement_WithOrgAndTenantTargeting(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, []int64{1, 2}, announcement.TargetOrgIDs)
			assert.Equal(t, []int64{5}, announcement.TargetTenantIDs)
			assert.Equal(t, testOperatorID, operatorID)
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":             "Targeted Announcement",
		"content":           "Content for specific orgs and tenants",
		"target_org_ids":    []int64{1, 2},
		"target_tenant_ids": []int64{5},
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

func TestCreateAnnouncement_EmptyOrgAndTenantTargeting(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.NotNil(t, announcement.TargetOrgIDs, "TargetOrgIDs should not be nil")
			assert.NotNil(t, announcement.TargetTenantIDs, "TargetTenantIDs should not be nil")
			assert.Empty(t, announcement.TargetOrgIDs)
			assert.Empty(t, announcement.TargetTenantIDs)
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":             "Global Announcement",
		"content":           "Content for everyone",
		"target_org_ids":    []int64{},
		"target_tenant_ids": []int64{},
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

func TestUpdateAnnouncement_WithOrgAndTenantTargeting(t *testing.T) {
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
			assert.Equal(t, []int64{3, 4}, announcement.TargetOrgIDs)
			assert.Equal(t, []int64{7, 8, 9}, announcement.TargetTenantIDs)
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":             "Updated Title",
		"content":           "Updated Content",
		"type":              platform.TypeAnnouncement,
		"severity":          platform.SeverityInfo,
		"target_org_ids":    []int64{3, 4},
		"target_tenant_ids": []int64{7, 8, 9},
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

func TestListAnnouncements_IncludesOrgAndTenantIDs(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:           "Targeted Announcement",
				Content:         "Content",
				Type:            platform.TypeAnnouncement,
				Severity:        platform.SeverityInfo,
				Active:          true,
				TargetRoles:     []string{"teacher"},
				TargetOrgIDs:    []int64{1},
				TargetTenantIDs: []int64{5, 10},
				CreatedBy:       testOperatorID,
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
	require.Len(t, data, 1)
	announcement := data[0].(map[string]any)

	// Verify target_org_ids is present as an array
	orgIDs, ok := announcement["target_org_ids"].([]any)
	require.True(t, ok, "target_org_ids should be a JSON array")
	assert.Len(t, orgIDs, 1)
	assert.Equal(t, float64(1), orgIDs[0])

	// Verify target_tenant_ids is present as an array
	tenantIDs, ok := announcement["target_tenant_ids"].([]any)
	require.True(t, ok, "target_tenant_ids should be a JSON array")
	assert.Len(t, tenantIDs, 2)
	assert.Equal(t, float64(5), tenantIDs[0])
	assert.Equal(t, float64(10), tenantIDs[1])
}

func TestCreateAnnouncement_OrgOnlyTargeting(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, []int64{5}, announcement.TargetOrgIDs)
			assert.Empty(t, announcement.TargetTenantIDs)
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":             "Org Only",
		"content":           "Only for org 5",
		"target_org_ids":    []int64{5},
		"target_tenant_ids": []int64{},
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

func TestCreateAnnouncement_TenantOnlyTargeting(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Empty(t, announcement.TargetOrgIDs)
			assert.Equal(t, []int64{12, 13}, announcement.TargetTenantIDs)
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":             "Tenant Only",
		"content":           "Only for tenants 12 and 13",
		"target_org_ids":    []int64{},
		"target_tenant_ids": []int64{12, 13},
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

func TestCreateAnnouncement_ResponseIncludesTargetingFields(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":             "Targeted",
		"content":           "Content",
		"target_org_ids":    []int64{1, 2},
		"target_tenant_ids": []int64{5},
		"target_roles":      []string{"admin"},
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

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]any)
	orgIDs, ok := data["target_org_ids"].([]any)
	require.True(t, ok, "response should include target_org_ids array")
	assert.Len(t, orgIDs, 2)

	tenantIDs, ok := data["target_tenant_ids"].([]any)
	require.True(t, ok, "response should include target_tenant_ids array")
	assert.Len(t, tenantIDs, 1)

	roles, ok := data["target_roles"].([]any)
	require.True(t, ok, "response should include target_roles array")
	assert.Len(t, roles, 1)
}

func TestCreateAnnouncement_ServiceError(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			return errors.New("database error")
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":   "Test",
		"content": "Content",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.CreateAnnouncement(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAnnouncement_ResponseIncludesTargetingFields(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:           "Targeted",
				Content:         "Content",
				Type:            platform.TypeAnnouncement,
				Severity:        platform.SeverityInfo,
				Active:          true,
				TargetRoles:     []string{"admin"},
				TargetOrgIDs:    []int64{3},
				TargetTenantIDs: []int64{7, 8},
				CreatedBy:       testOperatorID,
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

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]any)
	orgIDs := data["target_org_ids"].([]any)
	assert.Equal(t, float64(3), orgIDs[0])

	tenantIDs := data["target_tenant_ids"].([]any)
	assert.Len(t, tenantIDs, 2)
	assert.Equal(t, float64(7), tenantIDs[0])
	assert.Equal(t, float64(8), tenantIDs[1])
}

func TestAnnouncementResponse_NilOrgAndTenantIDsDefaultToEmptyArrays(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:           "Global Announcement",
				Content:         "Content for everyone",
				Type:            platform.TypeAnnouncement,
				Severity:        platform.SeverityInfo,
				Active:          true,
				TargetRoles:     nil,
				TargetOrgIDs:    nil,
				TargetTenantIDs: nil,
				CreatedBy:       testOperatorID,
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
	require.Len(t, data, 1)
	announcement := data[0].(map[string]any)

	// Verify nil slices are serialized as empty arrays, not null or omitted
	orgIDs, ok := announcement["target_org_ids"].([]any)
	require.True(t, ok, "target_org_ids should be an empty JSON array, not null or omitted")
	assert.Empty(t, orgIDs)

	tenantIDs, ok := announcement["target_tenant_ids"].([]any)
	require.True(t, ok, "target_tenant_ids should be an empty JSON array, not null or omitted")
	assert.Empty(t, tenantIDs)
}

func TestGetStats_InvalidID(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/abc/stats", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetStats(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetStats_NotFound(t *testing.T) {
	mockService := &mockAnnouncementService{
		getStatsFn: func(ctx context.Context, id int64) (*platform.AnnouncementStats, error) {
			return nil, &platformSvc.AnnouncementNotFoundError{}
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/999/stats", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetStats(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetStats_ServiceError(t *testing.T) {
	mockService := &mockAnnouncementService{
		getStatsFn: func(ctx context.Context, id int64) (*platform.AnnouncementStats, error) {
			return nil, errors.New("database error")
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

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetStats_ResponseFields(t *testing.T) {
	mockService := &mockAnnouncementService{
		getStatsFn: func(ctx context.Context, id int64) (*platform.AnnouncementStats, error) {
			return &platform.AnnouncementStats{
				AnnouncementID: id,
				TargetCount:    200,
				SeenCount:      150,
				DismissedCount: 30,
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

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]any)
	assert.Equal(t, float64(200), data["target_count"])
	assert.Equal(t, float64(150), data["seen_count"])
	assert.Equal(t, float64(30), data["dismissed_count"])
}

func TestGetViewDetails_InvalidID(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/abc/views", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetViewDetails(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetViewDetails_NotFound(t *testing.T) {
	mockService := &mockAnnouncementService{
		getViewDetailsFn: func(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error) {
			return nil, &platformSvc.AnnouncementNotFoundError{}
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/999/views", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetViewDetails(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetViewDetails_ServiceError(t *testing.T) {
	mockService := &mockAnnouncementService{
		getViewDetailsFn: func(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error) {
			return nil, errors.New("database error")
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

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetViewDetails_EmptyList(t *testing.T) {
	mockService := &mockAnnouncementService{
		getViewDetailsFn: func(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error) {
			return []*platform.AnnouncementViewDetail{}, nil
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
	assert.Empty(t, data)
}

func TestGetViewDetails_MultipleDetails(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getViewDetailsFn: func(ctx context.Context, id int64) ([]*platform.AnnouncementViewDetail, error) {
			return []*platform.AnnouncementViewDetail{
				{UserID: 10, UserName: "Alice", SeenAt: now, Dismissed: false},
				{UserID: 20, UserName: "Bob", SeenAt: now, Dismissed: true},
				{UserID: 30, UserName: "Charlie", SeenAt: now, Dismissed: false},
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
	assert.Len(t, data, 3)
	firstDetail := data[0].(map[string]any)
	assert.Equal(t, "Alice", firstDetail["user_name"])
	secondDetail := data[1].(map[string]any)
	assert.Equal(t, true, secondDetail["dismissed"])
}

func TestDeleteAnnouncement_InvalidID(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/announcements/abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDeleteAnnouncement_ServiceError(t *testing.T) {
	mockService := &mockAnnouncementService{
		deleteFn: func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
			return errors.New("database error")
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/announcements/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.DeleteAnnouncement(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPublishAnnouncement_InvalidID(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/abc/publish", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.PublishAnnouncement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPublishAnnouncement_NotFound(t *testing.T) {
	mockService := &mockAnnouncementService{
		publishFn: func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
			return &platformSvc.AnnouncementNotFoundError{}
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/999/publish", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	claims := jwt.AppClaims{ID: 1}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.PublishAnnouncement(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPublishAnnouncement_ServiceError(t *testing.T) {
	mockService := &mockAnnouncementService{
		publishFn: func(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
			return errors.New("database error")
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

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUpdateAnnouncement_InvalidID(t *testing.T) {
	mockService := &mockAnnouncementService{}
	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{"title": "Test", "content": "Content"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/announcements/abc", bytes.NewReader(jsonBody))
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
}

func TestUpdateAnnouncement_ServiceError(t *testing.T) {
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
		updateFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			return errors.New("update failed")
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

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUpdateAnnouncement_ClearExpiresAt(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:     "Title",
				Content:   "Content",
				Type:      platform.TypeAnnouncement,
				Severity:  platform.SeverityInfo,
				ExpiresAt: &expiresAt,
			}
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return announcement, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Nil(t, announcement.ExpiresAt, "expires_at should be cleared when empty string is sent")
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	emptyStr := ""
	body := map[string]any{
		"title":      "Title",
		"content":    "Content",
		"type":       platform.TypeAnnouncement,
		"severity":   platform.SeverityInfo,
		"expires_at": &emptyStr,
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

func TestUpdateAnnouncement_SetValidExpiresAt(t *testing.T) {
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
		updateFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.NotNil(t, announcement.ExpiresAt)
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	futureTime := now.Add(72 * time.Hour).Format(time.RFC3339)
	body := map[string]any{
		"title":      "Title",
		"content":    "Content",
		"type":       platform.TypeAnnouncement,
		"severity":   platform.SeverityInfo,
		"expires_at": &futureTime,
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

func TestUpdateAnnouncement_SetActiveFlag(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:    "Title",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
				Active:   true,
			}
			announcement.ID = testAnnouncementID
			announcement.CreatedAt = now
			announcement.UpdatedAt = now
			return announcement, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.False(t, announcement.Active, "active flag should be set to false")
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	activeVal := false
	body := map[string]any{
		"title":    "Title",
		"content":  "Content",
		"type":     platform.TypeAnnouncement,
		"severity": platform.SeverityInfo,
		"active":   &activeVal,
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

func TestNewAnnouncementResponse_PublishedStatus(t *testing.T) {
	now := time.Now()
	publishedAt := now.Add(-1 * time.Hour)
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:       "Published",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				PublishedAt: &publishedAt,
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

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]any)
	assert.Equal(t, "published", data["status"])
	assert.NotEmpty(t, data["published_at"])
}

func TestNewAnnouncementResponse_ExpiredStatus(t *testing.T) {
	now := time.Now()
	publishedAt := now.Add(-48 * time.Hour)
	expiresAt := now.Add(-1 * time.Hour)
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:       "Expired",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				PublishedAt: &publishedAt,
				ExpiresAt:   &expiresAt,
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

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]any)
	assert.Equal(t, "expired", data["status"])
	assert.NotEmpty(t, data["expires_at"])
}

func TestNewAnnouncementResponse_DraftStatus(t *testing.T) {
	now := time.Now()
	mockService := &mockAnnouncementService{
		getAnnouncementFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			announcement := &platform.Announcement{
				Title:       "Draft",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				Active:      true,
				PublishedAt: nil,
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

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]any)
	assert.Equal(t, "draft", data["status"])
}

func TestCreateAnnouncement_DefaultTypeAndSeverity(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, platform.TypeAnnouncement, announcement.Type, "type should default to announcement")
			assert.Equal(t, platform.SeverityInfo, announcement.Severity, "severity should default to info")
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	body := map[string]any{
		"title":   "Test Title",
		"content": "Test Content",
		// No type or severity specified
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

func TestListAnnouncements_EmptyList(t *testing.T) {
	mockService := &mockAnnouncementService{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			return []*platform.Announcement{}, nil
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
	assert.Empty(t, data)
}

func TestCreateAnnouncement_WithVersion(t *testing.T) {
	mockService := &mockAnnouncementService{
		createFn: func(ctx context.Context, announcement *platform.Announcement, operatorID int64, clientIP net.IP) error {
			assert.NotNil(t, announcement.Version)
			assert.Equal(t, "2.0.0", *announcement.Version)
			announcement.ID = testAnnouncementID
			return nil
		},
	}

	resource := operator.NewAnnouncementsResource(mockService)

	version := "2.0.0"
	body := map[string]any{
		"title":   "Release",
		"content": "Release notes",
		"type":    platform.TypeRelease,
		"version": &version,
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
