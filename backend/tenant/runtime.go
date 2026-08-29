package tenant

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var ErrRuntimeRequired = errors.New("tenant: runtime is required")

type runtimeKey struct{}
type runtimeObserverKey struct{}

type RuntimeOutcome string

const (
	RuntimeTransaction   RuntimeOutcome = "transaction"
	RuntimeMissingTenant RuntimeOutcome = "missing_tenant"
)

type RuntimeEvent struct {
	Outcome RuntimeOutcome
	Err     error
}

// Runtime is the tenant execution seam shared by HTTP requests and workers.
// Its functions are supplied by the composition root; the tenant package does
// not know which database or transaction implementation backs them.
type Runtime struct {
	withinTenant func(context.Context, int64, func(context.Context, any) error) error
	withinAdmin  func(context.Context, func(context.Context, any) error) error
	savepoint    func(context.Context, SavepointAction) error
}

type SavepointAction = uint8

const (
	CreateSavepoint SavepointAction = iota + 1
	RollbackSavepoint
	ReleaseSavepoint
)

func NewRuntime(
	withinTenant func(context.Context, int64, func(context.Context, any) error) error,
	withinAdmin func(context.Context, func(context.Context, any) error) error,
	savepoint func(context.Context, SavepointAction) error,
) (Runtime, error) {
	if withinTenant == nil || withinAdmin == nil || savepoint == nil {
		return Runtime{}, fmt.Errorf("%w: transaction functions are required", ErrRuntimeRequired)
	}
	return Runtime{withinTenant: withinTenant, withinAdmin: withinAdmin, savepoint: savepoint}, nil
}

func WithRuntime(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}

// WithRuntimeObserver reports completed runtime transactions to a composition
// boundary without coupling this package to HTTP or worker metrics.
func WithRuntimeObserver(ctx context.Context, observer func(RuntimeEvent)) context.Context {
	return context.WithValue(ctx, runtimeObserverKey{}, observer)
}

func observeRuntime(ctx context.Context, event RuntimeEvent) {
	observer, _ := ctx.Value(runtimeObserverKey{}).(func(RuntimeEvent))
	if observer != nil {
		observer(event)
	}
}

func runtimeFromContext(ctx context.Context) (Runtime, error) {
	runtime, ok := ctx.Value(runtimeKey{}).(Runtime)
	if !ok || runtime.withinTenant == nil || runtime.withinAdmin == nil || runtime.savepoint == nil {
		return Runtime{}, ErrRuntimeRequired
	}
	return runtime, nil
}

// WithinTenant runs fn in the tenant transaction selected by id. It installs
// the validated tenant context before the transaction adapter can invoke fn.
func WithinTenant(ctx context.Context, id TenantID, fn func(context.Context) error) error {
	if id.IsZero() {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeMissingTenant, Err: ErrTenantRequired})
		return ErrTenantRequired
	}
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := runtimeFromContext(ctx)
	if err != nil {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
		return err
	}

	if current, currentErr := TenantFromContext(ctx); currentErr == nil && current != id {
		return fmt.Errorf("tenant: nested transaction with mismatched tenant ID (%d vs %d)", current.Int64(), id.Int64())
	}

	ownsCommitHooks := !HasAfterCommitHooks(ctx)
	scoped := WithTenant(ctx, id)
	scoped, commitHooks := withAfterCommitHooks(scoped)
	err = runtime.withinTenant(scoped, id.Int64(), func(txCtx context.Context, _ any) error {
		return fn(txCtx)
	})
	if ownsCommitHooks {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
	}
	if err != nil {
		return err
	}
	if ownsCommitHooks {
		runAfterCommitHooks(commitHooks)
	}
	return nil
}

// WithinCurrentTenant runs fn for the validated tenant already in ctx.
func WithinCurrentTenant(ctx context.Context, fn func(context.Context) error) error {
	id, err := TenantFromContext(ctx)
	if err != nil {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeMissingTenant, Err: err})
		return err
	}
	return WithinTenant(ctx, id, fn)
}

