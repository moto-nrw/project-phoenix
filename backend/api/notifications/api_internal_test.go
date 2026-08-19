package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	testutil.SeedTestJWTConfig()
}

// captureService records the event the handler dispatched.
type captureService struct {
	gotEvent notificationsService.Event
	called   bool
	err      error
}

type pushSubscriptionServiceStub struct {
	notificationsService.PushSubscriptionService
	subscribeErr   error
	unsubscribeErr error
}

func (s pushSubscriptionServiceStub) Subscribe(context.Context, int64, notificationsService.PushSubscriptionInput) error {
	return s.subscribeErr
}

func (s pushSubscriptionServiceStub) Unsubscribe(context.Context, int64, string) error {
	return s.unsubscribeErr
}

func (c *captureService) Notify(_ context.Context, event notificationsService.Event) error {
	c.called = true
	c.gotEvent = event
	return c.err
}

func newTestRequest(tenantID int64) *http.Request {
	ctx := context.Background()
	if tenantID > 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 73})
	return httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(ctx)
}

func TestSendTestNotificationDispatchesOnlyToRequester(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.called)
	assert.Equal(t, "test", svc.gotEvent.Type)
	assert.Equal(t, int64(41), svc.gotEvent.Audience.TenantID)
	assert.Equal(t, notificationsService.ScopeStaff, svc.gotEvent.Audience.Scope)
	assert.Equal(t, []int64{73}, svc.gotEvent.Audience.StaffAccountIDs)
	assert.NotEmpty(t, svc.gotEvent.Title)
}

func TestSendTestNotificationDisabledMapsToConflict(t *testing.T) {
	t.Parallel()

	svc := &captureService{err: notificationsService.ErrDisabled}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSendTestNotificationErrorsMapToInternal(t *testing.T) {
	t.Parallel()

	svc := &captureService{err: errors.New("boom")}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSendTestNotificationMissingTenant(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(0))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, svc.called)
}

func TestSendTestNotificationMissingAccount(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{NotificationsService: svc}
	ctx := tenant.WithTenantID(context.Background(), 41)
	req := httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(ctx)

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, svc.called)
}

func TestSendTestNotificationNilService(t *testing.T) {
	t.Parallel()

	rs := &Resource{}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTestNotificationRouteAllowsStaffWithoutConfigUpdate(t *testing.T) {
	t.Parallel()

	db, _ := testutil.SetupAPITest(t)

	svc := &captureService{}
	router := NewResource(svc, nil, nil, db).Router()
	claims := testutil.TeacherTestClaims(73)
	require.NotContains(t, claims.Permissions, "config:update")
	req := testutil.NewAuthenticatedRequest(
		t,
		http.MethodPost,
		"/test",
		nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rec := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, svc.called)
	assert.Equal(t, []int64{73}, svc.gotEvent.Audience.StaffAccountIDs)
}

func TestSubscribePushClassifiesServiceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serviceErr    error
		wantStatus    int
		wantBody      string
		forbiddenBody string
	}{
		{
			name:       "invalid browser subscription",
			serviceErr: notificationsService.ErrInvalidPushSubscription,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:          "repository failure",
			serviceErr:    errors.New("database connection failed: internal detail"),
			wantStatus:    http.StatusInternalServerError,
			wantBody:      "Push-Registrierung konnte nicht gespeichert werden.",
			forbiddenBody: "internal detail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &Resource{PushService: pushSubscriptionServiceStub{subscribeErr: tt.serviceErr}}
			req := httptest.NewRequest(
				http.MethodPost,
				"/push/subscriptions",
				strings.NewReader(`{"endpoint":"https://fcm.googleapis.com/fcm/send/device","keys":{"p256dh":"key","auth":"auth"}}`),
			)
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 42}))
			rec := httptest.NewRecorder()

			rs.subscribePush(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Contains(t, rec.Body.String(), tt.wantBody)
			}
			if tt.forbiddenBody != "" {
				assert.NotContains(t, rec.Body.String(), tt.forbiddenBody)
			}
		})
	}
}

func TestUnsubscribePushHidesServiceErrors(t *testing.T) {
	t.Parallel()

	rs := &Resource{PushService: pushSubscriptionServiceStub{
		unsubscribeErr: errors.New("delete subscription: pq: internal detail"),
	}}
	req := httptest.NewRequest(
		http.MethodDelete,
		"/push/subscriptions?endpoint=https%3A%2F%2Ffcm.googleapis.com%2Ffcm%2Fsend%2Fdevice",
		nil,
	)
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 42}))
	rec := httptest.NewRecorder()

	rs.unsubscribePush(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Push-Registrierung konnte nicht entfernt werden.")
	assert.NotContains(t, rec.Body.String(), "internal detail")
}
