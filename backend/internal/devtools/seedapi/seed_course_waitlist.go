package api

import (
	"context"
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
		"note":      "Unser Kind möchte gern in die Theater-AG, gern auch später.",
	})
	if err != nil {
		return fmt.Errorf("request waitlisted course: %w", err)
	}
	return nil
}

// waitlistFamilies are parents of approved online enrollments whose child is
// not in the Theater-AG. Their app accounts exist only after the invitations
// are accepted (seedFamilyAppAccountsStep); behind Lina Amini they make the
// team's waiting list a real list (#4767).
var waitlistFamilies = []string{
	"martin.wolf@example.test",
	"nadine.hartmann@example.test",
	"rania.al-hassan@example.test",
}

// seedFurtherWaitlistedCourseRequests lets each waitlist family ask for the
// full Theater-AG, one after another, so the queue has a stable order.
func seedFurtherWaitlistedCourseRequests(ctx context.Context, rt *Runtime, password string) error {
	courseID := rt.Enrollment.Offerings["ag-theater"]
	for _, email := range waitlistFamilies {
		auth, err := rt.Adapter.LoginParent(ctx, scopedEmail(email, rt.FixedSeeder.accountScope), password)
		if err != nil {
			return fmt.Errorf("seed waiting list: login %s: %w", email, err)
		}
		raw, err := rt.Client.GetWithAuth(auth, "/parent/me/children")
		if err != nil {
			return fmt.Errorf("seed waiting list: list children of %s: %w", email, err)
		}
		var children struct {
			Data []struct {
				StudentID string `json:"student_id"`
			} `json:"data"`
		}
		if err := parseJSON(raw, &children); err != nil || len(children.Data) == 0 {
			return fmt.Errorf("seed waiting list: no child for %s: %w", email, err)
		}
		studentID, err := strconv.ParseInt(children.Data[0].StudentID, 10, 64)
		if err != nil {
			return fmt.Errorf("seed waiting list: child id of %s: %w", email, err)
		}
		if err := seedWaitlistedCourseRequest(rt, auth, studentID, courseID); err != nil {
			return err
		}
	}
	return nil
}
