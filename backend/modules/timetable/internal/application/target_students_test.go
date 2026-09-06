package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestMatchTargetStudents(t *testing.T) {
	t.Parallel()
	grade := int16(2)
	class := "2b"
	educationGroupID := int64(len(t.Name()))
	targets := map[int64][]domain.GroupTarget{
		10: {{TargetGroupType: "jahrgang", TargetGradeLevel: &grade}},
		20: {{TargetGroupType: "klasse", TargetSchoolClass: &class}},
		30: {{TargetGroupType: "gruppe", EducationGroupID: &educationGroupID}},
	}
	students := []domain.TargetStudent{
		{ID: 4, SchoolClass: "2B", EducationGroupID: &educationGroupID},
		{ID: 2, SchoolClass: " 2b ", EducationGroupID: &educationGroupID},
		{ID: 3, SchoolClass: "3a"},
		{ID: 1, SchoolClass: "2a", EnrolledUntil: "2026-08-31"},
	}

	result := matchTargetStudents(targets, students, "2026-09-04")

	assert.Equal(t, []int64{2, 4}, result[10])
	assert.Equal(t, []int64{2, 4}, result[20])
	assert.Equal(t, []int64{2, 4}, result[30])
}

func TestCareEndedBeforeTreatsInvalidOrEmptyDatesAsOpenEnded(t *testing.T) {
	t.Parallel()
	assert.False(t, careEndedBefore("", "2026-09-04"))
	assert.False(t, careEndedBefore("not-a-date", "2026-09-04"))
	assert.False(t, careEndedBefore("2026-09-04", "2026-09-04"))
	assert.True(t, careEndedBefore("2026-09-03", "2026-09-04"))
}
