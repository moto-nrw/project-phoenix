package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	routetest.SeedTestJWTConfig()
}

// captureService records the event the handler dispatched.
type captureService struct {
	gotEvent Event
	called   bool
	err      error
}

type pushSubscriptionServiceStub struct {
	PushSubscriptionService
	subscribeErr       error
	unsubscribeErr     error
	unsubscribedPortal *string
}

func (s pushSubscriptionServiceStub) Subscribe(context.Context, int64, PushSubscriptionInput) error {
	return s.subscribeErr
}

func (s pushSubscriptionServiceStub) Unsubscribe(context.Context, int64, string) error {
	if s.unsubscribedPortal != nil {
		*s.unsubscribedPortal = PortalStaff
	}
	return s.unsubscribeErr
}

func (s pushSubscriptionServiceStub) UnsubscribeSchool(context.Context, int64, string) error {
	if s.unsubscribedPortal != nil {
		*s.unsubscribedPortal = PortalSchool
	}
	return s.unsubscribeErr
}

func (c *captureService) Notify(_ context.Context, event Event) error {
	c.called = true
	c.gotEvent = event
	return c.err
}

func newTestRequest(tenantID int64) *http.Request {
	ctx := context.Background()
	if tenantID > 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	ctx = routetest.WithAuthenticatedContext(ctx, routetest.Claims{ID: 73}, nil)
	return httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(ctx)
}

func TestSendTestNotificationDispatchesOnlyToRequester(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{Runtime: testNotificationRuntime(), NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.called)
	assert.Equal(t, "test", svc.gotEvent.Type)
	assert.Equal(t, int64(41), svc.gotEvent.Audience.TenantID)
	assert.Equal(t, ScopeStaff, svc.gotEvent.Audience.Scope)
	assert.Equal(t, []int64{73}, svc.gotEvent.Audience.StaffAccountIDs)
	assert.NotEmpty(t, svc.gotEvent.Title)
	assert.Equal(t, PortalStaff, svc.gotEvent.Portal)
}

// The test notification proves the setup of the portal it was fired from, so
// a request through the school mount must not reach the OGS devices (#2208).
func TestSendTestNotificationCarriesTheRequestingPortal(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{Runtime: testNotificationRuntime(), NotificationsService: svc}

	req := newTestRequest(41)
	req = req.WithContext(context.WithValue(req.Context(), portalCtxKey{}, PortalSchool))

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, PortalSchool, svc.gotEvent.Portal)
}

func TestSendTestNotificationDisabledMapsToConflict(t *testing.T) {
	t.Parallel()

	svc := &captureService{err: errNotificationTestDisabled}
	rs := &Resource{Runtime: testNotificationRuntime(), NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(),
		`"code":"`+"communication.notifications_disabled"+`"`)
}

func TestSendTestNotificationErrorsMapToInternal(t *testing.T) {
	t.Parallel()

	svc := &captureService{err: errors.New("boom")}
	rs := &Resource{Runtime: testNotificationRuntime(), NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSendTestNotificationMissingTenant(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{Runtime: testNotificationRuntime(), NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(0))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, svc.called)
}

func TestSendTestNotificationMissingAccount(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	rs := &Resource{Runtime: testNotificationRuntime(), NotificationsService: svc}
	ctx := tenant.WithTenantID(context.Background(), 41)
	req := httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(ctx)

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, svc.called)
}

