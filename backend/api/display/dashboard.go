package display

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// DashboardResponse is the public dashboard wire shape. GDPR contract: it
// carries counts and room/activity metadata ONLY — never student names,
// student IDs, photos, or per-child pickup times. The screen rendering this
// hangs in a publicly visible entrance area.
type DashboardResponse struct {
	Status             string             `json:"status"` // always "active" (inactive displays never reach aggregation)
	SchoolName         string             `json:"school_name"`
	DisplayName        string             `json:"display_name"`
	ServerTime         string             `json:"server_time"` // RFC3339, Europe/Berlin
	Date               string             `json:"date"`        // YYYY-MM-DD
	RoomOccupancy      []RoomOccupancy    `json:"room_occupancy"`
	RunningActivities  []RunningActivity  `json:"running_activities"`
	UpcomingActivities []UpcomingActivity `json:"upcoming_activities"`
	PickupTimes        []PickupBucket     `json:"pickup_times"`
	Totals             DashboardTotals    `json:"totals"`
}

type RoomOccupancy struct {
	Name         string  `json:"name"`
	GroupName    *string `json:"group_name,omitempty"`
	CategoryName *string `json:"category_name,omitempty"`
	StudentCount int     `json:"student_count"`
	Capacity     *int    `json:"capacity,omitempty"`
	IsOccupied   bool    `json:"is_occupied"`
}

type RunningActivity struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	RoomName     string `json:"room_name"`
	Participants int    `json:"participants"`
	MaxCapacity  *int   `json:"max_capacity,omitempty"`
}

type UpcomingActivity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	StartTime string `json:"start_time"` // HH:MM wall clock
	RoomName  string `json:"room_name"`
}

type PickupBucket struct {
	Time  string `json:"time"` // HH:MM wall clock
	Count int    `json:"count"`
}

type DashboardTotals struct {
	StudentsPresent   int `json:"students_present"`
	RoomsOccupied     int `json:"rooms_occupied"`
	ActivitiesRunning int `json:"activities_running"`
}

func NewDashboardResponse(payload devicefleet.Dashboard) DashboardResponse {
	response := DashboardResponse{
		Status: payload.Status, SchoolName: payload.SchoolName, DisplayName: payload.DisplayName,
		ServerTime:         payload.ServerTime.Format(time.RFC3339),
		Date:               payload.Date.Format(time.DateOnly),
		RoomOccupancy:      make([]RoomOccupancy, 0, len(payload.RoomOccupancy)),
		RunningActivities:  make([]RunningActivity, 0, len(payload.RunningActivities)),
		UpcomingActivities: make([]UpcomingActivity, 0, len(payload.UpcomingActivities)),
		PickupTimes:        make([]PickupBucket, 0, len(payload.PickupTimes)),
		Totals: DashboardTotals{
			StudentsPresent:   payload.StudentsPresent,
			RoomsOccupied:     payload.RoomsOccupied,
			ActivitiesRunning: payload.ActivitiesRunning,
		},
	}
	for _, room := range payload.RoomOccupancy {
		response.RoomOccupancy = append(response.RoomOccupancy, RoomOccupancy{
			Name: room.Name, GroupName: room.GroupName, CategoryName: room.CategoryName,
			StudentCount: room.StudentCount, Capacity: room.Capacity, IsOccupied: room.IsOccupied,
		})
	}
	for _, running := range payload.RunningActivities {
		response.RunningActivities = append(response.RunningActivities, RunningActivity{
			ID: running.ID, Name: running.Name, Category: running.Category,
			RoomName: running.RoomName, Participants: running.Participants, MaxCapacity: running.MaxCapacity,
		})
	}
	for _, upcoming := range payload.UpcomingActivities {
		response.UpcomingActivities = append(response.UpcomingActivities, UpcomingActivity{
			ID: upcoming.ID, Name: upcoming.Name, Category: upcoming.Category,
			StartTime: upcoming.StartTime, RoomName: upcoming.RoomName,
		})
	}
	for _, bucket := range payload.PickupTimes {
		response.PickupTimes = append(response.PickupTimes, PickupBucket{Time: bucket.Time, Count: bucket.Count})
	}
	return response
}
