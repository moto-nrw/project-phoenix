package active

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// ===== Analytics Handlers =====

// getCounts handles getting various counts for analytics
func (rs *Resource) getCounts(w http.ResponseWriter, r *http.Request) {
	// Get active groups count
	activeGroupsCount, err := rs.ActiveService.GetActiveGroupsCount(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get total visits count
	totalVisitsCount, err := rs.ActiveService.GetTotalVisitsCount(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get active visits count
	activeVisitsCount, err := rs.ActiveService.GetActiveVisitsCount(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	response := AnalyticsResponse{
		ActiveGroupsCount: activeGroupsCount,
		TotalVisitsCount:  totalVisitsCount,
		ActiveVisitsCount: activeVisitsCount,
	}

	common.Respond(w, r, http.StatusOK, response, "Counts retrieved successfully")
}

// getRoomUtilization handles getting room utilization for analytics
func (rs *Resource) getRoomUtilization(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	roomID, err := common.ParseIDParam(r, "roomId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid room ID")))
		return
	}

	// Get room utilization
	utilization, err := rs.ActiveService.GetRoomUtilization(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	response := AnalyticsResponse{
		RoomUtilization: utilization,
	}

	common.Respond(w, r, http.StatusOK, response, "Room utilization retrieved successfully")
}

// getStudentAttendance handles getting student attendance rate for analytics
func (rs *Resource) getStudentAttendance(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStudentID)))
		return
	}

	// Get student attendance rate
	attendanceRate, err := rs.ActiveService.GetStudentAttendanceRate(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	response := AnalyticsResponse{
		AttendanceRate: attendanceRate,
	}

	common.Respond(w, r, http.StatusOK, response, "Student attendance rate retrieved successfully")
}

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
