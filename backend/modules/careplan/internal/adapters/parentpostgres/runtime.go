package parent

import (
	"context"

	"github.com/uptrace/bun"
)

// Runtime resolves the ambient database handle for parent cross-tenant work.
type Runtime interface {
	DB(context.Context) bun.IDB
}

// RuntimeFunc adapts composition-root database resolution.
type RuntimeFunc func(context.Context) bun.IDB

func (f RuntimeFunc) DB(ctx context.Context) bun.IDB { return f(ctx) }

func requireRuntime(runtime Runtime) Runtime {
	if runtime == nil {
		panic("parent repository: runtime is required")
	}
	return runtime
}