func TestSendTestNotificationNilService(t *testing.T) {
	t.Parallel()

	rs := &Resource{Runtime: testNotificationRuntime()}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
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
			serviceErr: errNotificationTestInvalidPushSubscription,
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
			rs := &Resource{Runtime: testNotificationRuntime(), PushService: pushSubscriptionServiceStub{subscribeErr: tt.serviceErr}}
			req := httptest.NewRequest(
				http.MethodPost,
				"/push/subscriptions",
				strings.NewReader(`{"endpoint":"https://fcm.googleapis.com/fcm/send/device","keys":{"p256dh":"key","auth":"auth"}}`),
			)
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(routetest.WithAuthenticatedContext(req.Context(), routetest.Claims{ID: 42}, nil))
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

	rs := &Resource{Runtime: testNotificationRuntime(), PushService: pushSubscriptionServiceStub{
		unsubscribeErr: errors.New("delete subscription: pq: internal detail"),
	}}
	req := httptest.NewRequest(
		http.MethodDelete,
		"/push/subscriptions?endpoint=https%3A%2F%2Ffcm.googleapis.com%2Ffcm%2Fsend%2Fdevice",
		nil,
	)
	req = req.WithContext(routetest.WithAuthenticatedContext(req.Context(), routetest.Claims{ID: 42}, nil))
	rec := httptest.NewRecorder()

	rs.unsubscribePush(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Push-Registrierung konnte nicht entfernt werden.")
	assert.NotContains(t, rec.Body.String(), "internal detail")
}

func TestUnsubscribePushUsesSchoolPortalSubscription(t *testing.T) {
	t.Parallel()

	portal := ""
	rs := &Resource{Runtime: testNotificationRuntime(), PushService: pushSubscriptionServiceStub{unsubscribedPortal: &portal}}
	req := httptest.NewRequest(
		http.MethodDelete,
		"/push/subscriptions?endpoint=https%3A%2F%2Ffcm.googleapis.com%2Ffcm%2Fsend%2Fdevice",
		nil,
	)
	ctx := routetest.WithAuthenticatedContext(req.Context(), routetest.Claims{ID: 42}, nil)
	ctx = context.WithValue(ctx, portalCtxKey{}, PortalSchool)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	rs.unsubscribePush(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, PortalSchool, portal)
}

var (
	errNotificationTestDisabled                = errors.New("notifications are disabled for this tenant")
	errNotificationTestWebPushNotConfigured    = errors.New("web push is not configured on this server")
	errNotificationTestInvalidPushSubscription = errors.New("invalid push subscription")
)

func (*captureService) IsDisabled(err error) bool { return errors.Is(err, errNotificationTestDisabled) }
func (pushSubscriptionServiceStub) IsNotConfigured(err error) bool {
	return errors.Is(err, errNotificationTestWebPushNotConfigured)
}
func (pushSubscriptionServiceStub) IsInvalidSubscription(err error) bool {
	return errors.Is(err, errNotificationTestInvalidPushSubscription)
}

// testNotificationRuntime uses the existing wire/request support for owner
// handler cases. Root binds the native runtime in composition/golden suites.
func testNotificationRuntime() Runtime {
	return Runtime{
		TenantID: tenant.FromContext,
		ActorID: func(ctx context.Context) *int64 {
			claims := routetest.ClaimsFromContext(ctx)
			if claims.ID <= 0 {
				return nil
			}
			id := int64(claims.ID)
			return &id
		},
		Caller: func(ctx context.Context) Caller {
			claims := routetest.ClaimsFromContext(ctx)
			return Caller{ID: claims.ID, FamilyID: claims.FamilyID}
		},
		Respond: routetest.RespondSuccess,
		Internal: func(w http.ResponseWriter, r *http.Request, err error) {
			routetest.RespondError(w, r, http.StatusInternalServerError, err)
		},
		Forbidden: func(w http.ResponseWriter, r *http.Request, err error) {
			routetest.RespondError(w, r, http.StatusForbidden, err)
		},
		Unauthorized: func(w http.ResponseWriter, r *http.Request) {
			routetest.RespondError(w, r, http.StatusUnauthorized, errors.New("token is unauthorized"))
		},
		Invalid: func(w http.ResponseWriter, r *http.Request, err error) { routetest.RespondInvalidRequest(w, r, err) },
		Disabled: func(w http.ResponseWriter, r *http.Request) {
			routetest.RespondCoded(w, r, http.StatusConflict, "communication.notifications_disabled", errNotificationTestDisabled, nil, "")
		},
		Conflict: func(w http.ResponseWriter, r *http.Request, err error) {
			routetest.RespondError(w, r, http.StatusConflict, err)
		},
		NotFound: func(w http.ResponseWriter, r *http.Request, err error) {
			routetest.RespondError(w, r, http.StatusNotFound, err)
		},
		Wrapped: func(w http.ResponseWriter, r *http.Request, message string, _ error) {
			routetest.RespondError(w, r, http.StatusInternalServerError, errors.New(message))
		},
	}
}
