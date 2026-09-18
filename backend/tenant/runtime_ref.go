package tenant

import (
	"context"
	"fmt"
)

// A module that opens its own transactions is composed before the root knows
// the unit of work: the runtime is built from the database handle the same
// root wires afterwards. RuntimeRef is the reference that closes that gap —
// the composition captures Attach once and the root binds the runtime when it
// has one, so no composed graph carries a mutable runtime field of its own
// (#3364).
type RuntimeRef struct {
	runtime  *UnitOfWork
	fallback func(context.Context) context.Context
}

// NewRuntimeRef creates a reference. fallback is used until a runtime is
// bound and may be nil, which leaves the context untouched.
func NewRuntimeRef(fallback func(context.Context) context.Context) *RuntimeRef {
	return &RuntimeRef{fallback: fallback}
}

// Bind wires the unit of work; the latest binding wins.
func (r *RuntimeRef) Bind(runtime UnitOfWork) {
	if r == nil {
		return
	}
	r.runtime = &runtime
}

// BindRuntime binds a unit of work handed over as an opaque value, for a
// consumer whose own contract must not name this package. The root supplies
// its UnitOfWork; any other value is a composition mistake and is reported
// rather than silently ignored.
func (r *RuntimeRef) BindRuntime(runtime any) error {
	unit, ok := runtime.(UnitOfWork)
	if !ok {
		return fmt.Errorf("%w: cannot bind %T", ErrRuntimeRequired, runtime)
	}
	r.Bind(unit)
	return nil
}

// Attach hands the bound runtime to a context, the fallback when none is
// bound, and the context unchanged when there is neither.
func (r *RuntimeRef) Attach(ctx context.Context) context.Context {
	if r == nil {
		return ctx
	}
	if r.runtime != nil {
		return WithUnitOfWork(ctx, *r.runtime)
	}
	if r.fallback != nil {
		return r.fallback(ctx)
	}
	return ctx
}
