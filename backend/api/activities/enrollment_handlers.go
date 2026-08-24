package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// parseStudentID parses student ID from URL param "studentId".
// Returns 0 and false if parsing fails (error already rendered).
func (rs *Resource) parseStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return 0, false
	}
	return studentID, true
}

// STUDENT ENROLLMENT HANDLERS

// getActivityStudents handles getting students enrolled in an activity
func (rs *Resource) getActivityStudents(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Get enrolled students
	students, err := rs.ActivityService.GetEnrolledStudents(r.Context(), activity.ID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build simplified student responses
	callerPermissions := jwt.PermissionsFromCtx(r.Context())
	responses := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		if !authorize.CanReadStudent(r.Context(), callerPermissions, student, rs.UserContextService) {
			continue
		}

		// Create a basic response with the ID
		studentResp := StudentResponse{
			ID: student.ID,
			// Default name values if no person data
			FirstName: "Student",
			LastName:  fmt.Sprintf("%d", student.ID),
		}

		// Check if student has person data
		if student.Person != nil {
			person := student.Person
			studentResp.FirstName = person.FirstName
			studentResp.LastName = person.LastName
		}

		responses = append(responses, studentResp)
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Students enrolled in activity '%s' retrieved successfully", activity.Name))
}

// getStudentEnrollments handles getting activities that a student is enrolled in
func (rs *Resource) getStudentEnrollments(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Get activities that student is enrolled in
	enrolledGroups, err := rs.ActivityService.GetStudentEnrollments(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build activity responses
	responses := make([]ActivityResponse, 0, len(enrolledGroups))
	for _, group := range enrolledGroups {
		if group == nil {
			continue // Skip nil groups to prevent panic
		}
		responses = append(responses, newActivityResponse(group, rs.getEnrollmentCount(r.Context(), group.ID)))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Activities for student ID %d retrieved successfully", studentID))
}

// getAvailableActivities handles getting activities available for a student to enroll in
func (rs *Resource) getAvailableActivities(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Get available activities for student
	availableGroups, err := rs.ActivityService.GetAvailableGroups(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build activity responses
	responses := make([]ActivityResponse, 0, len(availableGroups))
	for _, group := range availableGroups {
		if group == nil {
			continue // Skip nil groups to prevent panic
		}
		responses = append(responses, newActivityResponse(group, rs.getEnrollmentCount(r.Context(), group.ID)))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Available activities for student ID %d retrieved successfully", studentID))
}

// We already have the enrollStudent method, no need to modify it since it follows the standard

// unenrollStudent handles removing a student from an activity
func (rs *Resource) unenrollStudent(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Unenroll student
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ActivityService.UnenrollStudent(ctx, activity.ID, studentID)
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, fmt.Sprintf("Student unenrolled from activity '%s' successfully", activity.Name))
}

// updateGroupEnrollments handles updating student enrollments in batch
func (rs *Resource) updateGroupEnrollments(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Parse request
	var req BatchEnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Update group enrollments
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ActivityService.UpdateGroupEnrollments(ctx, activity.ID, req.StudentIDs)
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Create a simplified response
	response := map[string]interface{}{
		"activity_id":       activity.ID,
		"activity_name":     activity.Name,
		"enrollment_count":  rs.getEnrollmentCount(r.Context(), activity.ID),
		"max_participants":  activity.ParticipantLimit(),
		"students_enrolled": req.StudentIDs,
	}

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Enrollments for activity '%s' updated successfully", activity.Name))
}

// enrollStudent handles enrolling a student in an activity
func (rs *Resource) enrollStudent(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}

	// Enroll student
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ActivityService.EnrollStudent(ctx, activity.ID, studentID)
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Student enrolled successfully")
}
