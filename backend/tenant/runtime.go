package tenant

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

var ErrRuntimeRequired = errors.New("tenant: runtime is required")

type runtimeKey struct{}
type runtimeObserverKey struct{}
type transactionKey struct{}

func withTransaction(ctx context.Context, transaction any) context.Context {
	if uow, ok := ctx.Value(runtimeKey{}).(UnitOfWork); ok && uow.withTransaction != nil {
		ctx = uow.withTransaction(ctx, transaction)
	}
	return context.WithValue(ctx, transactionKey{}, transaction)
}

type UnitOfWorkEventKind string

const (
	UnitOfWorkTransaction   UnitOfWorkEventKind = "transaction"
	UnitOfWorkMissingTenant UnitOfWorkEventKind = "missing_tenant"
	UnitOfWorkPoolWait      UnitOfWorkEventKind = "pool_wait"
	UnitOfWorkLockWait      UnitOfWorkEventKind = "lock_wait"
)

type UnitOfWorkResult string

const (
	UnitOfWorkCommitted     UnitOfWorkResult = "commit"
	UnitOfWorkRolledBack    UnitOfWorkResult = "rollback"
	UnitOfWorkPanicked      UnitOfWorkResult = "panic"
	UnitOfWorkNotStarted    UnitOfWorkResult = "not_started"
	UnitOfWorkCommitUnknown UnitOfWorkResult = "commit_unknown"
)

type UnitOfWorkEvent struct {
	Kind     UnitOfWorkEventKind
	Result   UnitOfWorkResult
	Err      error
	Duration time.Duration
	Retries  int
}

// Compatibility names for callers migrating from the tenant-propagation
// runtime introduced by #2642. They are aliases, not a second implementation.
type RuntimeOutcome = UnitOfWorkEventKind
type RuntimeEvent = UnitOfWorkEvent

const (
	RuntimeTransaction   = UnitOfWorkTransaction
	RuntimeMissingTenant = UnitOfWorkMissingTenant
)

// UnitOfWork is the transaction lifecycle seam shared by HTTP and workers.
// Its functions are supplied by the composition root; this package does not
// know which database or transaction adapter backs them.
type UnitOfWork struct {
	withinTenant    func(context.Context, int64, func(context.Context, any) error) error
	withinAdmin     func(context.Context, func(context.Context, any) error) error
	savepoint       func(context.Context, SavepointAction) error
	retryable       func(error) bool
	acquireLock     func(context.Context, string, bool) error
	withoutTx       func(context.Context) context.Context
	withTenant      func(context.Context, int64) context.Context
	withTransaction func(context.Context, any) context.Context
}

// WithTransactionDetacher installs the persistence adapter operation used to
// mask its private transaction context. It keeps context-key ownership inside
// the adapter that reads the key.
func (uow UnitOfWork) WithTransactionDetacher(detach func(context.Context) context.Context) UnitOfWork {
	previous := uow.withoutTx
	uow.withoutTx = func(ctx context.Context) context.Context {
		if previous != nil {
			ctx = previous(ctx)
		}
		return detach(ctx)
	}
	return uow
}

func (uow UnitOfWork) WithContextAdapters(
	withTenant func(context.Context, int64) context.Context,
	withTransaction func(context.Context, any) context.Context,
) UnitOfWork {
	previousTenant := uow.withTenant
	uow.withTenant = func(ctx context.Context, tenantID int64) context.Context {
		if previousTenant != nil {
			ctx = previousTenant(ctx, tenantID)
		}
		return withTenant(ctx, tenantID)
	}
	previousTransaction := uow.withTransaction
	uow.withTransaction = func(ctx context.Context, transaction any) context.Context {
		if previousTransaction != nil {
			ctx = previousTransaction(ctx, transaction)
		}
		return withTransaction(ctx, transaction)
	}
	return uow
}

// SavepointAction is private to this package's UnitOfWork protocol; the
// transaction adapter never sees these values (see SavepointController).
type SavepointAction uint8

