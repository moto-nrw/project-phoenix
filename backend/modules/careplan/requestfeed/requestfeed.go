// Package requestfeed exposes a personal RSS subscription for new parent
// requests. The URL is a capability: callers only receive the raw secret when
// a subscription is created or rotated.
package requestfeed

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("request feed not found")
	ErrAlreadyActive = errors.New("request feed already active")
)

type Status struct {
	Active bool
}

type Created struct {
	URL string
}

type Feed struct {
	XML string
}

type engine interface {
	Status(context.Context, int64, int64) (Status, error)
	Provision(context.Context, int64, int64) (Created, error)
	Rotate(context.Context, int64, int64) (Created, error)
	ByToken(context.Context, string) (Feed, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("request feed: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) Status(ctx context.Context, tenantID, accountID int64) (Status, error) {
	if tenantID <= 0 || accountID <= 0 {
		return Status{}, ErrNotFound
	}
	return m.engine.Status(ctx, tenantID, accountID)
}

func (m *Module) Provision(ctx context.Context, tenantID, accountID int64) (Created, error) {
	if tenantID <= 0 || accountID <= 0 {
		return Created{}, ErrNotFound
	}
	return m.engine.Provision(ctx, tenantID, accountID)
}

func (m *Module) Rotate(ctx context.Context, tenantID, accountID int64) (Created, error) {
	if tenantID <= 0 || accountID <= 0 {
		return Created{}, ErrNotFound
	}
	return m.engine.Rotate(ctx, tenantID, accountID)
}

func (m *Module) ByToken(ctx context.Context, token string) (Feed, error) {
	if token == "" {
		return Feed{}, ErrNotFound
	}
	return m.engine.ByToken(ctx, token)
}
