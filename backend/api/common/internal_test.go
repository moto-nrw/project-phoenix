package common

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// filterCheckedInStudents Tests
// =============================================================================

func TestFilterCheckedInStudents_Empty(t *testing.T) {
	t.Parallel()

	result := filterCheckedInStudents(map[int64]*activeService.AttendanceStatus{})
	assert.Empty(t, result)
}

func TestFilterCheckedInStudents_AllCheckedIn(t *testing.T) {
	t.Parallel()

	attendances := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1, Status: "checked_in"},
		2: {StudentID: 2, Status: "checked_in"},
		3: {StudentID: 3, Status: "checked_in"},
	}
	result := filterCheckedInStudents(attendances)
	assert.Len(t, result, 3)
}

func TestFilterCheckedInStudents_NoneCheckedIn(t *testing.T) {
	t.Parallel()

	attendances := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1, Status: "not_checked_in"},
		2: {StudentID: 2, Status: "checked_out"},
	}
	result := filterCheckedInStudents(attendances)
	assert.Empty(t, result)
}

func TestFilterCheckedInStudents_Mixed(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	result := extractActiveGroupIDs(map[int64]*activeModels.Visit{})
	assert.Empty(t, result)
}

func TestExtractActiveGroupIDs_AllWithGroups(t *testing.T) {
	t.Parallel()

	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: {ActiveGroupID: 200},
		3: {ActiveGroupID: 300},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 3)
}

func TestExtractActiveGroupIDs_DuplicateGroups(t *testing.T) {
	t.Parallel()

	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: {ActiveGroupID: 100}, // Same group
		3: {ActiveGroupID: 200},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 2) // Only unique group IDs
}

func TestExtractActiveGroupIDs_ZeroGroupID(t *testing.T) {
	t.Parallel()

	visits := map[int64]*activeModels.Visit{
		1: {ActiveGroupID: 100},
		2: {ActiveGroupID: 0}, // Zero should be skipped
		3: {ActiveGroupID: 200},
	}
	result := extractActiveGroupIDs(visits)
	assert.Len(t, result, 2)
}

func TestExtractActiveGroupIDs_NilVisit(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	primary := map[int64]*activeService.AttendanceStatus{
		1: {StudentID: 1},
	}
	fallback := map[int64]*activeService.AttendanceStatus{
		2: {StudentID: 2},
	}
	result := coalesce(primary, fallback)
	assert.Equal(t, primary, result)
}

func TestCoalesceMap_NilPrimary(t *testing.T) {
	t.Parallel()

	fallback := map[int64]*activeService.AttendanceStatus{
		2: {StudentID: 2},
	}
	result := coalesce(nil, fallback)
	assert.Equal(t, fallback, result)
}

func TestCoalesceMap_BothNil(t *testing.T) {
	t.Parallel()

	result := coalesce[int64, *activeService.AttendanceStatus](nil, nil)
	assert.Nil(t, result)
}

// =============================================================================
// coalesceVisitMap Tests
// =============================================================================

func TestCoalesceVisitMap_NonNilPrimary(t *testing.T) {
	t.Parallel()

	primary := map[int64]*activeModels.Visit{
		1: {StudentID: 1},
	}
	fallback := map[int64]*activeModels.Visit{
		2: {StudentID: 2},
	}
	result := coalesce(primary, fallback)
	assert.Equal(t, primary, result)
}

func TestCoalesceVisitMap_NilPrimary(t *testing.T) {
	t.Parallel()

	fallback := map[int64]*activeModels.Visit{
		2: {StudentID: 2},
	}
	result := coalesce(nil, fallback)
	assert.Equal(t, fallback, result)
}

func TestCoalesceVisitMap_BothNil(t *testing.T) {
	t.Parallel()

	result := coalesce[int64, *activeModels.Visit](nil, nil)
	assert.Nil(t, result)
}

// =============================================================================
// coalesceGroupMap Tests
// =============================================================================

func TestCoalesceGroupMap_NonNilPrimary(t *testing.T) {
	t.Parallel()

	// Typed pointer literals keep the fixture-ID intent explicit.
	primary := map[int64]*activeModels.Group{
		101: {GroupID: ptrtest.Ptr(int64(101))},
	}
	fallback := map[int64]*activeModels.Group{
		102: {GroupID: ptrtest.Ptr(int64(102))},
	}
	result := coalesce(primary, fallback)
	assert.Equal(t, primary, result)
}

func TestCoalesceGroupMap_NilPrimary(t *testing.T) {
	t.Parallel()

	fallback := map[int64]*activeModels.Group{
		102: {GroupID: ptrtest.Ptr(int64(102))},
	}
	result := coalesce(nil, fallback)
	assert.Equal(t, fallback, result)
}

func TestCoalesceGroupMap_BothNil(t *testing.T) {
	t.Parallel()

	result := coalesce[int64, *activeModels.Group](nil, nil)
	assert.Nil(t, result)
}

// =============================================================================
// newEmptyLocationSnapshot Tests
// =============================================================================

func TestNewEmptyLocationSnapshot(t *testing.T) {
	t.Parallel()

	snapshot := newEmptyLocationSnapshot(PresenceModeDetailed)

	assert.NotNil(t, snapshot)
	assert.Equal(t, PresenceModeDetailed, snapshot.Mode)
	assert.NotNil(t, snapshot.Attendances)
	assert.NotNil(t, snapshot.Visits)
	assert.NotNil(t, snapshot.Groups)
	assert.Empty(t, snapshot.Attendances)
	assert.Empty(t, snapshot.Visits)
	assert.Empty(t, snapshot.Groups)
}

func TestNewEmptyLocationSnapshot_BinaryMode(t *testing.T) {
	t.Parallel()

	snapshot := newEmptyLocationSnapshot(PresenceModeBinary)
	assert.Equal(t, PresenceModeBinary, snapshot.Mode)
}

func TestNewEmptyLocationSnapshot_EmptyModeDefaultsToDetailed(t *testing.T) {
	t.Parallel()

	snapshot := newEmptyLocationSnapshot("")
	assert.Equal(t, PresenceModeDetailed, snapshot.Mode)
}

// =============================================================================
// StudentLocationInfo Tests (internals)
// =============================================================================

func TestStudentLocationInfo_ZeroValue(t *testing.T) {
	t.Parallel()

	var info StudentLocationInfo
	assert.Empty(t, info.Location)
	assert.Nil(t, info.Since)
}

func TestStudentLocationInfo_WithValues(t *testing.T) {
	t.Parallel()

	now := time.Now()
	info := StudentLocationInfo{
		Location: "Test Location",
		Since:    &now,
	}
	assert.Equal(t, "Test Location", info.Location)
	assert.Equal(t, &now, info.Since)
}
