package contract

import (
	"context"
	"errors"
	"sync"
	"time"
)

// The limits of reading the provider's block. Callers wait for it inside their
// transaction, so these limits are what bounds that wait.
const (
	ProviderBlockReads = 10
	ProviderBlockTime  = 3 * time.Second
)

// ErrProviderBlockBudgetSpent says a read did not happen. Its address counts as
// EmailProviderBlockUnreadable and nothing is sent on a guess.
var ErrProviderBlockBudgetSpent = errors.New("the budget for reading the provider's block is spent")

// ProviderBlockBudget counts the reads and the time spent waiting for the provider.
// Only that wait counts: other work in a long request does not use it up.
type ProviderBlockBudget struct {
	mu      sync.Mutex
	reads   int
	spent   time.Duration
	stopped bool
}

type providerBlockBudgetKey struct{}

// WithProviderBlockBudget makes everything under the context share one budget. A
// context that already has one keeps it.
func WithProviderBlockBudget(ctx context.Context) context.Context {
	if _, shared := ctx.Value(providerBlockBudgetKey{}).(*ProviderBlockBudget); shared {
		return ctx
	}
	return context.WithValue(ctx, providerBlockBudgetKey{}, &ProviderBlockBudget{})
}

// ProviderBlockBudgetOf is the context's shared budget, or a new one for one call.
func ProviderBlockBudgetOf(ctx context.Context) *ProviderBlockBudget {
	if budget, shared := ctx.Value(providerBlockBudgetKey{}).(*ProviderBlockBudget); shared {
		return budget
	}
	return &ProviderBlockBudget{}
}

// Read runs one read within what is left, or answers ErrProviderBlockBudgetSpent
// without running it. A read that outlasts its time ends the budget.
func (b *ProviderBlockBudget) Read(ctx context.Context, read func(context.Context) error) error {
	remaining, ok := b.take()
	if !ok {
		return ErrProviderBlockBudgetSpent
	}
	readCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	started := time.Now()
	err := read(readCtx)
	b.finish(time.Since(started), readCtx.Err() != nil)
	return err
}

// Stop ends the budget early, for example when the provider asks to slow down.
func (b *ProviderBlockBudget) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
}

func (b *ProviderBlockBudget) take() (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped || b.reads >= ProviderBlockReads || b.spent >= ProviderBlockTime {
		return 0, false
	}
	b.reads++
	return ProviderBlockTime - b.spent, true
}

func (b *ProviderBlockBudget) finish(took time.Duration, expired bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spent += took
	b.stopped = b.stopped || expired
}
