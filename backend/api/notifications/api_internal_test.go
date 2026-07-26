package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureService records the event the handler dispatched.
type captureService struct {
	gotEvent notificationsService.Event
	called   bool
	err      error
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
	return httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(ctx)
}

func TestSendTestNotificationDispatchesTenantEvent(t *testing.T) {
	svc := &captureService{}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.called)
	assert.Equal(t, "test", svc.gotEvent.Type)
	assert.Equal(t, int64(41), svc.gotEvent.Audience.TenantID)
	assert.Equal(t, notificationsService.ScopeTenant, svc.gotEvent.Audience.Scope)
	assert.NotEmpty(t, svc.gotEvent.Title)
}

func TestSendTestNotificationDisabledMapsToConflict(t *testing.T) {
	svc := &captureService{err: notificationsService.ErrDisabled}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSendTestNotificationErrorsMapToInternal(t *testing.T) {
	svc := &captureService{err: errors.New("boom")}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSendTestNotificationMissingTenant(t *testing.T) {
	svc := &captureService{}
	rs := &Resource{NotificationsService: svc}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(0))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, svc.called)
}

func TestSendTestNotificationNilService(t *testing.T) {
	rs := &Resource{}

	rec := httptest.NewRecorder()
	rs.sendTestNotification(rec, newTestRequest(41))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
