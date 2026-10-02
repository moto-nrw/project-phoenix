package authorize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The note audience is the first place in this system where a group decides
// what a staff member sees about a child (CONTEXT.md "Gruppe"), so the three
// ways of "having" a child and the admin shortcut are pinned here.
//
// The policy takes facts and returns a decision: the caller's classes arrive
// already normalized, which is why the fixtures below spell them lowercase.

func groupID(id int64) *int64 { return &id }

func TestResolveStudentNoteAudience(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		permissions  []string
		reader       StudentNoteReader
		child        StudentNoteChild
		wantCareTeam bool
	}{
		{
			name:   "a colleague without the child sees only the team notes",
			reader: StudentNoteReader{GroupIDs: []int64{2}, ActivityGroupIDs: []int64{9}, Classes: []string{"4b"}},
			child:  StudentNoteChild{GroupID: groupID(1), SchoolClass: "3a", ActivityGroupIDs: []int64{8}},
		},
		{
			name:         "the child's own group",
			reader:       StudentNoteReader{GroupIDs: []int64{1, 2}},
			child:        StudentNoteChild{GroupID: groupID(1), SchoolClass: "3a"},
			wantCareTeam: true,
		},
		{
			name:         "an activity the child is enrolled in",
			reader:       StudentNoteReader{ActivityGroupIDs: []int64{8, 9}},
			child:        StudentNoteChild{GroupID: groupID(1), ActivityGroupIDs: []int64{8}},
			wantCareTeam: true,
		},
		{
			name:         "the child's school class",
			reader:       StudentNoteReader{Classes: []string{"3a"}},
			child:        StudentNoteChild{SchoolClass: "3a"},
			wantCareTeam: true,
		},
		{
			name:   "a child with no class matches no class",
			reader: StudentNoteReader{Classes: []string{"3a"}},
			child:  StudentNoteChild{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			audience := ResolveStudentNoteAudience(test.permissions, test.reader, test.child)
			if audience.Admin {
				t.Error("a colleague is not an admin")
			}
			if audience.CareTeam != test.wantCareTeam {
				t.Errorf("care team = %v, want %v", audience.CareTeam, test.wantCareTeam)
			}
		})
	}
}

func TestResolveStudentNoteAudienceAdmin(t *testing.T) {
	t.Parallel()

	audience := ResolveStudentNoteAudience(
		[]string{"admin:*"}, StudentNoteReader{}, StudentNoteChild{})
	if !audience.Admin || !audience.CareTeam {
		t.Errorf("an admin reaches every audience, got %+v", audience)
	}
}

func TestResolveStudentNoteAudienceSubstitutionDoesNotGrantLeadership(t *testing.T) {
	t.Parallel()

	// GroupIDs contains both the caller's teacher groups and temporary
	// substitutions. A substitute has the child for care-team visibility, but
	// does not lead the group and must not reach or remove leadership notes.
	audience := ResolveStudentNoteAudience(nil,
		StudentNoteReader{GroupIDs: []int64{12}}, StudentNoteChild{GroupID: groupID(12)})

	assert.True(t, audience.CareTeam)
	assert.Empty(t, audience.LedEducationGroupIDs)
	assert.False(t, CanDeleteStudentNote(audience, nil, groupID(12), nil))
}

func TestCanDeleteStudentNote(t *testing.T) {
	t.Parallel()

	led := StudentNoteAudience{LedActivityGroupIDs: []int64{8}, LedEducationGroupIDs: []int64{1}}

	tests := []struct {
		name       string
		audience   StudentNoteAudience
		activity   *int64
		education  *int64
		childGroup *int64
		want       bool
	}{
		{
			name:     "the lead of the activity a note refers to",
			audience: led, activity: groupID(8), want: true,
		},
		{
			name:     "another activity's lead may not",
			audience: led, activity: groupID(9),
		},
		{
			name:     "the lead of the group a note refers to",
			audience: led, education: groupID(1), want: true,
		},
		{
			name:     "a note without a reference falls back to the child's group",
			audience: led, childGroup: groupID(1), want: true,
		},
		{
			name:     "a colleague who leads neither",
			audience: StudentNoteAudience{}, childGroup: groupID(1),
		},
		{
			name:     "an admin may always",
			audience: StudentNoteAudience{Admin: true}, childGroup: groupID(5), want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := CanDeleteStudentNote(
				test.audience, test.activity, test.education, test.childGroup)
			if got != test.want {
				t.Errorf("CanDeleteStudentNote = %v, want %v", got, test.want)
			}
		})
	}
}
