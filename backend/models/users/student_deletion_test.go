package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStudentDeletionCountsTotal(t *testing.T) {
	t.Parallel()

	counts := StudentDeletionCounts{
		TimetableAssignments: 1,
		ActivityEnrollments:  2,
		AttendanceRecords:    3,
		CareSchedules:        4,
		GuardianLinks:        5,
		CompanionLinks:       6,
		Communications:       7,
		Consents:             8,
		EnrollmentReferences: 9,
		OtherRecords:         10,
	}

	assert.Equal(t, 55, counts.Total())
}
