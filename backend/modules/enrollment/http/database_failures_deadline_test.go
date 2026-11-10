package enrollmenthttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// A context's deadline and its timer callback are separate observations. Keep
// cancellation pending to force the socket timer to win, as it can under load.
type pendingDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c pendingDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

func TestDatabaseFailureAfterRequestDeadlineBeforeContextTimer(t *testing.T) {
	t.Parallel()
	ctx := pendingDeadlineContext{Context: context.Background(), deadline: time.Now().Add(-time.Second)}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	require.NoError(t, client.SetReadDeadline(ctx.deadline))
	_, err := client.Read(make([]byte, 1))
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	require.NoError(t, ctx.Err(), "the context timer has not run")
	err = errors.Join(err, tenant.SavepointRollbackFailure(tenant.ErrSavepointControl))
	require.Equal(t, all(http.StatusRequestTimeout), statusMappings(t, ctx, err))
}

func TestDatabaseFailureRequestDeadlineBoundaries(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	rollbackErr := errors.Join(os.ErrDeadlineExceeded, tenant.SavepointRollbackFailure(tenant.ErrSavepointControl))
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want int
	}{
		{"future_request_deadline", pendingDeadlineContext{Context: context.Background(), deadline: time.Now().Add(time.Hour)}, http.StatusInternalServerError},
		{"no_request_deadline", context.Background(), http.StatusInternalServerError},
		{"cancelled_before_expired_deadline", pendingDeadlineContext{Context: cancelled, deadline: time.Now().Add(-time.Second)}, 499},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, all(tc.want), statusMappings(t, tc.ctx, rollbackErr))
		})
	}
	expired := pendingDeadlineContext{Context: context.Background(), deadline: time.Now().Add(-time.Second)}
	require.Nil(t, statusFailures().requestEnded(httptest.NewRequestWithContext(expired, http.MethodPost, "/", nil), errors.New("business refusal")), "elapsed request time does not turn a business refusal into a timeout")
}
