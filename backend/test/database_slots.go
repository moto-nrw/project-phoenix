package test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	isolatedTestDatabaseOpenTimeout = 2 * time.Minute
	isolatedTestDatabaseDropTimeout = 30 * time.Second
	// Leave the existing open/drop budgets and time to report the failure before
	// Go's package alarm. Waiting does not consume the two-minute open budget.
	isolatedTestDatabaseDeadlineReserve = isolatedTestDatabaseOpenTimeout + isolatedTestDatabaseDropTimeout + 5*time.Second
)

func isolatedTestDatabaseSlotContext(tb testing.TB) (context.Context, context.CancelFunc, error) {
	deadlineTest, ok := tb.(interface{ Deadline() (time.Time, bool) })
	if !ok {
		return nil, nil, fmt.Errorf("isolated test database slot for %q requires a finite test deadline (-timeout)", tb.Name())
	}
	deadline, ok := deadlineTest.Deadline()
	if !ok {
		return nil, nil, fmt.Errorf("isolated test database slot for %q requires a finite test deadline (-timeout)", tb.Name())
	}
	ctx, cancel := context.WithDeadline(tb.Context(), deadline.Add(-isolatedTestDatabaseDeadlineReserve))
	return ctx, cancel, nil
}

type isolatedTestDatabaseLimiter struct {
	slots  chan struct{}
	mu     sync.Mutex
	owners map[string]int
	queued int
}

func newIsolatedTestDatabaseLimiter(limit int) *isolatedTestDatabaseLimiter {
	return &isolatedTestDatabaseLimiter{slots: make(chan struct{}, limit), owners: make(map[string]int)}
}

func (l *isolatedTestDatabaseLimiter) acquire(ctx context.Context, owner string) (func(), error) {
	started := time.Now()
	l.mu.Lock()
	l.queued++
	l.mu.Unlock()
	defer func() { l.mu.Lock(); l.queued--; l.mu.Unlock() }()
	if err := ctx.Err(); err != nil {
		return nil, l.waitError(owner, started, err)
	}
	select {
	case <-ctx.Done():
		return nil, l.waitError(owner, started, ctx.Err())
	case l.slots <- struct{}{}:
	}
	l.mu.Lock()
	l.owners[owner]++
	l.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			<-l.slots
			l.owners[owner]--
			if l.owners[owner] == 0 {
				delete(l.owners, owner)
			}
		})
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, l.waitError(owner, started, err)
	}
	return release, nil
}

func (l *isolatedTestDatabaseLimiter) waitError(requester string, started time.Time, err error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	owners := make([]string, 0, len(l.owners))
	for owner := range l.owners {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	// queue includes the failing requester until acquire returns.
	return fmt.Errorf("isolated test database slot: requester=%q owner=%q waited=%s queue=%d: %w",
		requester, strings.Join(owners, ", "), time.Since(started).Round(time.Millisecond), l.queued, err)
}
