package activities

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
)

// =============================================================================
// STUDENT ENROLLMENT HANDLERS
// =============================================================================

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
	responses := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		// Skip nil students to prevent panic
		if student == nil {
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
	if err := rs.ActivityService.EnrollStudent(r.Context(), activity.ID, studentID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Student enrolled successfully")
}

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
	if err := rs.ActivityService.UnenrollStudent(r.Context(), activity.ID, studentID); err != nil {
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
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Update group enrollments
	if err := rs.ActivityService.UpdateGroupEnrollments(r.Context(), activity.ID, req.StudentIDs); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Create a simplified response
	response := map[string]interface{}{
		"activity_id":       activity.ID,
		"activity_name":     activity.Name,
		"enrollment_count":  rs.getEnrollmentCount(r.Context(), activity.ID),
		"max_participants":  activity.MaxParticipants,
		"students_enrolled": req.StudentIDs,
	}

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Enrollments for activity '%s' updated successfully", activity.Name))
}
