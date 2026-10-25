package timetable_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
)

func TestBlockFilterMatches(t *testing.T) {
	t.Parallel()
	group, otherGroup, staff, otherStaff := int64(71), int64(81), int64(11), int64(12)
	rows := []timetable.InstanceStaff{{StaffID: staff, IsAbsent: true}, {StaffID: 13, IsSubstitute: true}}
	groups := []int64{group}

	tests := []struct {
		name   string
		filter timetable.BlockFilter
		want   bool
	}{
		{"no filter keeps every block", timetable.BlockFilter{}, true},
		{"group of the block", timetable.BlockFilter{EducationGroupID: &group}, true},
		{"other group", timetable.BlockFilter{EducationGroupID: &otherGroup}, false},
		{"absent staff row still shows the person", timetable.BlockFilter{StaffID: &staff}, true},
		{"staff not on the block", timetable.BlockFilter{StaffID: &otherStaff}, false},
		{"both must match", timetable.BlockFilter{EducationGroupID: &group, StaffID: &otherStaff}, false},
		{"both match", timetable.BlockFilter{EducationGroupID: &group, StaffID: &staff}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.filter.Matches(rows, groups))
			assert.Equal(t, tt.filter.EducationGroupID != nil || tt.filter.StaffID != nil, tt.filter.Active())
		})
	}
}
