package api

import "fmt"

// possiblyEnrolledStudentIndex is the demo child (Lena Hofmann, Klasse 1b)
// whose family submits the public form again (#6053). No other seed step
// enrolls her, so her request stays open and the office sees that she may
// already be enrolled.
const possiblyEnrolledStudentIndex = 13

// laterSeedSubmissions are the requests submitted after the public demo
// requests: the third demo parent's request through the parent portal and
// the public request for a child the school already cares for.
func (s parentEnrollmentSeedStep) laterSeedSubmissions(
	rt *Runtime, phaseID int64, offerings map[string]int64,
	parents []ParentCredentials, parentAuths map[string]AuthRef,
) ([]enrollmentSubmission, error) {
	var submissions []enrollmentSubmission
	if len(parents) > 2 {
		parent := parents[2]
		submissions = append(submissions, enrollmentSubmission{
			source: "parent",
			auth:   parentAuths[parent.Email],
			path:   "/parent/enrollments/" + rt.Bootstrap.TenantSlug + "/submit",
			body: withDemoCoGuardian(s.enrollmentSubmissionWithDays(phaseID, offerings, "Lina", "Amini", "2019-02-14", 1,
				"Shirin", "Amini", parent.Email, "parent-authenticated",
				[]int64{offerings["ogs-ganztag"], offerings["mittagessen"]}, map[int64][]string{
					offerings["ogs-ganztag"]: {"mon", "tue", "wed", "thu"},
					offerings["mittagessen"]: {"mon", "tue", "wed", "thu"},
				},
			), scopedEmail("reza.amini@email.de", rt.FixedSeeder.accountScope)),
			status: "approved",
			reason: "Zusage: vier Betreuungstage",
		})
	}
	possiblyEnrolled, err := s.possiblyEnrolledSubmission(rt, phaseID, offerings)
	if err != nil {
		return nil, err
	}
	return append(submissions, possiblyEnrolled), nil
}

// possiblyEnrolledSubmission is the public request of a child the school
// already cares for, with the child's name and birthday (#6053). It stays
// open: before approving, the office chooses the enrolled child or a new one.
func (s parentEnrollmentSeedStep) possiblyEnrolledSubmission(rt *Runtime, phaseID int64, offerings map[string]int64) (enrollmentSubmission, error) {
	if possiblyEnrolledStudentIndex >= len(DemoStudents) {
		return enrollmentSubmission{}, fmt.Errorf("demo child %d does not exist", possiblyEnrolledStudentIndex)
	}
	student := DemoStudents[possiblyEnrolledStudentIndex]
	grade, err := demoClassGrade(student.Class)
	if err != nil {
		return enrollmentSubmission{}, fmt.Errorf("demo child %s %s has no grade in class %q", student.FirstName, student.LastName, student.Class)
	}
	birthday := demoStudentBirthday(possiblyEnrolledStudentIndex, student, rt.FixedSeeder.seedDay)
	return enrollmentSubmission{
		source: "public",
		key:    "possibly-enrolled",
		path:   "/api/enrollment/" + rt.Bootstrap.TenantSlug + "/submit",
		body: s.enrollmentSubmissionWithDays(phaseID, offerings, student.FirstName, student.LastName, birthday, int16(grade),
			"Sabine", student.LastName, scopedEmail("sabine.wiederanmeldung@example.test", rt.FixedSeeder.accountScope), "possibly-enrolled",
			[]int64{offerings["ogs-kurz"], offerings["mittagessen"]}, map[int64][]string{
				offerings["ogs-kurz"]:    {"mon", "wed"},
				offerings["mittagessen"]: {"mon", "wed"},
			},
		),
	}, nil
}
