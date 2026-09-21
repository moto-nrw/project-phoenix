package cmd

import (
	"context"
	"sync"
	"time"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

const (
	// demoOperatorSessionLifetime stays inside the 15 minute access token
	// lifetime, so a seed that starts with a shared session still finishes
	// with it. A deployment with shorter tokens loses a seed to an expired
	// session; the scheduler forgets the session on every failed seed, so the
	// repetition signs in again.
	demoOperatorSessionLifetime = 10 * time.Minute
	// demoOperatorLoginPause keeps a refused login from being repeated by
	// every waiting order: the second factor allows three codes in 15 minutes.
	demoOperatorLoginPause = time.Minute
)

// sharedLogin keeps one successful login for its lifetime and waits out a
// refused one, whoever asks.
type sharedLogin[T any] struct {
	now func() time.Time

	mu       sync.Mutex
	value    T
	expires  time.Time
	failedAt time.Time
	failure  error
}

func (s *sharedLogin[T]) login(fetch func() (T, error)) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.now().Before(s.expires) {
		return s.value, nil
	}
	if s.pausedLocked() {
		var none T
		return none, s.failure
	}
	value, err := fetch()
	if err != nil {
		s.failedAt, s.failure = s.now(), err
		return value, err
	}
	s.value, s.expires = value, s.now().Add(demoOperatorSessionLifetime)
	return value, nil
}

// forget makes the next caller sign in again.
func (s *sharedLogin[T]) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expires = time.Time{}
}

// paused reports a refused login that is still waited out.
func (s *sharedLogin[T]) paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pausedLocked()
}

func (s *sharedLogin[T]) pausedLocked() bool {
	return s.failure != nil && s.now().Before(s.failedAt.Add(demoOperatorLoginPause))
}

// sharedOperatorSession lets the seed workers of the demo scheduler share one
// operator login (#3463). Every operator login drives the second factor, which
// is limited to a few codes per window; one login per demo school would lock
// the operator out as soon as several prospects arrive together. While a
// login is refused no order can be seeded, and none of them is to blame.
type sharedOperatorSession struct {
	seedapi.Adapter
	*sharedLogin[seedapi.AuthRef]
}

func newSharedOperatorSession(adapter seedapi.Adapter) *sharedOperatorSession {
	return &sharedOperatorSession{Adapter: adapter, sharedLogin: &sharedLogin[seedapi.AuthRef]{now: time.Now}}
}

func (s *sharedOperatorSession) LoginOperator(ctx context.Context, email, password string) (seedapi.AuthRef, error) {
	return s.login(func() (seedapi.AuthRef, error) { return s.Adapter.LoginOperator(ctx, email, password) })
}
