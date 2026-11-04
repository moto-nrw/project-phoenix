package sse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"
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