const (
	CreateSavepoint SavepointAction = iota + 1
	RollbackSavepoint
	ReleaseSavepoint
)

// SavepointController is what a transaction adapter implements. The three
// operations are named methods so the adapter shares no numeric constants
// with this package; the architecture policy forbids either side importing
// the other.
type SavepointController interface {
	CreateSavepoint(context.Context) error
	RollbackSavepoint(context.Context) error
	ReleaseSavepoint(context.Context) error
}

// SavepointFunc adapts a SavepointController to the function NewUnitOfWork takes.
func SavepointFunc(controller SavepointController) func(context.Context, SavepointAction) error {
	return func(ctx context.Context, action SavepointAction) error {
		switch action {
		case CreateSavepoint:
			return controller.CreateSavepoint(ctx)
		case RollbackSavepoint:
			return controller.RollbackSavepoint(ctx)
		case ReleaseSavepoint:
			return controller.ReleaseSavepoint(ctx)
		default:
			return fmt.Errorf("tenant: unknown savepoint action %d", action)
		}
	}
}

func NewUnitOfWork(
	withinTenant func(context.Context, int64, func(context.Context, any) error) error,
	withinAdmin func(context.Context, func(context.Context, any) error) error,
	savepoint func(context.Context, SavepointAction) error,
	retryable func(error) bool,
	acquireLocks ...func(context.Context, string, bool) error,
) (UnitOfWork, error) {
	if withinTenant == nil || withinAdmin == nil || savepoint == nil || retryable == nil {
		return UnitOfWork{}, fmt.Errorf("%w: transaction functions are required", ErrRuntimeRequired)
	}
	var acquireLock func(context.Context, string, bool) error
	if len(acquireLocks) > 0 {
		acquireLock = acquireLocks[0]
	}
	return UnitOfWork{withinTenant: withinTenant, withinAdmin: withinAdmin, savepoint: savepoint, retryable: retryable, acquireLock: acquireLock}, nil
}

func WithUnitOfWork(ctx context.Context, uow UnitOfWork) context.Context {
	return context.WithValue(ctx, runtimeKey{}, uow)
}

// WithUnitOfWorkObserver reports completed transactions to a composition seam
// without coupling this package to HTTP or worker metrics.
func WithUnitOfWorkObserver(ctx context.Context, observer func(UnitOfWorkEvent)) context.Context {
	return context.WithValue(ctx, runtimeObserverKey{}, observer)
}

func WithRuntimeObserver(ctx context.Context, observer func(RuntimeEvent)) context.Context {
	return WithUnitOfWorkObserver(ctx, observer)
}

func observeUnitOfWork(ctx context.Context, event UnitOfWorkEvent) {
	observer, _ := ctx.Value(runtimeObserverKey{}).(func(UnitOfWorkEvent))
	if observer != nil {
		observer(event)
	}
}

// ObserveMissingTenant reports an entry-point rejection that happens before a
// tenant runtime operation can start, such as an invalid tenant ID in a token.
func ObserveMissingTenant(ctx context.Context, err error) {
	observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkMissingTenant, Err: err})
}

// ObservePoolWait and ObserveLockWait let transaction adapters and repository
// lock helpers contribute timing evidence through the same entry-point
// observer as the UnitOfWork itself.
func ObservePoolWait(ctx context.Context, duration time.Duration) {
	observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkPoolWait, Duration: duration})
}

func ObserveLockWait(ctx context.Context, duration time.Duration) {
	observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkLockWait, Duration: duration})
}

func unitOfWorkFromContext(ctx context.Context) (UnitOfWork, error) {
	uow, ok := ctx.Value(runtimeKey{}).(UnitOfWork)
	if !ok || uow.withinTenant == nil || uow.withinAdmin == nil || uow.savepoint == nil || uow.retryable == nil {
		return UnitOfWork{}, ErrRuntimeRequired
	}
	return uow, nil
}

