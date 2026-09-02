package education

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/stretchr/testify/assert"
)

// The staff half of these helpers moved to the composition layer with #2667:
// substitutions carry only the staff IDs and School Membership resolves them.
// What stays here is the group half.

// Tests for collectSubstitutionGroupIDs

func TestCollectSubstitutionGroupIDs_NilSlice(t *testing.T) {
	t.Parallel()

	groupIDs := collectSubstitutionGroupIDs(nil)
	assert.NotNil(t, groupIDs)
	assert.Empty(t, groupIDs)
}

func TestCollectSubstitutionGroupIDs_EmptySlice(t *testing.T) {
	t.Parallel()

	substitutions := []*education.GroupSubstitution{}
	groupIDs := collectSubstitutionGroupIDs(substitutions)
	assert.NotNil(t, groupIDs)
	assert.Empty(t, groupIDs)
}

func TestCollectSubstitutionGroupIDs_SingleSubstitution(t *testing.T) {
	t.Parallel()

	regularStaffID := int64(10)
	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID,
			SubstituteStaffID: 2,
		},
	}
	groupIDs := collectSubstitutionGroupIDs(substitutions)

	assert.Len(t, groupIDs, 1)
	assert.True(t, groupIDs[1])
}

func TestCollectSubstitutionGroupIDs_ZeroGroupID(t *testing.T) {
	t.Parallel()

	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           0, // Should be skipped
			SubstituteStaffID: 2,
		},
	}
	groupIDs := collectSubstitutionGroupIDs(substitutions)

	assert.Empty(t, groupIDs)
}

func TestCollectSubstitutionGroupIDs_MultipleWithOverlappingIDs(t *testing.T) {
	t.Parallel()

	substitutions := []*education.GroupSubstitution{
		{GroupID: 1, SubstituteStaffID: 2},
		{GroupID: 1, SubstituteStaffID: 3}, // Duplicate group ID
		{GroupID: 2, SubstituteStaffID: 2},
	}
	groupIDs := collectSubstitutionGroupIDs(substitutions)

	// Deduplication should occur
	assert.Len(t, groupIDs, 2)
	assert.True(t, groupIDs[1])
	assert.True(t, groupIDs[2])
}

// Tests for assignGroupsToSubstitutions

func TestAssignGroupsToSubstitutions_NilSubstitutions(t *testing.T) {
	t.Parallel()

	group := &education.Group{Name: "Test"}
	group.ID = 1

	groupMap := map[int64]*education.Group{1: group}

	// Should not panic
	assert.NotPanics(t, func() {
		assignGroupsToSubstitutions(nil, groupMap)
	})
}

func TestAssignGroupsToSubstitutions_EmptyMap(t *testing.T) {
	t.Parallel()

	substitutions := []*education.GroupSubstitution{
		{GroupID: 1, SubstituteStaffID: 2},
	}

	assignGroupsToSubstitutions(substitutions, make(map[int64]*education.Group))

	assert.Nil(t, substitutions[0].Group)
	assert.Nil(t, substitutions[0].RegularStaff)
	assert.Nil(t, substitutions[0].SubstituteStaff)
}

func TestAssignGroupsToSubstitutions_MatchingIDs(t *testing.T) {
	t.Parallel()

	group := &education.Group{Name: "Test Group"}
	group.ID = 1

	substitutions := []*education.GroupSubstitution{
		{GroupID: 1, SubstituteStaffID: 2},
	}

	assignGroupsToSubstitutions(substitutions, map[int64]*education.Group{1: group})

	assert.Equal(t, group, substitutions[0].Group)
}

func TestAssignGroupsToSubstitutions_NonMatchingIDs(t *testing.T) {
	t.Parallel()

	group := &education.Group{Name: "Other"}
	group.ID = 99 // Different ID

	substitutions := []*education.GroupSubstitution{
		{GroupID: 1, SubstituteStaffID: 2},
	}

	assignGroupsToSubstitutions(substitutions, map[int64]*education.Group{99: group})

	assert.Nil(t, substitutions[0].Group)
}

func TestAssignGroupsToSubstitutions_MultipleSubstitutions(t *testing.T) {
	t.Parallel()

	group1 := &education.Group{Name: "Group 1"}
	group1.ID = 1
	group2 := &education.Group{Name: "Group 2"}
	group2.ID = 2

	substitutions := []*education.GroupSubstitution{
		{GroupID: 1, SubstituteStaffID: 2},
		{GroupID: 2, SubstituteStaffID: 3},
	}

	assignGroupsToSubstitutions(substitutions, map[int64]*education.Group{1: group1, 2: group2})

	assert.Equal(t, group1, substitutions[0].Group)
	assert.Equal(t, group2, substitutions[1].Group)
}
