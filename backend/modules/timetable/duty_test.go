package timetable_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
)

func TestValidateTemplateShapeDuty(t *testing.T) {
	t.Parallel()

	listKind := timetable.ListKindMensa
	groupID := new(int64)
	cases := []struct {
		name    string
		shape   timetable.TemplateShape
		wantErr string
	}{
		{"care needs a room", timetable.TemplateShape{Type: timetable.GroupTypeCare}, "room_id is required"},
		{"care with room", timetable.TemplateShape{Type: timetable.GroupTypeCare, RoomID: 3}, ""},
		{"negative room", timetable.TemplateShape{Type: timetable.GroupTypeDuty, RoomID: -1}, "room_id is required"},
		{"duty without room", timetable.TemplateShape{Type: timetable.GroupTypeDuty, TargetGroupType: timetable.TargetGroupTypeNone}, ""},
		{"duty with room", timetable.TemplateShape{Type: timetable.GroupTypeDuty, RoomID: 3}, ""},
		{"duty with target group", timetable.TemplateShape{Type: timetable.GroupTypeDuty, TargetGroupType: timetable.TargetGroupTypeGrade}, "no target group"},
		{"duty with targets", timetable.TemplateShape{Type: timetable.GroupTypeDuty, HasTargets: true}, "no target group"},
		{"duty with education group", timetable.TemplateShape{Type: timetable.GroupTypeDuty, EducationGroupID: groupID}, "no target group"},
		{"duty with children", timetable.TemplateShape{Type: timetable.GroupTypeDuty, HasStudents: true}, "no children"},
		{"duty with offering source", timetable.TemplateShape{Type: timetable.GroupTypeDuty, HasOfferingSource: true}, "no offering source"},
		{"duty with participant limit", timetable.TemplateShape{Type: timetable.GroupTypeDuty, MaxParticipants: 5}, "no participant limit"},
		{"duty with list kind", timetable.TemplateShape{Type: timetable.GroupTypeDuty, ListKind: &listKind}, "no list_kind"},
		{"care keeps children", timetable.TemplateShape{Type: timetable.GroupTypeCare, RoomID: 3, HasStudents: true, ListKind: &listKind}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := timetable.ValidateTemplateShape(tc.shape)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestIsValidGroupTypeAcceptsDuty(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"care", "activity", "external", "duty"} {
		assert.True(t, timetable.IsValidGroupType(value), value)
	}
	assert.False(t, timetable.IsValidGroupType("dienst"))
	assert.False(t, timetable.IsValidGroupType(""))
}

func TestIsUnderstaffedWithMinimum(t *testing.T) {
	t.Parallel()

	present := timetable.InstanceStaff{}
	absent := timetable.InstanceStaff{IsAbsent: true}
	substitute := timetable.InstanceStaff{IsSubstitute: true}
	cases := []struct {
		name    string
		rows    []timetable.InstanceStaff
		minimum int
		want    bool
	}{
		{"no floor keeps the absence rule", []timetable.InstanceStaff{present}, 0, false},
		{"floor met", []timetable.InstanceStaff{present, present}, 2, false},
		{"one short of the floor", []timetable.InstanceStaff{present}, 2, true},
		{"absence without substitute", []timetable.InstanceStaff{present, absent}, 1, true},
		{"substitute restores the floor", []timetable.InstanceStaff{present, absent, substitute}, 2, false},
		{"nobody planned", nil, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, timetable.IsUnderstaffedWithMinimum(tc.rows, tc.minimum))
		})
	}
}
