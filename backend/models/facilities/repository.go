package facilities

import (
	"context"
	"time"
)

// RoomRepository is the temporary compatibility contract for legacy callers.
// Its implementation delegates every room read and write to the owner facade.
type RoomRepository interface {
	Create(context.Context, *Room) error
	FindByID(context.Context, any) (*Room, error)
	Update(context.Context, *Room) error
	Delete(context.Context, any) error
	List(context.Context, map[string]any) ([]*Room, error)
	FindByIDs(context.Context, []int64) ([]*Room, error)
	FindByIDForUpdate(context.Context, int64) (*Room, error)
	FindByName(context.Context, string) (*Room, error)
	FindByCategory(context.Context, string) ([]*Room, error)
}

// RoomOccupancyRow remains a compatibility DTO while live presence moves to
// its own projection. It contains no room persistence behavior.
type RoomOccupancyRow struct {
	ID                  int64
	Name                string
	Building            string
	Floor               *int
	Capacity            *int
	Category            *string
	Color               *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	IsOccupied          bool
	GroupName           *string
	CategoryName        *string
	StudentCount        int
	SupervisorStaffIDs  []int64
	SupervisorPersonIDs []int64
	SupervisorNames     *string
}
