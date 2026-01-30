package education

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

// Tests for collectSubstitutionRelatedIDs

func TestCollectSubstitutionRelatedIDs_NilSlice(t *testing.T) {
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(nil)
	assert.NotNil(t, groupIDs)
	assert.NotNil(t, staffIDs)
	assert.Empty(t, groupIDs)
	assert.Empty(t, staffIDs)
}

func TestCollectSubstitutionRelatedIDs_EmptySlice(t *testing.T) {
	substitutions := []*education.GroupSubstitution{}
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(substitutions)
	assert.NotNil(t, groupIDs)
	assert.NotNil(t, staffIDs)
	assert.Empty(t, groupIDs)
	assert.Empty(t, staffIDs)
}

func TestCollectSubstitutionRelatedIDs_SingleSubstitutionAllIDsPopulated(t *testing.T) {
	regularStaffID := int64(10)
	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID,
			SubstituteStaffID: 2,
		},
	}
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(substitutions)

	assert.Len(t, groupIDs, 1)
	assert.True(t, groupIDs[1])

	assert.Len(t, staffIDs, 2)
	assert.True(t, staffIDs[10])
	assert.True(t, staffIDs[2])
}

func TestCollectSubstitutionRelatedIDs_NilRegularStaffID(t *testing.T) {
	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    nil,
			SubstituteStaffID: 2,
		},
	}
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(substitutions)

	assert.Len(t, groupIDs, 1)
	assert.True(t, groupIDs[1])

	assert.Len(t, staffIDs, 1)
	assert.True(t, staffIDs[2])
	assert.False(t, staffIDs[0]) // Nil ID shouldn't be in map
}

func TestCollectSubstitutionRelatedIDs_ZeroGroupID(t *testing.T) {
	regularStaffID := int64(10)
	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           0, // Should be skipped
			RegularStaffID:    &regularStaffID,
			SubstituteStaffID: 2,
		},
	}
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(substitutions)

	assert.Empty(t, groupIDs)

	assert.Len(t, staffIDs, 2)
	assert.True(t, staffIDs[10])
	assert.True(t, staffIDs[2])
}

func TestCollectSubstitutionRelatedIDs_MultipleWithOverlappingIDs(t *testing.T) {
	regularStaffID1 := int64(10)
	regularStaffID2 := int64(10) // Same as first
	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID1,
			SubstituteStaffID: 2,
		},
		{
			GroupID:           1,                // Duplicate group ID
			RegularStaffID:    &regularStaffID2, // Duplicate staff ID
			SubstituteStaffID: 3,
		},
		{
			GroupID:           2,
			RegularStaffID:    nil,
			SubstituteStaffID: 2, // Duplicate staff ID
		},
	}
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(substitutions)

	// Deduplication should occur
	assert.Len(t, groupIDs, 2)
	assert.True(t, groupIDs[1])
	assert.True(t, groupIDs[2])

	assert.Len(t, staffIDs, 3)
	assert.True(t, staffIDs[10])
	assert.True(t, staffIDs[2])
	assert.True(t, staffIDs[3])
}

// Tests for mapKeysToSlice

func TestMapKeysToSlice_NilMap(t *testing.T) {
	result := mapKeysToSlice(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestMapKeysToSlice_EmptyMap(t *testing.T) {
	m := make(map[int64]bool)
	result := mapKeysToSlice(m)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestMapKeysToSlice_SingleEntry(t *testing.T) {
	m := map[int64]bool{
		42: true,
	}
	result := mapKeysToSlice(m)
	assert.Len(t, result, 1)
	assert.Contains(t, result, int64(42))
}

func TestMapKeysToSlice_MultipleEntries(t *testing.T) {
	m := map[int64]bool{
		10: true,
		20: true,
		30: true,
		40: true,
		50: true,
	}
	result := mapKeysToSlice(m)
	assert.Len(t, result, 5)
	// Don't check order since map iteration is random
	assert.Contains(t, result, int64(10))
	assert.Contains(t, result, int64(20))
	assert.Contains(t, result, int64(30))
	assert.Contains(t, result, int64(40))
	assert.Contains(t, result, int64(50))
}

func TestMapKeysToSlice_FalseValues(t *testing.T) {
	// Even with false values, keys should be included
	m := map[int64]bool{
		11: false,
		22: true,
		33: false,
	}
	result := mapKeysToSlice(m)
	assert.Len(t, result, 3)
	assert.Contains(t, result, int64(11))
	assert.Contains(t, result, int64(22))
	assert.Contains(t, result, int64(33))
}

// Tests for assignRelationsToSubstitutions

func TestAssignRelationsToSubstitutions_NilSubstitutions(t *testing.T) {
	group := &education.Group{Name: "Test"}
	group.ID = 1
	staff := &users.Staff{PersonID: 1}
	staff.ID = 1

	groupMap := map[int64]*education.Group{1: group}
	staffMap := map[int64]*users.Staff{1: staff}

	// Should not panic
	assert.NotPanics(t, func() {
		assignRelationsToSubstitutions(nil, groupMap, staffMap)
	})
}

func TestAssignRelationsToSubstitutions_EmptyMaps(t *testing.T) {
	regularStaffID := int64(10)
	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID,
			SubstituteStaffID: 2,
		},
	}

	groupMap := make(map[int64]*education.Group)
	staffMap := make(map[int64]*users.Staff)

	assignRelationsToSubstitutions(substitutions, groupMap, staffMap)

	// Relations should remain nil
	assert.Nil(t, substitutions[0].Group)
	assert.Nil(t, substitutions[0].RegularStaff)
	assert.Nil(t, substitutions[0].SubstituteStaff)
}

func TestAssignRelationsToSubstitutions_MatchingIDs(t *testing.T) {
	regularStaffID := int64(10)

	group := &education.Group{Name: "Test Group"}
	group.ID = 1

	regularStaff := &users.Staff{PersonID: 1}
	regularStaff.ID = 10

	substituteStaff := &users.Staff{PersonID: 2}
	substituteStaff.ID = 2

	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID,
			SubstituteStaffID: 2,
		},
	}

	groupMap := map[int64]*education.Group{1: group}
	staffMap := map[int64]*users.Staff{10: regularStaff, 2: substituteStaff}

	assignRelationsToSubstitutions(substitutions, groupMap, staffMap)

	assert.Equal(t, group, substitutions[0].Group)
	assert.Equal(t, regularStaff, substitutions[0].RegularStaff)
	assert.Equal(t, substituteStaff, substitutions[0].SubstituteStaff)
}

