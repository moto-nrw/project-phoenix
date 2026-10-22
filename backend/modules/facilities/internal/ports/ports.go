package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
)

type Store interface {
	Create(context.Context, facilities.CreateRoom) (facilities.Room, domain.OperationStats, error)
	Update(context.Context, facilities.UpdateRoom) (facilities.Room, domain.OperationStats, error)
	Delete(context.Context, int64) (domain.OperationStats, error)
	FindByID(context.Context, int64, string) (facilities.Room, bool, domain.OperationStats, error)
	FindByName(context.Context, string) (facilities.Room, bool, domain.OperationStats, error)
	FindToilet(context.Context, int64) (facilities.Room, bool, domain.OperationStats, error)
	List(context.Context, facilities.RoomFilter) ([]facilities.Room, domain.OperationStats, error)
	ListPage(context.Context, facilities.RoomFilter, int, int) ([]facilities.Room, int, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]facilities.Room, domain.OperationStats, error)
	LockByIDs(context.Context, []int64) ([]facilities.Room, domain.OperationStats, error)
}

type Transaction interface {
	RunWrite(context.Context, func(context.Context) error) error
	AcquireLock(context.Context, string) error
}

// RoomUsage answers the two questions a room deletion asks other owners: is
// a session running in the room, and does a care offering need it. Facilities
// owns the port and the decision; the composition root binds the answers to
// Student Presence and Care Plan.
type RoomUsage interface {
	HasActiveSessions(ctx context.Context, roomID int64) (bool, error)
	RequiredByCareOffering(ctx context.Context, roomID int64) (bool, error)
}

type DeletionLock func(context.Context) error

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)

// CategoryStore persists the room category catalog of the caller's school.
type CategoryStore interface {
	List(context.Context) ([]facilities.RoomCategory, error)
	FindForUpdate(context.Context, int64) (facilities.RoomCategory, bool, error)
	Create(context.Context, facilities.RoomCategoryInput) (facilities.RoomCategory, error)
	Update(context.Context, int64, facilities.RoomCategoryInput) (facilities.RoomCategory, error)
	Delete(context.Context, int64) error
	RenameRooms(ctx context.Context, from, to string) error
	// CountRoomsByCategory counts the ordinary rooms per category name,
	// keyed by the lower-case trimmed name.
	CountRoomsByCategory(context.Context) (map[string]int, error)
}
