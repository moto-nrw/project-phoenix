package platform_test

import (
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

	"github.com/moto-nrw/project-phoenix/api/platform"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModel "github.com/moto-nrw/project-phoenix/modules/communication"
)

// Test constants used in mock assertions (not DB-dependent)
const (
	testAnnouncementID int64 = 1
	testUserID         int64 = 123
)

// Mock AnnouncementService for platform API
type mockPlatformAnnouncementService struct {
	getUnreadForUserFn func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platformModel.Announcement, error)
	countUnreadFn      func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error)
	markSeenFn         func(ctx context.Context, userID, announcementID int64) error
	markDismissedFn    func(ctx context.Context, userID, announcementID int64) error
}

func (m *mockPlatformAnnouncementService) CreateAnnouncement(ctx context.Context, announcement *platformModel.Announcement, operatorID int64, clientIP net.IP) error {
	return nil
}

func (m *mockPlatformAnnouncementService) GetAnnouncement(ctx context.Context, id int64) (*platformModel.Announcement, error) {
	return nil, nil
}

func (m *mockPlatformAnnouncementService) UpdateAnnouncement(ctx context.Context, announcement *platformModel.Announcement, operatorID int64, clientIP net.IP) error {
	return nil
}

func (m *mockPlatformAnnouncementService) DeleteAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	return nil
}

func (m *mockPlatformAnnouncementService) ListAnnouncements(ctx context.Context, includeInactive bool) ([]*platformModel.Announcement, error) {
	return nil, nil
}

func (m *mockPlatformAnnouncementService) PublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	return nil
}

func (m *mockPlatformAnnouncementService) UnpublishAnnouncement(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	return nil
}

func (m *mockPlatformAnnouncementService) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platformModel.Announcement, error) {
	if m.getUnreadForUserFn != nil {
		return m.getUnreadForUserFn(ctx, userID, userRoles, tenantID, orgID)
	}
	return nil, nil
}

func (m *mockPlatformAnnouncementService) CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, userID, userRoles, tenantID, orgID)
	}
	return 0, nil
}

func (m *mockPlatformAnnouncementService) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	if m.markSeenFn != nil {
		return m.markSeenFn(ctx, userID, announcementID)
	}
	return nil
}

func (m *mockPlatformAnnouncementService) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	if m.markDismissedFn != nil {
		return m.markDismissedFn(ctx, userID, announcementID)
	}
	return nil
}

func (m *mockPlatformAnnouncementService) GetStats(ctx context.Context, id int64) (*platformModel.AnnouncementStats, error) {
	return nil, nil
}

func (m *mockPlatformAnnouncementService) GetViewDetails(ctx context.Context, id int64) ([]*platformModel.AnnouncementViewDetail, error) {
	return nil, nil
}

func TestGetUnread_Success(t *testing.T) {
	t.Parallel()

	now := time.Now()
	version := "1.0.0"
	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platformModel.Announcement, error) {
			assert.Equal(t, testUserID, userID)
			assert.Equal(t, []string{"teacher"}, userRoles)
			announcement := &platformModel.Announcement{
				Title:       "Important Update",
				Content:     "Please read this",
				Type:        platformModel.TypeAnnouncement,
				Severity:    platformModel.SeverityInfo,
				Version:     &version,
				PublishedAt: &now,
				Active:      true,
			}
			announcement.ID = testAnnouncementID
			return []*platformModel.Announcement{announcement}, nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread", nil)
	claims := jwt.AppClaims{
		ID:    123,
		Roles: []string{"teacher"},
	}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnread(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]any)
	assert.Len(t, data, 1)
	announcement := data[0].(map[string]any)
	assert.Equal(t, "Important Update", announcement["title"])
	assert.Equal(t, "1.0.0", announcement["version"])
}