func TestAssignRelationsToSubstitutions_NonMatchingIDs(t *testing.T) {
	regularStaffID := int64(10)

	group := &education.Group{Name: "Other"}
	group.ID = 99 // Different ID

	staff := &users.Staff{PersonID: 1}
	staff.ID = 99 // Different ID

	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID,
			SubstituteStaffID: 2,
		},
	}

	groupMap := map[int64]*education.Group{99: group}
	staffMap := map[int64]*users.Staff{99: staff}

	assignRelationsToSubstitutions(substitutions, groupMap, staffMap)

	// Relations should remain nil when IDs don't match
	assert.Nil(t, substitutions[0].Group)
	assert.Nil(t, substitutions[0].RegularStaff)
	assert.Nil(t, substitutions[0].SubstituteStaff)
}

func TestAssignRelationsToSubstitutions_NilRegularStaffID(t *testing.T) {
	group := &education.Group{Name: "Test"}
	group.ID = 1

	substituteStaff := &users.Staff{PersonID: 1}
	substituteStaff.ID = 2

	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    nil, // Nil pointer
			SubstituteStaffID: 2,
		},
	}

	groupMap := map[int64]*education.Group{1: group}
	staffMap := map[int64]*users.Staff{2: substituteStaff}

	assignRelationsToSubstitutions(substitutions, groupMap, staffMap)

	assert.Equal(t, group, substitutions[0].Group)
	assert.Nil(t, substitutions[0].RegularStaff) // Should skip lookup
	assert.Equal(t, substituteStaff, substitutions[0].SubstituteStaff)
}

func TestAssignRelationsToSubstitutions_MultipleSubstitutions(t *testing.T) {
	regularStaffID1 := int64(10)
	regularStaffID2 := int64(11)

	group1 := &education.Group{Name: "Group 1"}
	group1.ID = 1
	group2 := &education.Group{Name: "Group 2"}
	group2.ID = 2

	staff10 := &users.Staff{PersonID: 1}
	staff10.ID = 10
	staff11 := &users.Staff{PersonID: 2}
	staff11.ID = 11
	staff2 := &users.Staff{PersonID: 3}
	staff2.ID = 2
	staff3 := &users.Staff{PersonID: 4}
	staff3.ID = 3

	substitutions := []*education.GroupSubstitution{
		{
			GroupID:           1,
			RegularStaffID:    &regularStaffID1,
			SubstituteStaffID: 2,
		},
		{
			GroupID:           2,
			RegularStaffID:    &regularStaffID2,
			SubstituteStaffID: 3,
		},
	}

	groupMap := map[int64]*education.Group{1: group1, 2: group2}
	staffMap := map[int64]*users.Staff{10: staff10, 11: staff11, 2: staff2, 3: staff3}

	assignRelationsToSubstitutions(substitutions, groupMap, staffMap)

	assert.Equal(t, group1, substitutions[0].Group)
	assert.Equal(t, staff10, substitutions[0].RegularStaff)
	assert.Equal(t, staff2, substitutions[0].SubstituteStaff)

	assert.Equal(t, group2, substitutions[1].Group)
	assert.Equal(t, staff11, substitutions[1].RegularStaff)
	assert.Equal(t, staff3, substitutions[1].SubstituteStaff)
}