// TransactionFromContext returns the adapter-owned transaction attached to
// the active unit of work. Consumers may type-assert the value to their
// database driver's transaction type without coupling tenant to that driver.
func TransactionFromContext(ctx context.Context) (any, bool) {
	tx := ctx.Value(transactionKey{})
	return tx, tx != nil
}

// ContextWithoutTransaction masks an ambient adapter transaction while
// preserving cancellation, deadlines, tenant identity, and other values.
func ContextWithoutTransaction(ctx context.Context) context.Context {
	ctx = withTransaction(ctx, nil)
	if uow, ok := ctx.Value(runtimeKey{}).(UnitOfWork); ok && uow.withoutTx != nil {
		return uow.withoutTx(ctx)
	}
	return ctx
}

// AcquireLock delegates a transaction-scoped advisory lock to the active
// unit-of-work adapter.
func AcquireLock(ctx context.Context, key string, shared bool) error {
	uow, err := unitOfWorkFromContext(ctx)
	if err != nil {
		return err
	}
	if uow.acquireLock == nil {
		return fmt.Errorf("%w: advisory lock function is required", ErrRuntimeRequired)
	}
	return uow.acquireLock(ctx, key, shared)
}

// WithinTenant runs fn in the tenant transaction selected by id. It installs
// the validated tenant context before the transaction adapter can invoke fn.
func WithinTenant(ctx context.Context, id TenantID, fn func(context.Context) error) error {
	return withinTenant(ctx, id, false, fn)
}

// WithinTenantRetry runs an explicitly retry-safe outer command, replaying
// the whole transaction after a deadlock or serialization failure. A nested
// call joins the ambient transaction and never owns a retry.
func WithinTenantRetry(ctx context.Context, id TenantID, fn func(context.Context) error) error {
	return withinTenant(ctx, id, true, fn)
}

const maxTransactionRetries = 3

func (uow UnitOfWork) execute(ctx context.Context, retry bool, run func(context.Context) error) (err error) {
	if HasAfterCommitHooks(ctx) {
		return run(ctx)
	}

	started := time.Now()
	retries := 0
	committed := false
	defer func() {
		if panicValue := recover(); panicValue != nil {
			if !committed {
				observeTransaction(ctx, UnitOfWorkPanicked, nil, started, retries)
			}
			panic(panicValue)
		}
	}()

	for attempt := 0; ; attempt++ {
		attemptCtx, commitHooks := withAfterCommitHooks(ctx)
		err = run(attemptCtx)
		if err == nil {
			committed = true
			observeTransaction(ctx, UnitOfWorkCommitted, nil, started, retries)
			runAfterCommitHooks(commitHooks)
			return nil
		}
		if !retry || attempt == maxTransactionRetries || !uow.retryable(err) {
			observeTransaction(ctx, transactionResult(err), err, started, retries)
			return err
		}
		retries++
	}
}

func observeTransaction(ctx context.Context, result UnitOfWorkResult, err error, started time.Time, retries int) {
	observeUnitOfWork(ctx, UnitOfWorkEvent{
		Kind:     UnitOfWorkTransaction,
		Result:   result,
		Err:      err,
		Duration: time.Since(started),
		Retries:  retries,
	})
}

func transactionResult(err error) UnitOfWorkResult {
	var notStarted interface{ TransactionNotStarted() }
	if errors.As(err, &notStarted) {
		return UnitOfWorkNotStarted
	}
	var unknownCommit interface{ CommitOutcomeUnknown() }
	if errors.As(err, &unknownCommit) {
		return UnitOfWorkCommitUnknown
	}
	return UnitOfWorkRolledBack
}

func withinTenant(ctx context.Context, id TenantID, retry bool, fn func(context.Context) error) error {
	if id.IsZero() {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkMissingTenant, Err: ErrTenantRequired})
		return ErrTenantRequired
	}
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := unitOfWorkFromContext(ctx)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkTransaction, Result: UnitOfWorkNotStarted, Err: err})
		return err
	}

	if current, currentErr := TenantFromContext(ctx); currentErr == nil && current != id {
		return fmt.Errorf("tenant: nested transaction with mismatched tenant ID (%d vs %d)", current.Int64(), id.Int64())
	}

	scoped := WithTenant(ctx, id)
	return runtime.execute(scoped, retry, func(attemptCtx context.Context) error {
		return runtime.withinTenant(attemptCtx, id.Int64(), func(txCtx context.Context, tx any) error {
			return fn(withTransaction(txCtx, tx))
		})
	})
}

