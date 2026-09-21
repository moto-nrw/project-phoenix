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

// sharedOperatorSession lets the seed workers of the demo scheduler share one
// operator login (#3463). Every operator login drives the second factor, which
// is limited to a few codes per window; one login per demo school would lock
// the operator out as soon as several prospects arrive together.
type sharedOperatorSession struct {
	seedapi.Adapter
	now func() time.Time

	mu       sync.Mutex
	auth     seedapi.AuthRef
	expires  time.Time
	failedAt time.Time
	failure  error
}

func newSharedOperatorSession(adapter seedapi.Adapter) *sharedOperatorSession {
	return &sharedOperatorSession{Adapter: adapter, now: time.Now}
}

func (s *sharedOperatorSession) LoginOperator(ctx context.Context, email, password string) (seedapi.AuthRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.now().Before(s.expires) {
		return s.auth, nil
	}
	if s.pausedLocked() {
		return seedapi.AuthRef{}, s.failure
	}
	auth, err := s.Adapter.LoginOperator(ctx, email, password)
	if err != nil {
		s.failedAt, s.failure = s.now(), err
		return auth, err
	}
	s.auth, s.expires = auth, s.now().Add(demoOperatorSessionLifetime)
	return auth, nil
}

// forget makes the next seed sign in again.
func (s *sharedOperatorSession) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expires = time.Time{}
}

// paused reports a refused login the scheduler still waits out. No order can
// be seeded meanwhile, and none of them is to blame.
func (s *sharedOperatorSession) paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pausedLocked()
}

func (s *sharedOperatorSession) pausedLocked() bool {
	return s.failure != nil && s.now().Before(s.failedAt.Add(demoOperatorLoginPause))
}
