package api

import (
	"fmt"
	"strconv"
)

// seedWaitlistedCourseRequest asks for the full Theater-AG (all eight places
// are taken, seedEnrollmentCourseBookings), so the team sees a waiting list
// with a first child on the AG and in /anfragen (#4767).
func seedWaitlistedCourseRequest(rt *Runtime, parentAuth AuthRef, studentID, courseOfferingID int64) error {
	if courseOfferingID == 0 {
		return fmt.Errorf("seed waiting list: Theater-AG offering missing")
	}
	_, err := rt.Client.PostWithAuth(parentAuth, fmt.Sprintf("/parent/me/children/%d/courses/requests", studentID), map[string]any{
		"course_id": strconv.FormatInt(courseOfferingID, 10),
		"note":      "Lina möchte gern in die Theater-AG, gern auch später.",
	})
	if err != nil {
		return fmt.Errorf("request waitlisted course: %w", err)
	}
	return nil
}
