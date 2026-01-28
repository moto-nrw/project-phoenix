package common

import (
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// uniqueInt64 Tests
// =============================================================================

func TestUniqueInt64_Empty(t *testing.T) {
	result := uniqueInt64([]int64{})
	assert.Empty(t, result)
}

func TestUniqueInt64_NilInput(t *testing.T) {
	result := uniqueInt64(nil)
	assert.Nil(t, result)
}

func TestUniqueInt64_AllUnique(t *testing.T) {
	input := []int64{1, 2, 3, 4, 5}
	result := uniqueInt64(input)
	assert.Equal(t, input, result)
}

func TestUniqueInt64_WithDuplicates(t *testing.T) {
	input := []int64{1, 2, 2, 3, 1, 4, 3, 5}
	result := uniqueInt64(input)
	assert.Len(t, result, 5)
	// Verify all original values are present (order may vary)
	seen := make(map[int64]bool)
	for _, v := range result {
		seen[v] = true
	}
	assert.True(t, seen[1])
	assert.True(t, seen[2])
	assert.True(t, seen[3])
	assert.True(t, seen[4])
	assert.True(t, seen[5])
}

func TestUniqueInt64_SingleElement(t *testing.T) {
	input := []int64{42}
	result := uniqueInt64(input)
	assert.Equal(t, []int64{42}, result)
}

func TestUniqueInt64_AllSame(t *testing.T) {
	input := []int64{7, 7, 7, 7, 7}
	result := uniqueInt64(input)
	assert.Equal(t, []int64{7}, result)
}

// =============================================================================
// filterCheckedInStudents Tests
// =============================================================================

func TestFilterCheckedInStudents_Empty(t *testing.T) {
	result := filterCheckedInStudents(map[int64]*activeService.AttendanceStatus{})
	assert.Empty(t, result)
}

func TestFilterCheckedInStudents_AllCheckedIn(t *testing.T) {
	attendances := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1, Status: "checked_in"},
		2: {StudentID: 2, Status: "checked_in"},
		3: {StudentID: 3, Status: "checked_in"},
	}
	result := filterCheckedInStudents(attendances)
	assert.Len(t, result, 3)
}

func TestFilterCheckedInStudents_NoneCheckedIn(t *testing.T) {
	attendances := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1, Status: "not_checked_in"},
		2: {StudentID: 2, Status: "checked_out"},
	}
	result := filterCheckedInStudents(attendances)
	assert.Empty(t, result)
}

func TestFilterCheckedInStudents_Mixed(t *testing.T) {
	attendances := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1, Status: "checked_in"},
		2: {StudentID: 2, Status: "not_checked_in"},
		3: {StudentID: 3, Status: "checked_in"},
		4: {StudentID: 4, Status: "checked_out"},
		5: {StudentID: 5, Status: "checked_in"},
	}
	result := filterCheckedInStudents(attendances)
	assert.Len(t, result, 3)
}

func TestFilterCheckedInStudents_NilValue(t *testing.T) {
	attendances := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1, Status: "checked_in"},
		2: nil, // Nil value should be skipped
		3: {StudentID: 3, Status: "checked_in"},
	}
	result := filterCheckedInStudents(attendances)
	assert.Len(t, result, 2)
}

// =============================================================================
// extractActiveGroupIDs Tests
// =============================================================================

func TestExtractActiveGroupIDs_Empty(t *testing.T) {
	result := extractActiveGroupIDs(map[int64]*activeModels.Visit{})
	assert.Empty(t, result)
}

func TestExtractActiveGroupIDs_AllWithGroups(t *testing.T) {
	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: {ActiveGroupID: 200},
		3: {ActiveGroupID: 300},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 3)
}

func TestExtractActiveGroupIDs_DuplicateGroups(t *testing.T) {
	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: {ActiveGroupID: 100}, // Same group
		3: {ActiveGroupID: 200},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 2) // Only unique group IDs
}

