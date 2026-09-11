// Package sideeffects holds the per-key handler registry used by the
// settings API after a setting value changes. Domains register their own
// handlers; the API wires one dispatcher.
package sideeffects

import (
	"context"
	"fmt"
)

// KeyHandler runs in the same tenant transaction as the setting write. The
// returned closure runs after commit and is meant for non-transactional work
// such as file deletion or broadcasts.
type KeyHandler func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)

type FailureObserver func(key string)

// Registry maps setting keys to their domain-owned handlers. Construct via
// NewRegistry, populate at boot via Register, then pass Dispatch to the
// settings/operator resources.
type Registry struct {
	handlers       map[string]KeyHandler
	observeFailure FailureObserver
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]KeyHandler)}
}

func (r *Registry) SetFailureObserver(observer FailureObserver) {
	r.observeFailure = observer
}

// Register binds a handler to a setting key.
func (r *Registry) Register(key string, handler KeyHandler) {
	if handler == nil {
		panic(fmt.Sprintf("sideeffects: nil handler for key %q", key))
	}
	if _, exists := r.handlers[key]; exists {
		panic(fmt.Sprintf("sideeffects: handler already registered for key %q", key))
	}
	r.handlers[key] = handler
}

// Dispatch matches the settings resources' ValueSetCallback signature.
// Unknown keys are a no-op.
func (r *Registry) Dispatch(ctx context.Context, tenantID int64, key string, value any) (func(), error) {
	handler, ok := r.handlers[key]
	if !ok {
		return nil, nil
	}
	postCommit, err := handler(ctx, tenantID, value)
	if err != nil && r.observeFailure != nil {
		r.observeFailure(key)
	}
	return postCommit, err
}
