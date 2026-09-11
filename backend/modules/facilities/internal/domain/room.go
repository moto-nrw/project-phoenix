package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("room not found")
	ErrDuplicate          = errors.New("room already exists")
	ErrDuplicateToilet    = errors.New("toilet room already exists")
	ErrColorAlreadyInUse  = errors.New("room color already exists")
	ErrSystemProtected    = errors.New("system room is protected")
	ErrSystemNameReserved = errors.New("system room name is reserved")
)

const (
	SchulhofRoomName = "Schulhof"
	WCRoomName       = "WC"
	WCRoomAliasName  = "Toilette"

	RoomNameUniqueConstraint    = "idx_rooms_tenant_name"
	RoomColorUniqueConstraint   = "uniq_facilities_rooms_tenant_color"
	RoomWCAliasUniqueConstraint = "uniq_facilities_rooms_tenant_wc_alias"
)

type Room struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	Building   string
	Floor      *int
	Capacity   *int
	Category   *string
	Color      *string
	IsSystem   bool
	IsOpenRoom bool
}

type CreateRoom struct {
	Name     string
	Building string
	Floor    *int
	Capacity *int
	Category *string
	Color    *string
	IsSystem bool
	// IsOpenRoom releases the room for permanent use. Ordinary rooms are
	// created unreleased; only the auto-provisioned Schulhof sets this.
	IsOpenRoom bool
}

type UpdateRoom struct {
	ID       int64
	Name     string
	Building string
	Floor    *int
	Capacity *int
	Category *string
	Color    *string
	// IsOpenRoom is nil when the caller did not express an opinion, which
	// leaves the stored release untouched. A room's release is an explicit
	// administrative decision and must not be reset as a side effect of an
	// unrelated edit (renaming a room, picking a colour).
	IsOpenRoom *bool
}

type RoomFilter struct {
	Name             *string
	NameContains     *string
	Building         *string
	BuildingContains *string
	Floor            *int
	Category         *string
	MinimumCapacity  *int
	MaximumCapacity  *int
	Search           *string
	ExcludeSystem    bool
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
