package active

import (
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// ===== Analytics Handlers =====

// getDashboardAnalytics handles getting dashboard analytics data
func (rs *Resource) getDashboardAnalytics(w http.ResponseWriter, r *http.Request) {
	// Get dashboard analytics
	analytics, err := rs.ActiveService.GetDashboardAnalytics(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	response := DashboardAnalyticsResponse{
		StudentsPresent:      analytics.StudentsPresent,
		StudentsInTransit:    analytics.StudentsInTransit,
		StudentsOnPlayground: analytics.StudentsOnPlayground,
		StudentsInRooms:      analytics.StudentsInRooms,
		StudentsSick:         analytics.StudentsSick,
		StudentsExcused:      analytics.StudentsExcused,
		ActiveActivities:     analytics.ActiveActivities,
		FreeRooms:            analytics.FreeRooms,
		TotalRooms:           analytics.TotalRooms,
		CapacityUtilization:  analytics.CapacityUtilization,
		ActivityCategories:   analytics.ActivityCategories,
		ActiveOGSGroups:      analytics.ActiveOGSGroups,
		StudentsInGroupRooms: analytics.StudentsInGroupRooms,
		SupervisorsToday:     analytics.SupervisorsToday,
		StudentsInHomeRoom:   analytics.StudentsInHomeRoom,
		RecentActivity:       make([]RecentActivityItem, 0),
		CurrentActivities:    make([]CurrentActivityItem, 0),
		ActiveGroupsSummary:  make([]ActiveGroupSummary, 0),
		LastUpdated:          time.Now(),
	}

	// Map recent activity
	for _, activity := range analytics.RecentActivity {
		response.RecentActivity = append(response.RecentActivity, RecentActivityItem{
			Type:      activity.Type,
			GroupName: activity.GroupName,
			RoomName:  activity.RoomName,
			Count:     activity.Count,
			Timestamp: activity.Timestamp,
		})
	}

	// Map current activities
	for _, activity := range analytics.CurrentActivities {
		response.CurrentActivities = append(response.CurrentActivities, CurrentActivityItem{
			ID:           strconv.FormatInt(activity.ID, 10),
			Name:         activity.Name,
			Category:     activity.Category,
			Participants: activity.Participants,
			MaxCapacity:  activity.MaxCapacity,
			Status:       activity.Status,
		})
	}

	// Map active groups summary
	for _, group := range analytics.ActiveGroupsSummary {
		response.ActiveGroupsSummary = append(response.ActiveGroupsSummary, ActiveGroupSummary{
			Name:         group.Name,
			Type:         group.Type,
			StudentCount: group.StudentCount,
			Location:     group.Location,
			Status:       group.Status,
		})
	}

	common.Respond(w, r, http.StatusOK, response, "Dashboard analytics retrieved successfully")
}
