package facilities

// RoomWithOccupancy is the tenant-safe room projection used by room and kiosk
// selectors. It carries room facts and the current occupancy labels, not
// persistence models or write capabilities from the contributing owners.
type RoomWithOccupancy struct {
	*Room
	IsOccupied      bool    `json:"is_occupied"`
	GroupName       *string `json:"group_name,omitempty"`
	CategoryName    *string `json:"category_name,omitempty"`
	StudentCount    int     `json:"student_count"`
	SupervisorNames *string `json:"supervisor_names,omitempty"`
}
