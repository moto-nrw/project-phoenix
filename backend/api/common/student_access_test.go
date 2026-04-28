package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/models/users"
)

func TestStudentAccessContext_HasFullAccessByGroupID(t *testing.T) {
	supervised := int64(100)
	other := int64(200)

	cases := []struct {
		name    string
		access  *StudentAccessContext
		groupID *int64
		want    bool
	}{
		{
			name:    "nil context never grants access",
			access:  nil,
			groupID: &supervised,
			want:    false,
		},
		{
			name:    "admin sees every student (group-less included)",
			access:  &StudentAccessContext{IsAdmin: true},
			groupID: nil,
			want:    true,
		},
		{
			name:    "all_staff scope sees every student (group-less included)",
			access:  &StudentAccessContext{AllStaffScope: true},
			groupID: nil,
			want:    true,
		},
		{
			name: "group supervisor sees only their group",
			access: &StudentAccessContext{
				MyGroupIDs: map[int64]struct{}{supervised: {}},
			},
			groupID: &supervised,
			want:    true,
		},
		{
			name: "group supervisor blocked from other groups",
			access: &StudentAccessContext{
				MyGroupIDs: map[int64]struct{}{supervised: {}},
			},
			groupID: &other,
			want:    false,
		},
		{
			name: "group supervisor blocked from group-less students",
			access: &StudentAccessContext{
				MyGroupIDs: map[int64]struct{}{supervised: {}},
			},
			groupID: nil,
			want:    false,
		},
		{
			name:    "supervisor with no groups always blocked",
			access:  &StudentAccessContext{},
			groupID: &supervised,
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.access.HasFullAccessByGroupID(tc.groupID))
		})
	}
}

func TestStudentAccessContext_HasFullAccessToStudent_NilGuards(t *testing.T) {
	access := &StudentAccessContext{IsAdmin: true}

	// Nil student is always denied even for admins — the caller should never
	// reach this path with nil, but the helper must not panic.
	assert.False(t, access.HasFullAccessToStudent(nil))

	groupID := int64(100)
	student := &users.Student{GroupID: &groupID}
	assert.True(t, access.HasFullAccessToStudent(student),
		"admin always sees concrete students")
}
