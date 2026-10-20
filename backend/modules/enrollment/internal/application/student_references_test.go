package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
)

func TestApprovedReferenceOfStudent(t *testing.T) {
	t.Parallel()

	studentID := int64(42)
	otherID := int64(84)
	for _, test := range []struct {
		name     string
		child    enrollment.RequestChild
		expected bool
	}{
		{name: "approved created child", child: enrollment.RequestChild{Status: enrollment.ChildStatusApproved, CreatedStudentID: &studentID}, expected: true},
		{name: "approved matched child", child: enrollment.RequestChild{Status: enrollment.ChildStatusApproved, MatchedStudentID: &studentID}, expected: true},
		{name: "withdrawn child", child: enrollment.RequestChild{Status: enrollment.ChildStatusWithdrawn, CreatedStudentID: &studentID}},
		{name: "other child", child: enrollment.RequestChild{Status: enrollment.ChildStatusApproved, CreatedStudentID: &otherID}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, approvedReferenceOfStudent(&test.child, studentID))
		})
	}
}
