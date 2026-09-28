package presence

import (
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ===== Analytics Handlers =====

// getDashboardAnalytics handles getting dashboard analytics data
func (rs *Resource) getDashboardAnalytics(w http.ResponseWriter, r *http.Request) {
	// Get dashboard analytics
	analytics, err := rs.Operations.DashboardAnalytics(r.Context())
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
		StudentsHome:         analytics.StudentsHome,
		StudentsAtSchool:     analytics.StudentsAtSchool,
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

	response.ActiveGroupsSummary = activeGroupSummaries(analytics.ActiveGroupsSummary)

	common.Respond(w, r, http.StatusOK, response, "Dashboard analytics retrieved successfully")
}

// activeGroupSummaries maps the running sessions of the dashboard's
// "Laufende Betreuung" list onto the wire; never nil.
func activeGroupSummaries(groups []studentpresence.ActiveGroupInfo) []ActiveGroupSummary {
	result := make([]ActiveGroupSummary, 0, len(groups))
	for _, group := range groups {
		result = append(result, ActiveGroupSummary{
			Name:         group.Name,
			Type:         group.Type,
			StudentCount: group.StudentCount,
			MaxCapacity:  group.MaxCapacity,
			Location:     group.Location,
			Status:       group.Status,
		})
	}
	return result
}
