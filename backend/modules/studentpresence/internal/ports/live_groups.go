package ports

import "time"

type StaffRoomSupervision struct{ StaffID, RoomID int64 }

type LiveGroupFilter struct {
	EndedOnly          bool
	Limit, Offset      int
	DeviceManagedOnly  bool
	LastActivityBefore *time.Time
	IDs                []int64
	RoomID             *int64
	DeviceID           *int64
	ActivityGroupIDs   []int64
	OpenOnly           bool
	From, Until        *time.Time
}

type GroupSupervision struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role                           string
	StartDate                      Date
	EndDate                        *Date
}

type GroupSupervisionFilter struct {
	StaffIDs      []int64
	EndedBy       *Date
	Limit, Offset int
	IDs           []int64
	OpenOnly      bool
	StartedBefore *Date
	// ForUpdate requires a caller-owned tenant transaction.
	ForUpdate bool
	GroupIDs  []int64
	StaffID   *int64
	ActiveOn  *Date
}