func TestGetUnread_NoRoles(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platformModel.Announcement, error) {
			assert.Equal(t, []string{}, userRoles)
			return []*platformModel.Announcement{}, nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread", nil)
	claims := jwt.AppClaims{
		ID:    123,
		Roles: []string{},
	}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnread(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMarkDismissed_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{
		markDismissedFn: func(ctx context.Context, userID, announcementID int64) error {
			return errors.New("database error")
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/1/dismiss", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkDismissed(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to mark announcement as dismissed")
}

func TestGetUnread_ResponseIncludesPublishedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platformModel.Announcement, error) {
			announcement := &platformModel.Announcement{
				Title:       "Test",
				Content:     "Content",
				Type:        platformModel.TypeRelease,
				Severity:    platformModel.SeverityWarning,
				PublishedAt: &now,
				Active:      true,
			}
			announcement.ID = testAnnouncementID
			return []*platformModel.Announcement{announcement}, nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread", nil)
	claims := jwt.AppClaims{
		ID:       123,
		Roles:    []string{"teacher"},
		OrgID:    5,
		TenantID: 10,
	}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnread(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]any)
	require.Len(t, data, 1)
	announcement := data[0].(map[string]any)
	assert.NotEmpty(t, announcement["published_at"])
	assert.Equal(t, "release", announcement["type"])
	assert.Equal(t, "warning", announcement["severity"])
}

func TestGetUnread_NilPublishedAtRendersEmpty(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platformModel.Announcement, error) {
			announcement := &platformModel.Announcement{
				Title:       "Draft",
				Content:     "Content",
				Type:        platformModel.TypeAnnouncement,
				Severity:    platformModel.SeverityInfo,
				PublishedAt: nil,
				Active:      true,
			}
			announcement.ID = testAnnouncementID
			return []*platformModel.Announcement{announcement}, nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread", nil)
	claims := jwt.AppClaims{ID: 123, Roles: []string{"teacher"}}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnread(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]any)
	require.Len(t, data, 1)
	announcement := data[0].(map[string]any)
	assert.Equal(t, "", announcement["published_at"])
}

// --- GetUnreadCount and MarkSeen ---
// These tests cover the 2 platform handler functions that were previously untested.

func TestGetUnreadCount_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{
		countUnreadFn: func(_ context.Context, _ int64, _ []string, _ int64, _ int64) (int, error) {
			return 5, nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread/count", nil)
	claims := jwt.AppClaims{ID: 123, Roles: []string{"teacher"}, TenantID: 10, OrgID: 5}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnreadCount(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	data := response["data"].(map[string]any)
	assert.Equal(t, float64(5), data["count"])
}

func TestMarkSeen_SuccessAndErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		markSeenFn func(context.Context, int64, int64) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success",
			id:         "1",
			wantStatus: http.StatusOK,
			wantBody:   "Announcement marked as seen",
		},
		{
			name:       "invalid ID",
			id:         "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "service error",
			id:   "1",
			markSeenFn: func(_ context.Context, _, _ int64) error {
				return errors.New("database error")
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "failed to mark announcement as seen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPlatformAnnouncementService{markSeenFn: tt.markSeenFn}
			resource := platform.NewAnnouncementsResource(mockService)

			req := httptest.NewRequest(http.MethodPost, "/announcements/"+tt.id+"/seen", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.id)
			claims := jwt.AppClaims{ID: 123}
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			resource.MarkSeen(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantBody != "" {
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestMarkDismissed_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{}
	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/1/dismiss", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkDismissed(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Announcement dismissed")
}

func TestMarkDismissed_InvalidID(t *testing.T) {
	t.Parallel()

	resource := platform.NewAnnouncementsResource(&mockPlatformAnnouncementService{})

	req := httptest.NewRequest(http.MethodPost, "/announcements/abc/dismiss", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkDismissed(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Error paths for GetUnread and GetUnreadCount ---

func TestGetUnread_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(_ context.Context, _ int64, _ []string, _ int64, _ int64) ([]*platformModel.Announcement, error) {
			return nil, errors.New("database error")
		},
	}
	resource := platform.NewAnnouncementsResource(mockService)
	req := httptest.NewRequest(http.MethodGet, "/announcements/unread", nil)
	claims := jwt.AppClaims{ID: 123, Roles: []string{"teacher"}}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	rr := httptest.NewRecorder()
	resource.GetUnread(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetUnreadCount_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockPlatformAnnouncementService{
		countUnreadFn: func(_ context.Context, _ int64, _ []string, _ int64, _ int64) (int, error) {
			return 0, errors.New("database error")
		},
	}
	resource := platform.NewAnnouncementsResource(mockService)
	req := httptest.NewRequest(http.MethodGet, "/announcements/unread/count", nil)
	claims := jwt.AppClaims{ID: 123, Roles: []string{"teacher"}}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	rr := httptest.NewRecorder()
	resource.GetUnreadCount(rr, req.WithContext(ctx))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
