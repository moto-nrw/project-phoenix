package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStudentAccessContext_HasFullAccess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		access *StudentAccessContext
		want   bool
	}{
		{
			name:   "nil context never grants access",
			access: nil,
			want:   false,
		},
		{
			name:   "admin sees every student",
			access: &StudentAccessContext{IsAdmin: true},
			want:   true,
		},
		{
			name:   "any verified staff member sees every student",
			access: &StudentAccessContext{IsStaff: true},
			want:   true,
		},
		{
			name:   "neither admin nor staff (guest, guardian) stays redacted",
			access: &StudentAccessContext{},
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.access.HasFullAccess())
		})
	}
}

func TestStudentAccessContext_HasFullAccessToStudent_NilGuards(t *testing.T) {
	t.Parallel()

	access := &StudentAccessContext{IsAdmin: true}

	// Nil student is always denied even for admins — the caller should never
	// reach this path with nil, but the helper must not panic.
	assert.False(t, access.HasFullAccessToStudent(nil))
	var absent *accessStudent
	assert.False(t, access.HasFullAccessToStudent(absent))

	groupID := int64(100)
	student := &accessStudent{GroupID: &groupID}
	assert.True(t, access.HasFullAccessToStudent(student),
		"admin always sees concrete students")

	// #2329: group membership is irrelevant — staff see group-less children too.
	staff := &StudentAccessContext{IsStaff: true}
	assert.True(t, staff.HasFullAccessToStudent(&accessStudent{}))

	var nilAccess *StudentAccessContext
	assert.False(t, nilAccess.HasFullAccessToStudent(student))
}

type accessStudent struct{ GroupID *int64 }

func (s *accessStudent) IsAuthorizationStudent() bool { return s != nil }