// WithinCurrentTenant runs fn for the validated tenant already in ctx.
func WithinCurrentTenant(ctx context.Context, fn func(context.Context) error) error {
	id, err := TenantFromContext(ctx)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkMissingTenant, Err: err})
		return err
	}
	return WithinTenant(ctx, id, fn)
}

// WithinAdmin runs fn in the cross-tenant administrative transaction.
func WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return withinAdmin(ctx, false, fn)
}

// WithinAdminRetry runs an explicitly retry-safe administrative command,
// replaying its whole transaction after a deadlock or serialization failure.
func WithinAdminRetry(ctx context.Context, fn func(context.Context) error) error {
	return withinAdmin(ctx, true, fn)
}

func withinAdmin(ctx context.Context, retry bool, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := unitOfWorkFromContext(ctx)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkTransaction, Err: err})
		return err
	}

	ctx = ContextWithoutTenant(ctx)
	return runtime.execute(ctx, retry, func(attemptCtx context.Context) error {
		return runtime.withinAdmin(attemptCtx, func(txCtx context.Context, tx any) error {
			txCtx = withTransaction(txCtx, tx)
			return fn(withAdminTxFlag(txCtx))
		})
	})
}

// WithTenantTx is the compatibility entry point for callers that still need
// the concrete transaction value. Database ownership stays behind UnitOfWork;
// the db argument is retained only while callers move to WithinTenant.
func WithTenantTx[DB, TX any](ctx context.Context, _ DB, rawID int64, fn func(context.Context, TX) error) error {
	id, err := NewTenantID(rawID)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkMissingTenant, Err: err})
		return err
	}
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := unitOfWorkFromContext(ctx)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkTransaction, Err: err})
		return err
	}
	if current, currentErr := TenantFromContext(ctx); currentErr == nil && current != id {
		return fmt.Errorf("tenant: nested transaction with mismatched tenant ID (%d vs %d)", current.Int64(), id.Int64())
	}

	scoped := WithTenant(ctx, id)
	return runtime.execute(scoped, false, func(attemptCtx context.Context) error {
		return runtime.withinTenant(attemptCtx, id.Int64(), func(txCtx context.Context, rawTX any) error {
			tx, ok := rawTX.(TX)
			if !ok {
				return fmt.Errorf("tenant: unit of work returned transaction type %T", rawTX)
			}
			return fn(withTransaction(txCtx, rawTX), tx)
		})
	})
}

// WithAdminTx is the compatibility entry point for callers that still need
// the concrete transaction value.
func WithAdminTx[DB, TX any](ctx context.Context, _ DB, fn func(context.Context, TX) error) error {
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := unitOfWorkFromContext(ctx)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkTransaction, Err: err})
		return err
	}
	ctx = ContextWithoutTenant(ctx)
	return runtime.execute(ctx, false, func(attemptCtx context.Context) error {
		return runtime.withinAdmin(attemptCtx, func(txCtx context.Context, rawTX any) error {
			tx, ok := rawTX.(TX)
			if !ok {
				return fmt.Errorf("tenant: unit of work returned transaction type %T", rawTX)
			}
			txCtx = withTransaction(txCtx, rawTX)
			return fn(withAdminTxFlag(txCtx), tx)
		})
	})
}

// WithAdminTxOrDirect keeps nil-database unit compositions usable without
// inventing an administrative transaction. A configured database always
// requires UnitOfWork; missing wiring never degrades to a direct call.
// An ambient admin transaction is reused; an ambient tenant transaction is
// rejected by the adapter, so callers reachable from a tenant request must
// check for an ambient transaction themselves before calling this.
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