// WithinAdmin runs fn in the cross-tenant administrative transaction.
func WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := runtimeFromContext(ctx)
	if err != nil {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
		return err
	}

	ownsCommitHooks := !HasAfterCommitHooks(ctx)
	ctx = ContextWithoutTenant(ctx)
	ctx, commitHooks := withAfterCommitHooks(ctx)
	err = runtime.withinAdmin(ctx, func(txCtx context.Context, _ any) error {
		return fn(withAdminTxFlag(txCtx))
	})
	if ownsCommitHooks {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
	}
	if err != nil {
		return err
	}
	if ownsCommitHooks {
		runAfterCommitHooks(commitHooks)
	}
	return nil
}

// WithTenantTx is the compatibility entry point for callers that still need
// the concrete transaction value. Database ownership stays behind Runtime;
// the db argument is retained only while callers move to WithinTenant.
func WithTenantTx[DB, TX any](ctx context.Context, _ DB, rawID int64, fn func(context.Context, TX) error) error {
	id, err := NewTenantID(rawID)
	if err != nil {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeMissingTenant, Err: err})
		return err
	}
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := runtimeFromContext(ctx)
	if err != nil {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
		return err
	}
	if current, currentErr := TenantFromContext(ctx); currentErr == nil && current != id {
		return fmt.Errorf("tenant: nested transaction with mismatched tenant ID (%d vs %d)", current.Int64(), id.Int64())
	}

	ownsCommitHooks := !HasAfterCommitHooks(ctx)
	scoped := WithTenant(ctx, id)
	scoped, commitHooks := withAfterCommitHooks(scoped)
	err = runtime.withinTenant(scoped, id.Int64(), func(txCtx context.Context, rawTX any) error {
		tx, ok := rawTX.(TX)
		if !ok {
			return fmt.Errorf("tenant: runtime returned transaction type %T", rawTX)
		}
		return fn(txCtx, tx)
	})
	if ownsCommitHooks {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
	}
	if err != nil {
		return err
	}
	if ownsCommitHooks {
		runAfterCommitHooks(commitHooks)
	}
	return nil
}

// WithAdminTx is the compatibility entry point for callers that still need
// the concrete transaction value.
func WithAdminTx[DB, TX any](ctx context.Context, _ DB, fn func(context.Context, TX) error) error {
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := runtimeFromContext(ctx)
	if err != nil {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
		return err
	}
	ownsCommitHooks := !HasAfterCommitHooks(ctx)
	ctx = ContextWithoutTenant(ctx)
	ctx, commitHooks := withAfterCommitHooks(ctx)
	err = runtime.withinAdmin(ctx, func(txCtx context.Context, rawTX any) error {
		tx, ok := rawTX.(TX)
		if !ok {
			return fmt.Errorf("tenant: runtime returned transaction type %T", rawTX)
		}
		return fn(withAdminTxFlag(txCtx), tx)
	})
	if ownsCommitHooks {
		observeRuntime(ctx, RuntimeEvent{Outcome: RuntimeTransaction, Err: err})
	}
	if err != nil {
		return err
	}
	if ownsCommitHooks {
		runAfterCommitHooks(commitHooks)
	}
	return nil
}

// WithAdminTxOrDirect keeps nil-database unit compositions usable without
// inventing an administrative transaction. A configured database always
// requires Runtime; missing runtime wiring never degrades to a direct call.
func WithAdminTxOrDirect[DB any](ctx context.Context, db DB, fn func(context.Context) error) error {
	if isNil(db) {
		if fn == nil {
			return errors.New("tenant: callback is required")
		}
		return fn(ContextWithoutTenant(ctx))
	}
	return WithinAdmin(ctx, fn)
}

func isNil[T any](value T) bool {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return true
	}
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
