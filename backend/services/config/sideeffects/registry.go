// Package sideeffects holds the per-key handler registry used by the
// settings API after a setting value changes. Domains register their own
// handlers; the API wires one dispatcher.
package sideeffects

import (
	"context"
	"errors"
	"fmt"
)

// KeyHandler runs in the same tenant transaction as the setting write. The
// returned closure runs after commit and is meant for non-transactional work
// such as file deletion or broadcasts.
type KeyHandler func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)

type FailureObserver func(key string)

// Registry maps setting keys to their domain-owned handlers. Construct via
// NewRegistryWith, then pass Dispatch to the settings/operator resources.
type Registry struct {
	handlers       map[string]KeyHandler
	observeFailure FailureObserver
}

// Handler binds one setting key to its handler.
type Handler struct {
	Key    string
	Handle func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
}

// NewRegistryWith constructs a registry with its failure observer and every
// handler at construction (ADR 0059). A missing observer, a nil handler and
// a key bound twice are construction errors.
func NewRegistryWith(observer FailureObserver, handlers ...Handler) (*Registry, error) {
	if observer == nil {
		return nil, errors.New("sideeffects: failure observer is required")
	}
	bound := make(map[string]KeyHandler, len(handlers))
	for _, handler := range handlers {
		if err := checkHandler(bound, handler.Key, handler.Handle); err != nil {
			return nil, err
		}
		bound[handler.Key] = handler.Handle
	}
	return &Registry{handlers: bound, observeFailure: observer}, nil
}

func checkHandler(handlers map[string]KeyHandler, key string, handler KeyHandler) error {
	if handler == nil {
		return fmt.Errorf("sideeffects: nil handler for key %q", key)
	}
	if _, exists := handlers[key]; exists {
		return fmt.Errorf("sideeffects: handler already registered for key %q", key)
	}
	return nil
}

// Dispatch matches the settings resources' ValueSetCallback signature.
// Unknown keys are a no-op.
func (r *Registry) Dispatch(ctx context.Context, tenantID int64, key string, value any) (func(), error) {
	handler, ok := r.handlers[key]
	if !ok {
		return nil, nil
	}
	postCommit, err := handler(ctx, tenantID, value)
	if err != nil {
		r.observeFailure(key)
	}
	return postCommit, err
}
