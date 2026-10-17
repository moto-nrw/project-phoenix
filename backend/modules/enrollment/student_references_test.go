package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApprovedReferenceOfStudent(t *testing.T) {
	t.Parallel()

	studentID := int64(42)
	otherID := int64(84)
	for _, test := range []struct {
		name     string
		child    RequestChild
		expected bool
	}{
		{name: "approved created child", child: RequestChild{Status: ChildStatusApproved, CreatedStudentID: &studentID}, expected: true},
		{name: "approved matched child", child: RequestChild{Status: ChildStatusApproved, MatchedStudentID: &studentID}, expected: true},
		{name: "withdrawn child", child: RequestChild{Status: ChildStatusWithdrawn, CreatedStudentID: &studentID}},
		{name: "other child", child: RequestChild{Status: ChildStatusApproved, CreatedStudentID: &otherID}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, approvedReferenceOfStudent(&test.child, studentID))
		})
	}
}
