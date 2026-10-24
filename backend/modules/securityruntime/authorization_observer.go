package securityruntime

import (
	"context"
	"time"
)

type AuthorizationEvent struct {
	Outcome string
	Reason  string
	Elapsed time.Duration
}

type AuthorizationObserver func(AuthorizationEvent)

type authorizationObserverContextKey struct{}

// WithAuthorizationObserver attaches this request's observer without retaining
// transport objects. Authorization decisions report values through this slot.
func WithAuthorizationObserver(ctx context.Context, observer AuthorizationObserver) context.Context {
	return context.WithValue(ctx, authorizationObserverContextKey{}, observer)
}

// ObserveAuthorization reports a decision if this request has an observer.
func ObserveAuthorization(ctx context.Context, event AuthorizationEvent) {
	observer, _ := ctx.Value(authorizationObserverContextKey{}).(AuthorizationObserver)
	if observer != nil {
		observer(event)
	}
}