func TestExtractActiveGroupIDs_ZeroGroupID(t *testing.T) {
	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: {ActiveGroupID: 0}, // Zero should be skipped
		3: {ActiveGroupID: 200},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 2)
}

func TestExtractActiveGroupIDs_NilVisit(t *testing.T) {
	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: nil, // Nil should be skipped
		3: {ActiveGroupID: 200},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 2)
}

// =============================================================================
// coalesceMap Tests
// =============================================================================

func TestCoalesceMap_NonNilPrimary(t *testing.T) {
	primary := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1},
	}
	fallback := map[int64]*activeService.AttendanceStatus{
		2: {StudentID: 2},
	}
	result := coalesceMap(primary, fallback)
	assert.Equal(t, primary, result)
}

func TestCoalesceMap_NilPrimary(t *testing.T) {
	fallback := map[int64]*activeService.AttendanceStatus{
		2: {StudentID: 2},
	}
	result := coalesceMap(nil, fallback)
	assert.Equal(t, fallback, result)
}

func TestCoalesceMap_BothNil(t *testing.T) {
	result := coalesceMap(nil, nil)
	assert.Nil(t, result)
}

// =============================================================================
// coalesceVisitMap Tests
// =============================================================================

func TestCoalesceVisitMap_NonNilPrimary(t *testing.T) {
	primary := map[int64]*activeModels.Visit{
		1: {StudentID: 1},
	}
	fallback := map[int64]*activeModels.Visit{
		2: {StudentID: 2},
	}
	result := coalesceVisitMap(primary, fallback)
	assert.Equal(t, primary, result)
}

func TestCoalesceVisitMap_NilPrimary(t *testing.T) {
	fallback := map[int64]*activeModels.Visit{
		2: {StudentID: 2},
	}
	result := coalesceVisitMap(nil, fallback)
	assert.Equal(t, fallback, result)
}

func TestCoalesceVisitMap_BothNil(t *testing.T) {
	result := coalesceVisitMap(nil, nil)
	assert.Nil(t, result)
}

// =============================================================================
// coalesceGroupMap Tests
// =============================================================================

func TestCoalesceGroupMap_NonNilPrimary(t *testing.T) {
	primary := map[int64]*activeModels.Group{
		1: {GroupID: 1},
	}
	fallback := map[int64]*activeModels.Group{
		2: {GroupID: 2},
	}
	result := coalesceGroupMap(primary, fallback)
	assert.Equal(t, primary, result)
}

func TestCoalesceGroupMap_NilPrimary(t *testing.T) {
	fallback := map[int64]*activeModels.Group{
		2: {GroupID: 2},
	}
	result := coalesceGroupMap(nil, fallback)
	assert.Equal(t, fallback, result)
}

func TestCoalesceGroupMap_BothNil(t *testing.T) {
	result := coalesceGroupMap(nil, nil)
	assert.Nil(t, result)
}

// =============================================================================
// newEmptyLocationSnapshot Tests
// =============================================================================

func TestNewEmptyLocationSnapshot(t *testing.T) {
	snapshot := newEmptyLocationSnapshot()

	assert.NotNil(t, snapshot)
	assert.NotNil(t, snapshot.Attendances)
	assert.NotNil(t, snapshot.Visits)
	assert.NotNil(t, snapshot.Groups)
	assert.Empty(t, snapshot.Attendances)
	assert.Empty(t, snapshot.Visits)
	assert.Empty(t, snapshot.Groups)
}

// =============================================================================
// StudentLocationInfo Tests (internals)
// =============================================================================

func TestStudentLocationInfo_ZeroValue(t *testing.T) {
	var info StudentLocationInfo
	assert.Empty(t, info.Location)
	assert.Nil(t, info.Since)
}

func TestStudentLocationInfo_WithValues(t *testing.T) {
	now := time.Now()
	info := StudentLocationInfo{
		Location: "Test Location",
		Since:    &now,
	}
	assert.Equal(t, "Test Location", info.Location)
	assert.Equal(t, &now, info.Since)
}
