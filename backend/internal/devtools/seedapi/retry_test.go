package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientRetriesRateLimitedRequests(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name string
		path string
		call func(*Client) error
	}{
		{"json", "/api/students", func(c *Client) error {
			_, err := c.Post("/api/students", map[string]string{"name": "Demo"})
			return err
		}},
		{"upload", "/api/files", func(c *Client) error {
			_, err := c.PostFile("/api/files", "file", "demo.txt", []byte("demo"))
			return err
		}},
		{"login", "/auth/login", func(c *Client) error { return c.Login("demo@example.test", "password", "demo") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				require.Equal(t, scenario.path, r.URL.Path)
				calls++
				if calls == 1 {
					w.Header().Set("Retry-After", "2")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = fmt.Fprint(w, `{"message":"Rate limit exceeded"}`)
					return
				}
				w.WriteHeader(http.StatusOK)
				if scenario.name == "login" {
					_, _ = fmt.Fprint(w, `{"data":{"access_token":"test-token"}}`)
				}
			})
			defer srv.Close()
			client := newTestClient(srv.URL, false)
			adapter := client.adapter.(*retryingAdapter)
			var delays []time.Duration
			adapter.sleep = func(_ context.Context, delay time.Duration) error {
				delays = append(delays, delay)
				return nil
			}
			require.NoError(t, scenario.call(client))
			require.Equal(t, 2, calls)
			require.Equal(t, []time.Duration{2 * time.Second}, delays)
		})
	}
}

func TestClientStopsAfterBoundedRateLimitRetries(t *testing.T) {
	t.Parallel()
	calls := 0
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, _ *seedHTTPRequest) {
		calls++
		w.Header().Set("Retry-After", "9999")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer srv.Close()
	client := newTestClient(srv.URL, false)
	adapter := client.adapter.(*retryingAdapter)
	var delays []time.Duration
	adapter.sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}
	_, err := client.Post("/api/students", nil)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.Equal(t, 1+maxRateLimitRetries, calls)
	require.Equal(t, []time.Duration{maxRetryDelay, maxRetryDelay, maxRetryDelay}, delays)
}

func TestClientDoesNotRetryOtherFailures(t *testing.T) {
	t.Parallel()
	calls := 0
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, _ *seedHTTPRequest) {
		calls++
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()
	client := newTestClient(srv.URL, false)
	client.adapter.(*retryingAdapter).sleep = func(context.Context, time.Duration) error {
		t.Fatal("unexpected retry")
		return nil
	}
	_, err := client.Post("/api/students", nil)
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestRetryStopsWhenWaitIsCancelled(t *testing.T) {
	t.Parallel()
	adapter := &retryingAdapter{sleep: func(context.Context, time.Duration) error { return context.Canceled }}
	calls := 0
	err := adapter.retry(context.Background(), func() error {
		calls++
		return &APIError{StatusCode: http.StatusTooManyRequests, RetryAfter: "2"}
	})
	require.True(t, errors.Is(err, context.Canceled))
	require.Equal(t, 1, calls)
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	require.Equal(t, 3*time.Second, retryDelay("3", now))
	require.Equal(t, 4*time.Second, retryDelay(now.Add(4*time.Second).Format(http.TimeFormat), now))
	require.Equal(t, time.Second, retryDelay("invalid", now))
	require.Equal(t, time.Second, retryDelay("0", now))
	require.Equal(t, maxRetryDelay, retryDelay("9999", now))
}
