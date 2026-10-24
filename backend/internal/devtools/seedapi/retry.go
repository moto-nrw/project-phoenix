package api

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	maxRateLimitRetries = 3
	maxRetryDelay       = 60 * time.Second
)

// retryingAdapter covers every HTTP operation the seed workflow performs,
// including logins and multipart uploads. Only rejected 429 requests are safe
// to replay; transport errors and other responses are never replayed.
type retryingAdapter struct {
	Adapter
	sleep func(context.Context, time.Duration) error
}

func newRetryingAdapter(adapter Adapter) Adapter {
	if adapter == nil {
		return nil
	}
	if _, ok := adapter.(*retryingAdapter); ok {
		return adapter
	}
	return &retryingAdapter{Adapter: adapter, sleep: sleepRetryDelay}
}

func sleepRetryDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *retryingAdapter) CheckHealth(ctx context.Context) error {
	return a.retry(ctx, func() error { return a.Adapter.CheckHealth(ctx) })
}

func (a *retryingAdapter) retry(ctx context.Context, request func() error) error {
	for attempt := 0; ; attempt++ {
		err := request()
		var apiErr *APIError
		if err == nil || attempt >= maxRateLimitRetries || !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
			return err
		}
		if waitErr := a.sleep(ctx, retryDelay(apiErr.RetryAfter, time.Now())); waitErr != nil {
			return waitErr
		}
	}
}

func retryDelay(header string, now time.Time) time.Duration {
	header = strings.TrimSpace(header)
	if seconds, err := strconv.ParseInt(header, 10, 64); err == nil && seconds > 0 {
		if seconds >= int64(maxRetryDelay/time.Second) {
			return maxRetryDelay
		}
		return time.Duration(seconds) * time.Second
	}
	for _, format := range []string{time.RFC1123, time.RFC850, time.ANSIC} {
		if until, err := time.Parse(format, header); err == nil {
			delay := until.Sub(now)
			if delay > maxRetryDelay {
				return maxRetryDelay
			}
			if delay > 0 {
				return delay
			}
		}
	}
	return time.Second
}

func (a *retryingAdapter) Raw(ctx context.Context, auth AuthRef, method, path string, body any, headers map[string]string) (raw []byte, status int, err error) {
	err = a.retry(ctx, func() error {
		raw, status, err = a.Adapter.Raw(ctx, auth, method, path, body, headers)
		return err
	})
	return
}

func (a *retryingAdapter) RawUpload(ctx context.Context, auth AuthRef, method, path, contentType string, body []byte) (raw []byte, status int, err error) {
	err = a.retry(ctx, func() error {
		raw, status, err = a.Adapter.RawUpload(ctx, auth, method, path, contentType, body)
		return err
	})
	return
}

func (a *retryingAdapter) LoginOperator(ctx context.Context, email, password string) (auth AuthRef, err error) {
	err = a.retry(ctx, func() error {
		auth, err = a.Adapter.LoginOperator(ctx, email, password)
		return err
	})
	return
}

func (a *retryingAdapter) LoginTenant(ctx context.Context, email, password, tenantSlug string) (auth AuthRef, err error) {
	err = a.retry(ctx, func() error {
		auth, err = a.Adapter.LoginTenant(ctx, email, password, tenantSlug)
		return err
	})
	return
}

func (a *retryingAdapter) LoginParent(ctx context.Context, email, password string) (auth AuthRef, err error) {
	err = a.retry(ctx, func() error {
		auth, err = a.Adapter.LoginParent(ctx, email, password)
		return err
	})
	return
}
