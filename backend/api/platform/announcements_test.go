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
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
)

// Test constants used in mock assertions (not DB-dependent)
const (
	testAnnouncementID int64 = 1
	testUserID         int64 = 123
)

// Mock AnnouncementService for platform API
type mockPlatformAnnouncementService struct {
	getUnreadForUserFn func(ctx context.Context, userID int64, userRoles []string) ([]*platformModel.Announcement, error)
	countUnreadFn      func(ctx context.Context, userID int64, userRoles []string) (int, error)
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

func (m *mockPlatformAnnouncementService) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*platformModel.Announcement, error) {
	if m.getUnreadForUserFn != nil {
		return m.getUnreadForUserFn(ctx, userID, userRoles)
	}
	return nil, nil
}

func (m *mockPlatformAnnouncementService) CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, userID, userRoles)
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
	now := time.Now()
	version := "1.0.0"
	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string) ([]*platformModel.Announcement, error) {
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
	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string) ([]*platformModel.Announcement, error) {
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

func TestGetUnread_ServiceError(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string) ([]*platformModel.Announcement, error) {
			return nil, errors.New("database error")
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread", nil)
	claims := jwt.AppClaims{ID: 123, Roles: []string{"teacher"}}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnread(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to retrieve announcements")
}

func TestGetUnreadCount_Success(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string) (int, error) {
			assert.Equal(t, testUserID, userID)
			assert.Equal(t, []string{"student", "other"}, userRoles)
			return 5, nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread-count", nil)
	claims := jwt.AppClaims{
		ID:    123,
		Roles: []string{"student", "other"},
	}
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

func TestGetUnreadCount_ServiceError(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string) (int, error) {
			return 0, errors.New("database error")
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/announcements/unread-count", nil)
	claims := jwt.AppClaims{ID: 123, Roles: []string{"teacher"}}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetUnreadCount(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to count announcements")
}

func TestMarkSeen_Success(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{
		markSeenFn: func(ctx context.Context, userID, announcementID int64) error {
			assert.Equal(t, testUserID, userID)
			assert.Equal(t, testAnnouncementID, announcementID)
			return nil
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/1/seen", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkSeen(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "marked as seen")
}

func TestMarkSeen_InvalidID(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{}
	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/abc/seen", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkSeen(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMarkSeen_ServiceError(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{
		markSeenFn: func(ctx context.Context, userID, announcementID int64) error {
			return errors.New("database error")
		},
	}

	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/1/seen", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkSeen(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to mark announcement as seen")
}

func TestMarkDismissed_Success(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{
		markDismissedFn: func(ctx context.Context, userID, announcementID int64) error {
			assert.Equal(t, testUserID, userID)
			assert.Equal(t, testAnnouncementID, announcementID)
			return nil
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

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "dismissed")
}

func TestMarkDismissed_InvalidID(t *testing.T) {
	mockService := &mockPlatformAnnouncementService{}
	resource := platform.NewAnnouncementsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/announcements/invalid/dismiss", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.MarkDismissed(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMarkDismissed_ServiceError(t *testing.T) {
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
