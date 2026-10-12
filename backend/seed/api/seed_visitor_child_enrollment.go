package api

import (
	"errors"
	"fmt"
	"maps"
)

// visitorChildOfferings are the bookings of the demo parent's child: care and
// lunch on every school day. The parents portal then shows booked care and
// pickup times for the whole week, and the courses of the demo phase as
// requestable (#3923).
var visitorChildOfferings = map[string][]string{
	"ogs-ganztag": {"mon", "tue", "wed", "thu", "fri"},
	"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
}

// seedVisitorChildEnrollment gives the child of the first demo parent an
// approved enrollment in the demo phase (#3923). The child came in through
// /api/students, not through an enrollment, so the parents portal had no
// booked care, no pickup times and no courses for it.
//
// Only an existing_students phase pins a submission to a child the school
// already has; in any other phase the approval would create a second child.
// The demo phase takes that audience for this one submission and is open to
// everyone again before any other family applies.
func (s parentEnrollmentSeedStep) seedVisitorChildEnrollment(
	rt *Runtime, adminAuth AuthRef, phaseID int64, phaseBody map[string]any,
	offerings map[string]int64, parents []ParentCredentials, parentAuths map[string]AuthRef,
) error {
	if len(parents) == 0 {
		return nil
	}
	parent := parents[0]
	parentAuth, ok := parentAuths[parent.Email]
	if !ok {
		return fmt.Errorf("parent %s is not logged in for the child's enrollment", parent.Email)
	}
	child, err := renewalChildFor(rt, parent)
	if err != nil {
		return err
	}
	// The grade of today's class, not of the seed data: the grade transition
	// may already have moved the class, and another grade would move it again.
	grade, err := seedStudentGrade(rt, adminAuth, parent.StudentIDs[0])
	if err != nil {
		return err
	}
	body := s.visitorChildSubmission(rt, phaseID, offerings, parent, child, grade)
	return withPhaseAudience(rt, adminAuth, phaseID, phaseBody, "existing_students", func() error {
		raw, err := rt.Client.PostWithAuth(parentAuth, "/parent/enrollments/"+rt.Bootstrap.TenantSlug+"/submit", body)
		if err != nil {
			return fmt.Errorf("submit enrollment of the demo parent's child: %w", err)
		}
		request, err := parseEnrollmentSubmitResponse(raw, "parent")
		if err != nil {
			return err
		}
		detail, err := s.loadEnrollmentRequestDetail(rt, adminAuth, request.RequestID)
		if err != nil {
			return err
		}
		for _, childID := range detail.ChildIDs {
			if err := s.decideEnrollmentChild(rt, adminAuth, request.RequestID, childID, "approved", "Demo-Zusage: Betreuung im laufenden Schuljahr"); err != nil {
				return err
			}
		}
		return nil
	})
}

// visitorChildSubmission is the parent form for the child. It names the
// guardian as the school shows them, which is the visitor in the public
// demo. It leaves out what an approval would write onto the existing child
// and guardian: a co-guardian, a phone number and the photo consent, which
// the parent withdraws elsewhere in the seed.
func (s parentEnrollmentSeedStep) visitorChildSubmission(
	rt *Runtime, phaseID int64, offerings map[string]int64, parent ParentCredentials, child renewalChild, grade int16,
) map[string]any {
	seedFirst, seedLast := splitSeedName(parent.Name)
	displaced := DemoGuardians[visitorGuardianIndex]
	guardianFirst, guardianLast := visitorDisplayName(rt.FixedSeeder.visitor, true, seedFirst, seedLast, displaced.FirstName, displaced.LastName)
	offeringIDs := make([]int64, 0, len(visitorChildOfferings))
	days := make(map[int64][]string, len(visitorChildOfferings))
	for _, key := range []string{"ogs-ganztag", "mittagessen"} {
		offeringIDs = append(offeringIDs, offerings[key])
		days[offerings[key]] = visitorChildOfferings[key]
	}
	body := s.enrollmentSubmissionWithDays(phaseID, offerings, child.student.FirstName, child.student.LastName,
		child.birthday, grade, guardianFirst, guardianLast, parent.Email, "visitor-child", offeringIDs, days)
	delete(body, "additional_guardians")
	delete(body, "guardian_phone")
	body["consent_flags"] = map[string]any{"data_processing": true}
	return body
}

// withPhaseAudience runs action while the phase admits only the given
// audience, and opens it to everyone again afterwards.
func withPhaseAudience(rt *Runtime, auth AuthRef, phaseID int64, phaseBody map[string]any, audience string, action func() error) (err error) {
	path := fmt.Sprintf("/api/enrollment/phases/%d", phaseID)
	temporary := maps.Clone(phaseBody)
	temporary["audience"] = audience
	if _, err := rt.Client.PutWithAuth(auth, path, temporary); err != nil {
		return fmt.Errorf("set phase audience %s: %w", audience, err)
	}
	defer func() {
		restored := maps.Clone(phaseBody)
		restored["audience"] = "open"
		if _, restoreErr := rt.Client.PutWithAuth(auth, path, restored); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("open phase to everyone again: %w", restoreErr))
		}
	}()
	return action()
}

// seedStudentGrade reads the grade of the class a child is in today.
func seedStudentGrade(rt *Runtime, auth AuthRef, studentID int64) (int16, error) {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/api/students/%d", studentID))
	if err != nil {
		return 0, fmt.Errorf("load student %d: %w", studentID, err)
	}
	var resp struct {
		Data struct {
			SchoolClass string `json:"school_class"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return 0, fmt.Errorf("parse student %d: %w", studentID, err)
	}
	grade, err := demoClassGrade(resp.Data.SchoolClass)
	if err != nil || grade <= 0 {
		return 0, fmt.Errorf("student %d has no grade in class %q", studentID, resp.Data.SchoolClass)
	}
	return int16(grade), nil
}
