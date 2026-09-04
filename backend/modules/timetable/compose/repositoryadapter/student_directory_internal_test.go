package repositoryadapter

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
)

func int16Ptr(value int16) *int16 { return &value }
func strPtr(value string) *string { return &value }

// The matcher must decide exactly like the SQL join it replaced (#2662):
// Jahrgang compares the first digit run of the class, Klasse the trimmed
// class case-insensitively, Gruppe the education group id.
func TestMatchTargetStudentsMirrorsTheFormerJoin(t *testing.T) {
	t.Parallel()
	groupID := int64(40)
	students := []DirectoryStudent{
		{ID: 11, SchoolClass: "3a"},
		{ID: 12, SchoolClass: " 3B "},
		{ID: 13, SchoolClass: "13c"},
		{ID: 14, SchoolClass: "Vorschule", GroupID: &groupID},
		{ID: 15, SchoolClass: "3a", EnrolledUntil: "2026-05-31"},
		{ID: 16, SchoolClass: "3a", EnrolledUntil: "2026-06-30"},
	}
	targets := map[int64][]*activities.GroupTarget{
		1: {
			{TargetGroupType: activities.TargetGroupTypeJahrgang, TargetGradeLevel: int16Ptr(3)},
			{TargetGroupType: activities.TargetGroupTypeJahrgang, TargetGradeLevel: int16Ptr(3)},
		},
		2: {{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: strPtr("3b")}},
		3: {{TargetGroupType: activities.TargetGroupTypeGruppe, EducationGroupID: &groupID}},
		4: {{TargetGroupType: activities.TargetGroupTypeJahrgang}, {TargetGroupType: activities.TargetGroupTypeKlasse}},
		5: {{TargetGroupType: activities.TargetGroupTypeAngebot}},
	}

	unbounded := matchTargetStudents(targets, students, nil)
	assert.Equal(t, []int64{11, 12, 15, 16}, unbounded[1], "Jahrgang 3 matches every class whose first digit run is 3 (also ' 3B '), once per student, but not 13c")
	assert.Equal(t, []int64{12}, unbounded[2])
	assert.Equal(t, []int64{14}, unbounded[3])
	assert.Empty(t, unbounded[4], "rules without a value match nobody")
	assert.Empty(t, unbounded[5])

	bounded := matchTargetStudents(targets, students, parseDirectoryDate("2026-06-15"))
	assert.Equal(t, []int64{11, 12, 16}, bounded[1], "care that ended before the day drops out, care ending after it stays")
}
