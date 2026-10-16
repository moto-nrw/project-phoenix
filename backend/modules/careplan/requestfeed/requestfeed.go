// Package requestfeed exposes a personal RSS subscription for new parent
// requests. The URL is a capability: callers only receive the raw secret when
// a subscription is created or rotated.
package requestfeed

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("request feed not found")
	ErrAlreadyActive = errors.New("request feed already active")
)

// FeedAccess is an account's current school-wide request permission in one
// school, with the school the feed names. The parent request feed projection
// supplies it (ADR 0058).
type FeedAccess struct {
	GeneralRequests    bool
	EnrollmentRequests bool
	SchoolName         string
	Subdomain          string
}

// FeedEntry is one new parent request the feed lists.
type FeedEntry struct {
	Kind      string
	ID        int64
	CreatedAt time.Time
}

// FeedSource reads the feed's facts in the caller's transaction. The root
// binds it to the parent request feed projection (ADR 0058).
type FeedSource interface {
	ResolveAccess(ctx context.Context, tenantID, accountID int64) (FeedAccess, bool, error)
	ListNewParentRequests(ctx context.Context, tenantID int64, since time.Time, generalRequests, enrollmentRequests bool) ([]FeedEntry, error)
}

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
