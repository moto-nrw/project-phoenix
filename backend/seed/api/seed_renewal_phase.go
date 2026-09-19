package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// demoClassGrade reads the grade out of a demo class label ("Klasse 1a" -> 1).
// The seeder is a dev tool and may not import the school-structure domain, and
// it does not need its grammar: every DemoStudent class has this one shape.
func demoClassGrade(class string) (int, error) {
	digits := strings.TrimLeft(strings.TrimPrefix(strings.TrimSpace(class), "Klasse"), " ")
	end := 0
	for end < len(digits) && digits[end] >= '0' && digits[end] <= '9' {
		end++
	}
	return strconv.Atoi(digits[:end])
}

// demoStudentBirthday is the birthday the fixed seeder gives the demo child at
// index i. Groups map to school classes: 1a/1b (born ~2019), 2a/2b (~2018),
// 3a/3b (~2017), 4a/4b (~2016); the index spreads the days across the year.
//
// It is a function because two steps need the same value: the child is created
// with it, and a re-enrollment has to send the identical date, since an
// existing_students phase finds the child by name AND birthday.
func demoStudentBirthday(i int, student DemoStudent) string {
	baseYear := 2019
	switch student.GroupKey {
	case "bärengruppe", "sonnengruppe": // Klasse 1b/2a, 2a/2b
		baseYear = 2018
	case "mondgruppe", "regenbogengruppe", "meeresgruppe": // Klasse 2b/3a, 3a/3b, 2b/3b
		baseYear = 2017
	case "blumengruppe", "schmetterlingsgruppe", "wiesengruppe": // Klasse 3b/4a, 4a/4b, 3a/4b
		baseYear = 2016
	}
	return fmt.Sprintf("%d-%02d-%02d", baseYear, (i%12)+1, (i%28)+1)
}

// renewalPhaseAnswers says which of the seeded parents answer the
// re-enrollment, by position in the parents slice, and what the school did
// with the answer. The remaining parents have the app and stay silent, and
// every other family has no app at all. Together that is each row the response
// overview (#3379) can show: answered and confirmed, answered and still open,
// missing but reachable by a Mitteilung, missing and only reachable by phone.
var renewalPhaseAnswers = []struct {
	parent int
	status string
}{
	{parent: 0, status: "approved"},
	{parent: 2},
	{parent: 4},
}

// seedRenewalPhase opens next school year's re-enrollment for the children the
// school already has. Without it the "Rücklauf" tab does not exist on any dev
// machine: it only appears for a phase that pins submissions to existing
// children, and the ordinary demo phase is open to everyone.
func (s parentEnrollmentSeedStep) seedRenewalPhase(rt *Runtime, adminAuth AuthRef, schemaID int64, parents []ParentCredentials, parentAuths map[string]AuthRef) error {
	phaseID, err := s.createRenewalPhase(rt, adminAuth, schemaID)
	if err != nil {
		return err
	}
	for _, answer := range renewalPhaseAnswers {
		if answer.parent >= len(parents) {
			continue
		}
		if err := s.submitRenewal(rt, adminAuth, phaseID, parents[answer.parent], parentAuths, answer.status); err != nil {
			return err
		}
	}
	return nil
}

func (s parentEnrollmentSeedStep) createRenewalPhase(rt *Runtime, auth AuthRef, schemaID int64) (int64, error) {
	now := time.Now().UTC()
	// The care year after the running one. It has to start in the future: the
	// overview then reads every class one grade up and leaves out the top
	// grade, which is what a real re-enrollment looks like in spring.
	startYear := now.Year()
	if now.Month() >= time.August {
		startYear++
	}
	body := map[string]any{
		"name":                         fmt.Sprintf("Wiederanmeldung %d/%d", startYear, startYear+1),
		"kind":                         "school_year",
		"service_start_date":           fmt.Sprintf("%d-08-01", startYear),
		"service_end_date":             fmt.Sprintf("%d-07-31", startYear+1),
		"enrollment_open_at":           now.Add(-24 * time.Hour).Format(time.RFC3339),
		"enrollment_close_at":          now.AddDate(0, 2, 0).Format(time.RFC3339),
		"show_status_reason_to_parent": true,
		"care_overflow_mode":           "waitlist",
		"care_offering_selection_mode": "optional",
		"audience":                     "existing_students",
		"is_active":                    true,
		"form_schema_id":               strconv.FormatInt(schemaID, 10),
	}
	respBody, err := rt.Client.PostWithAuth(auth, "/api/enrollment/phases", body)
	if err != nil {
		return 0, fmt.Errorf("create renewal phase: %w", err)
	}
	id, err := parseEnvelopeStringID(respBody)
	if err != nil {
		return 0, fmt.Errorf("parse renewal phase response: %w", err)
	}
	return id, nil
}

// renewalChild is the demo child behind a seeded parent account together with
// the grade it enters next year.
type renewalChild struct {
	student   DemoStudent
	birthday  string
	nextGrade int16
}

func renewalChildFor(rt *Runtime, parent ParentCredentials) (renewalChild, error) {
	if len(parent.StudentIDs) == 0 {
		return renewalChild{}, fmt.Errorf("parent %s has no child to re-enroll", parent.Email)
	}
	for index, studentID := range rt.FixedSeeder.studentIDByIndex {
		if studentID != parent.StudentIDs[0] || index < 0 || index >= len(DemoStudents) {
			continue
		}
		student := DemoStudents[index]
		grade, err := demoClassGrade(student.Class)
		if err != nil {
			return renewalChild{}, fmt.Errorf("demo child %s %s has no grade in class %q", student.FirstName, student.LastName, student.Class)
		}
		return renewalChild{student: student, birthday: demoStudentBirthday(index, student), nextGrade: int16(grade + 1)}, nil
	}
	return renewalChild{}, fmt.Errorf("demo child %d of parent %s was not created", parent.StudentIDs[0], parent.Email)
}

func (s parentEnrollmentSeedStep) submitRenewal(rt *Runtime, adminAuth AuthRef, phaseID int64, parent ParentCredentials, parentAuths map[string]AuthRef, status string) error {
	child, err := renewalChildFor(rt, parent)
	if err != nil {
		return err
	}
	auth, ok := parentAuths[parent.Email]
	if !ok {
		return fmt.Errorf("parent %s is not logged in for the renewal", parent.Email)
	}
	guardianFirst, guardianLast := splitSeedName(parent.Name)
	body := s.enrollmentSubmissionWithDays(phaseID, nil, child.student.FirstName, child.student.LastName,
		child.birthday, child.nextGrade, guardianFirst, guardianLast, parent.Email, "renewal", nil, nil)

	respBody, err := rt.Client.PostWithAuth(auth, "/parent/enrollments/"+rt.Bootstrap.TenantSlug+"/submit", body)
	if err != nil {
		return fmt.Errorf("submit renewal for %s: %w", parent.Email, err)
	}
	if status == "" {
		return nil
	}
	request, err := parseEnrollmentSubmitResponse(respBody, "parent")
	if err != nil {
		return err
	}
	detail, err := s.loadEnrollmentRequestDetail(rt, adminAuth, request.RequestID)
	if err != nil {
		return err
	}
	for _, childID := range detail.ChildIDs {
		if err := s.decideEnrollmentChild(rt, adminAuth, request.RequestID, childID, status, "Demo-Zusage: Wiederanmeldung"); err != nil {
			return err
		}
	}
	return nil
}

// splitSeedName splits "Vorname Nachname" at the first space.
func splitSeedName(name string) (string, string) {
	for i, r := range name {
		if r == ' ' {
			return name[:i], name[i+1:]
		}
	}
	return name, ""
}
