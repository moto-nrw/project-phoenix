package facilities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RoomRepository defines the interface for room repository operations
type RoomRepository interface {
	base.CRUDRepository[*Room]

	// FindByIDs retrieves rooms by their IDs.
	FindByIDs(ctx context.Context, ids []int64) ([]*Room, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Room, error)

	// FindByName retrieves a room by its name (case-insensitive).
	FindByName(ctx context.Context, name string) (*Room, error)

	// FindWithOccupancy returns the room plus its live occupancy aggregate.
	FindWithOccupancy(ctx context.Context, id int64) (*RoomOccupancyRow, error)

	// ListWithOccupancy returns rooms (optionally filtered) plus their live
	// occupancy aggregates, ordered by name.
	ListWithOccupancy(ctx context.Context, options *base.QueryOptions) ([]RoomOccupancyRow, error)

	// FindByCategory retrieves rooms by category
	FindByCategory(ctx context.Context, category string) ([]*Room, error)
}

// RoomOccupancyRow is the room + live-occupancy projection returned by the
// occupancy lookups; the service maps it onto its RoomWithOccupancy view.
type RoomOccupancyRow struct {
	ID        int64     `bun:"id"`
	Name      string    `bun:"name"`
	Building  string    `bun:"building"`
	Floor     *int      `bun:"floor"`
	Capacity  *int      `bun:"capacity"`
	Category  *string   `bun:"category"`
	Color     *string   `bun:"color"`
	CreatedAt time.Time `bun:"created_at"`
	UpdatedAt time.Time `bun:"updated_at"`

	IsOccupied   bool    `bun:"is_occupied"`
	GroupName    *string `bun:"group_name"`
	CategoryName *string `bun:"category_name"`
	StudentCount int     `bun:"student_count"`
	// SupervisorStaffIDs are the staff members currently supervising in the
	// room, the only supervision fact the repository reads itself.
	SupervisorStaffIDs []int64 `bun:"supervisor_staff_ids,array"`
	// SupervisorPersonIDs are the persons behind those staff members. The
	// composition layer fills them through School Membership and turns them
	// into SupervisorNames through the People Directory; the repository
	// never reads staff rows or person names itself.
	SupervisorPersonIDs []int64 `bun:"-"`
	SupervisorNames     *string `bun:"supervisor_names,scanonly"`
}
