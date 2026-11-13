package sse

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-ogs/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedSubscriptionResolver struct{ err error }

func (f failedSubscriptionResolver) ResolveSSETopics(context.Context) (int64, []string, []string, []string, error) {
	return 0, nil, nil, nil, f.err
}

func TestStaffEvents_TechnicalSetupFailures(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{errors.New("database unavailable"), context.DeadlineExceeded, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			runtime := Runtime[realtime.Event, *realtime.Client]{
				TokenDeadline: func(ctx context.Context) (context.Context, context.CancelFunc, bool) {
					ctx, cancel := context.WithCancel(ctx)
					return ctx, cancel, true
				},
				Identity:     func(context.Context) Identity { return Identity{} },
				WithinTenant: func(ctx context.Context, _ int64, fn func(context.Context) error) error { return fn(ctx) },
			}
			resource := NewResource[realtime.Event, *realtime.Client](nil, failedSubscriptionResolver{err: cause}, runtime, nil)
			rr := httptest.NewRecorder()
			resource.StaffEvents(rr, httptest.NewRequest(http.MethodGet, "/events", nil))
			require.Equal(t, http.StatusInternalServerError, rr.Code)
			var problem struct {
				Code   string `json:"code"`
				Status int    `json:"status"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
			assert.Equal(t, "general.server", problem.Code)
			assert.Equal(t, http.StatusInternalServerError, problem.Status)
			assert.NotContains(t, rr.Body.String(), "event: connected")
		})
	}
}

// A client that disconnects while the subscription resolves is no server
// fault: the stream answers 499, never 500 (Sentry BACKEND-1B).
func TestStaffEvents_ClientGoneDuringSetupIsNoServerError(t *testing.T) {
	t.Parallel()
	runtime := Runtime[realtime.Event, *realtime.Client]{
		TokenDeadline: func(ctx context.Context) (context.Context, context.CancelFunc, bool) {
			ctx, cancel := context.WithCancel(ctx)
			return ctx, cancel, true
		},
		Identity:     func(context.Context) Identity { return Identity{} },
		WithinTenant: func(ctx context.Context, _ int64, fn func(context.Context) error) error { return fn(ctx) },
	}
	ctx, cancel := context.WithCancel(t.Context())
	resolver := cancellingSubscriptionResolver{cancel: cancel}
	resource := NewResource[realtime.Event, *realtime.Client](nil, resolver, runtime, nil)
	rr := httptest.NewRecorder()
	resource.StaffEvents(rr, httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx))
	require.Equal(t, statusClientClosedRequest, rr.Code)
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
	assert.Equal(t, "general.unavailable", problem.Code)
}

// cancellingSubscriptionResolver plays the browser closing the tab while the
// topics are read: the request context ends and the read fails with it.
type cancellingSubscriptionResolver struct{ cancel context.CancelFunc }

func (c cancellingSubscriptionResolver) ResolveSSETopics(ctx context.Context) (int64, []string, []string, []string, error) {
	c.cancel()
	return 0, nil, nil, nil, ctx.Err()
}

// The school stream's role check fails with the request when the client goes
// away: 499, not 500. On a live request the failure stays a server error.
func TestSchoolAccessCheck_ClientGoneIsNoServerError(t *testing.T) {
	t.Parallel()
	for _, gone := range []bool{true, false} {
		ctx, cancel := context.WithCancel(t.Context())
		if gone {
			cancel()
		}
		resource := &Resource[realtime.Event, *realtime.Client]{schoolAccess: &stubSchoolAccess{err: context.Canceled}}
		rr := httptest.NewRecorder()
		require.False(t, resource.verifySchoolAccess(ctx, rr, 1, 1, slog.Default()))
		cancel()
		if gone {
			assert.Equal(t, statusClientClosedRequest, rr.Code)
		} else {
			assert.Equal(t, http.StatusInternalServerError, rr.Code)
		}
	}
}
