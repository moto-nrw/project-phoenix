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
	ErrGroupNotFound  = errors.New("group not found")
	ErrInvalidGroup   = errors.New("invalid group")
	ErrInvalidStudent = errors.New("invalid student")
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
	GroupListing
}

// GroupListing supplies bounded name-matching candidates to import workflows.
type GroupListing interface {
	ListGroups(context.Context, int) ([]Group, error)
}

// StudentTransitionHistory is the School Structure half of a permanent child
// deletion (#2710): education.grade_transition_history keeps a denormalized
// copy of the child's name without a foreign key, so the owner counts and
// anonymizes those rows inside the deletion's transaction.
type StudentTransitionHistory interface {
	// CountStudentTransitionHistory counts the ledger rows that still carry
	// the child's name; anonymized rows are retained history, not impact.
	CountStudentTransitionHistory(context.Context, int64) (int, error)
	// AnonymizeStudentTransitionHistory replaces the stored name with the
	// placeholder and clears the stored RFID tag on every ledger row of the
	// child. It returns the number of rows changed.
	AnonymizeStudentTransitionHistory(context.Context, int64) (int64, error)
}

// PurgedStudentPlaceholder is the name the ledger keeps for a deleted child.
const PurgedStudentPlaceholder = "Gelöschtes Kind"

type engine interface {
	FindGroup(context.Context, int64) (Group, error)
	ListGroupsByID(context.Context, []int64) ([]Group, error)
	ListGroups(context.Context, int) ([]Group, error)
	StudentTransitionHistory
}

func (m *Module) CountStudentTransitionHistory(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, ErrInvalidStudent
	}
	return m.engine.CountStudentTransitionHistory(ctx, studentID)
}

func (m *Module) AnonymizeStudentTransitionHistory(ctx context.Context, studentID int64) (int64, error) {
	if studentID <= 0 {
		return 0, ErrInvalidStudent
	}
	return m.engine.AnonymizeStudentTransitionHistory(ctx, studentID)
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
	return m.engine.FindGroup(ctx, id)
}

// ListGroups returns at most limit groups of the explicit tenant in context.
func (m *Module) ListGroups(ctx context.Context, limit int) ([]Group, error) {
	if limit <= 0 || limit > 1000 {
		return nil, ErrInvalidGroup
	}
	return m.engine.ListGroups(ctx, limit)
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
