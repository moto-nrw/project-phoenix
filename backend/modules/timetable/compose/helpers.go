package compose

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/activities"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
)

// =============================================================================
// HELPER METHODS - Reduce code duplication for common parsing/validation
// =============================================================================

// getStaffIDAndManagePermission extracts the current staff ID and checks for admin-level permission.
// Returns (staffID, hasAdminPermission, error). If user is not staff, returns (0, hasAdminPermission, error).
// Note: Only AdminWildcard and FullAccess bypass ownership checks, NOT "activities:manage".
// "activities:manage" allows creating/editing activities, but ownership rules still apply.
func (rs *Resource) getStaffIDAndManagePermission(r *http.Request) (int64, bool, error) {
	// Check if user has admin-level permission that bypasses ownership checks
	// Note: "activities:manage" does NOT bypass ownership - only true admin permissions do
	hasAdminPermission := authorize.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context()))

	// Get current staff
	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil {
		// User is not staff - they can only have manage permission if admin
		return 0, hasAdminPermission, err
	}

	return staff.ID, hasAdminPermission, nil
}

// requireActivityModification applies the same owner/supervisor/admin policy
// used by the activity update and delete handlers.
func (rs *Resource) requireActivityModification(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activityID, err := common.ParseID(r)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
			return
		}

		staffID, hasAdminPermission, err := rs.getStaffIDAndManagePermission(r)
		if err != nil && !hasAdminPermission {
			common.RenderError(w, r, common.ErrorForbidden(activitiesSvc.ErrNotOwner))
			return
		}

		allowed, err := rs.ActivityService.CanModifyActivity(r.Context(), activityID, staffID, hasAdminPermission)
		if err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
		if !allowed {
			common.RenderError(w, r, common.ErrorForbidden(activitiesSvc.ErrNotOwner))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// parseAndGetActivity parses activity ID from URL and returns the activity if it exists.
// Returns nil and false if parsing fails or activity doesn't exist (error already rendered).
func (rs *Resource) parseAndGetActivity(w http.ResponseWriter, r *http.Request) (*activities.Group, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidActivityID)))
		return nil, false
	}

	activity, err := rs.ActivityService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return nil, false
	}

	return activity, true
}

// newCategoryResponse converts a category model to a response object
func newCategoryResponse(category *activities.Category) CategoryResponse {
	// Handle nil category input
	if category == nil {
		slog.Default().Warn("Attempted to create CategoryResponse from nil category")
		return CategoryResponse{
			Name: "Unknown Category", // Provide a safe default
		}
	}

	return CategoryResponse{
		ID:          category.ID,
		Name:        category.Name,
		Description: category.Description,
		Color:       category.Color,
		ShiftTypeID: category.ShiftTypeID,
		IsSystem:    category.IsSystem,
		ArchivedAt:  category.ArchivedAt,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

// newActivityResponse converts an activity group model to a response object
func newActivityResponse(group *activities.Group, enrollmentCount int) ActivityResponse {
	// Check if group is nil to prevent panic
	if group == nil {
		slog.Default().Error("Attempted to create ActivityResponse from nil group")
		// Return empty response rather than panic
		return ActivityResponse{}
	}

	// Create response with only the direct fields that are guaranteed to be safe
	response := ActivityResponse{
		ID:              group.ID,
		Name:            group.Name,
		MaxParticipants: group.ParticipantLimit(),
		IsOpen:          group.IsOpen,
		CategoryID:      group.CategoryID,
		CreatedBy:       group.CreatedBy,
		EnrollmentCount: enrollmentCount,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
		Schedules:       []ScheduleResponse{},
	}

	// Add creator name if available
	if group.CreatedByStaff != nil && group.CreatedByStaff.Person != nil {
		response.CreatedByName = group.CreatedByStaff.Person.FirstName + " " + group.CreatedByStaff.Person.LastName
	}

	// Safely add optional fields with nil checks
	if group.PlannedRoomID != nil {
		response.PlannedRoomID = group.PlannedRoomID
	}

	// Add category details if available - with extra nil checks
	if group.Category != nil {
		category := newCategoryResponse(group.Category)
		response.Category = &category
	}

	// Add schedules if available - with thorough nil checking
	if group.Schedules != nil {
		scheduleResponses := make([]ScheduleResponse, 0, len(group.Schedules))
		for _, schedule := range group.Schedules {
			if schedule == nil {
				slog.Default().Warn("Nil schedule encountered", slog.Int64("group_id", group.ID))
				continue
			}
			scheduleResponses = append(scheduleResponses, newScheduleResponse(schedule))
		}
		if len(scheduleResponses) > 0 {
			response.Schedules = scheduleResponses
		}
	}

	return response
}

// =============================================================================
// RESPONSE CONVERSION HELPERS - Reduce duplication in response creation
// =============================================================================

// newScheduleResponse converts a schedule model to a response object.
func newScheduleResponse(schedule *activities.Schedule) ScheduleResponse {
	if schedule == nil {
		return ScheduleResponse{}
	}
	return ScheduleResponse{
		ID:              schedule.ID,
		Weekday:         schedule.Weekday,
		TimeframeID:     schedule.TimeframeID,
		ActivityGroupID: schedule.ActivityGroupID,
		CreatedAt:       schedule.CreatedAt,
		UpdatedAt:       schedule.UpdatedAt,
	}
}

// newSupervisorResponse converts a supervisor model to a response object with staff details.
func newSupervisorResponse(supervisor *activities.SupervisorPlanned) SupervisorResponse {
	if supervisor == nil {
		return SupervisorResponse{}
	}
	resp := SupervisorResponse{
		ID:        supervisor.ID,
		StaffID:   supervisor.StaffID,
		IsPrimary: supervisor.IsPrimary,
	}
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		resp.FirstName = supervisor.Staff.Person.FirstName
		resp.LastName = supervisor.Staff.Person.LastName
	}
	return resp
}

// =============================================================================
// DATA RETRIEVAL HELPERS
// =============================================================================

// getEnrollmentCount returns the number of enrolled students for an activity.
func (rs *Resource) getEnrollmentCount(ctx context.Context, activityID int64) (int, error) {
	students, err := rs.ActivityService.GetEnrolledStudents(ctx, activityID)
	if err != nil {
		return 0, err
	}
	return len(students), nil
}

// formatEndTime safely formats the end time, handling nil values
func formatEndTime(endTime *time.Time) string {
	if endTime == nil {
		return ""
	}
	return endTime.Format("15:04")
}
