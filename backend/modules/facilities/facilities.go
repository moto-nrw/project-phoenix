// Package facilities is the public Facilities capability. It owns
// facilities.rooms and answers room lookups for every other owner without
// leaking the table into their repositories.
package facilities

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRoomNotFound = errors.New("room not found")
	ErrInvalidRoom  = errors.New("invalid room")
)

// Room is the facilities view of one physical room.
type Room struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	Building  string    `json:"building,omitempty"`
	Floor     *int      `json:"floor,omitempty"`
	Capacity  *int      `json:"capacity,omitempty"`
	Category  *string   `json:"category,omitempty"`
	Color     *string   `json:"color,omitempty"`
	IsSystem  bool      `json:"is_system"`
}

// Query is the read seam every foreign reader of facilities.rooms uses.
type Query interface {
	FindRoom(context.Context, int64) (Room, error)
	ListRoomsByID(context.Context, []int64) ([]Room, error)
}

type Capability interface {
	Query
}

type engine interface {
	FindRoom(context.Context, int64) (Room, error)
	ListRoomsByID(context.Context, []int64) ([]Room, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("facilities: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindRoom(ctx context.Context, id int64) (Room, error) {
	if id <= 0 {
		return Room{}, ErrInvalidRoom
	}
	return m.engine.FindRoom(ctx, id)
}

// ListRoomsByID returns the rooms visible in the caller's transaction for
// the given IDs, sorted by name. Missing IDs are simply absent: consumers
// rendering a room name treat absence like the former LEFT JOIN.
func (m *Module) ListRoomsByID(ctx context.Context, ids []int64) ([]Room, error) {
	if len(ids) == 0 {
		return []Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidRoom
		}
	}
	return m.engine.ListRoomsByID(ctx, ids)
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrRoomNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidRoom):
		return "invalid"
	default:
		return "internal_error"
	}
}
