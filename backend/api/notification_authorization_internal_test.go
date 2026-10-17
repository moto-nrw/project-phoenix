package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { routetest.SeedTestJWTConfig() }

type notificationRouteCaptureService struct {
	called   bool
	gotEvent deliveryCompose.NotificationEvent
}

func (c *notificationRouteCaptureService) Notify(_ context.Context, event deliveryCompose.NotificationEvent) error {
	c.called = true
	c.gotEvent = event
	return nil
}

func TestTestNotificationRouteAllowsStaffWithoutConfigUpdate(t *testing.T) {
	t.Parallel()

	testpkg.SetupTestDB(t)

	svc := &notificationRouteCaptureService{}
	router := NewNotificationHTTP(svc, nil, nil).Router()
	claims := routetest.TeacherTestClaims(73)
	require.NotContains(t, claims.Permissions, "config:update")
	req := routetest.NewAuthenticatedRequest(
		t,
		http.MethodPost,
		"/test",
		nil,
		routetest.WithJWTBearer(routetest.MintTestJWT(t, claims)),
	)

	rec := routetest.ExecuteRequest(router, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, svc.called)
	assert.Equal(t, []int64{73}, svc.gotEvent.Audience.StaffAccountIDs)
}
