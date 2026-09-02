// Package schoolstructure is the public School Structure capability. It owns
// education.groups and answers class and group structure questions for every
// other owner without leaking the table into their repositories.
package schoolstructure

import (
	"context"
	"errors"
	"time"
)

var (
	ErrGroupNotFound = errors.New("group not found")
	ErrInvalidGroup  = errors.New("invalid group")
)

// Group is the structure view of one education group (Klasse/Gruppe).
type Group struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	RoomID    *int64    `json:"room_id,omitempty"`
}

// Query is the read seam every foreign reader of education.groups uses.
type Query interface {
	FindGroup(context.Context, int64) (Group, error)
	ListGroupsByID(context.Context, []int64) ([]Group, error)
}

type Capability interface {
	Query
}

type engine interface {
	FindGroupByID(context.Context, int64) (Group, error)
	ListGroupsByID(context.Context, []int64) ([]Group, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("school structure: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindGroup(ctx context.Context, id int64) (Group, error) {
	if id <= 0 {
		return Group{}, ErrInvalidGroup
	}
	return m.engine.FindGroupByID(ctx, id)
}

// ListGroupsByID returns the groups visible in the caller's transaction for
// the given IDs, sorted by name. Missing IDs are simply absent: consumers
// rendering a display name treat absence like the former LEFT JOIN.
func (m *Module) ListGroupsByID(ctx context.Context, ids []int64) ([]Group, error) {
	if len(ids) == 0 {
		return []Group{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidGroup
		}
	}
	return m.engine.ListGroupsByID(ctx, ids)
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrGroupNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidGroup):
		return "invalid"
	default:
		return "internal_error"
	}
}
