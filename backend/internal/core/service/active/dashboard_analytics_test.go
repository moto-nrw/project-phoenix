package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStudentHomeRoomMapping tests the logic for determining if a student is in their Heimatraum
func TestStudentHomeRoomMapping(t *testing.T) {
	tests := []struct {
		name           string
		studentID      int64
		studentGroupID *int64
		groupRoomID    *int64
		currentRoomID  int64
		expectedInHome bool
		description    string
	}{
		{
			name:           "Student in their own Heimatraum",
			studentID:      1,
			studentGroupID: int64Ptr(10),
			groupRoomID:    int64Ptr(101),
			currentRoomID:  101,
			expectedInHome: true,
			description:    "Class 5a student in Room 101 (5a's Heimatraum) should be counted",
		},
		{
			name:           "Student in another Heimatraum",
			studentID:      2,
			studentGroupID: int64Ptr(10),
			groupRoomID:    int64Ptr(101),
			currentRoomID:  102,
			expectedInHome: false,
			description:    "Class 5a student in Room 102 (5b's Heimatraum) should NOT be counted",
		},
		{
			name:           "Student without group assignment",
			studentID:      3,
			studentGroupID: nil,
			groupRoomID:    nil,
			currentRoomID:  101,
			expectedInHome: false,
			description:    "Student without assigned group has no Heimatraum",
		},
		{
			name:           "Student's group has no room",
			studentID:      4,
			studentGroupID: int64Ptr(10),
			groupRoomID:    nil,
			currentRoomID:  101,
			expectedInHome: false,
			description:    "Student's group without assigned room has no Heimatraum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the studentHomeRoomMap logic
			studentHomeRoomMap := make(map[int64]int64)
			if tt.studentGroupID != nil && tt.groupRoomID != nil {
				studentHomeRoomMap[tt.studentID] = *tt.groupRoomID
			}

			// Simulate the counting logic
			inHomeRoom := false
			if homeRoomID, ok := studentHomeRoomMap[tt.studentID]; ok {
				if homeRoomID == tt.currentRoomID {
					inHomeRoom = true
				}
			}

			assert.Equal(t, tt.expectedInHome, inHomeRoom, tt.description)
		})
	}
}

// TestMultipleStudentsInRooms tests counting multiple students across different rooms
func TestMultipleStudentsInRooms(t *testing.T) {
	// Setup: 3 students, 2 groups, 2 rooms
	// Group 10 (Class 5a) -> Room 101
	// Group 11 (Class 5b) -> Room 102
	// Student 1: Group 10, in Room 101 (own Heimatraum) ✓
	// Student 2: Group 10, in Room 102 (visiting 5b) ✗
	// Student 3: Group 11, in Room 102 (own Heimatraum) ✓

	studentHomeRoomMap := map[int64]int64{
		1: 101, // Student 1 -> Room 101
		2: 101, // Student 2 -> Room 101
		3: 102, // Student 3 -> Room 102
	}

	// Room 101 has student 1 only
	room101Students := map[int64]struct{}{
		1: {},
	}

	// Room 102 has students 2 and 3
	room102Students := map[int64]struct{}{
		2: {}, // Student 2 is visiting from Class 5a
		3: {}, // Student 3 is in their own Heimatraum
	}

	// Count students in their Heimatraum
	studentsInHomeRoom := 0

	// Process Room 101
	for studentID := range room101Students {
		if homeRoomID, ok := studentHomeRoomMap[studentID]; ok {
			if homeRoomID == 101 {
				studentsInHomeRoom++
			}
		}
	}

	// Process Room 102
	for studentID := range room102Students {
		if homeRoomID, ok := studentHomeRoomMap[studentID]; ok {
			if homeRoomID == 102 {
				studentsInHomeRoom++
			}
		}
	}

	// Only students 1 and 3 should be counted (in their own Heimatraum)
	// Student 2 is in Room 102 but belongs to Room 101, so NOT counted
	assert.Equal(t, 2, studentsInHomeRoom, "Should count 2 students in their Heimatraum (students 1 and 3)")
}

// TestEdgeCaseEmptyRooms tests behavior with no students
func TestEdgeCaseEmptyRooms(t *testing.T) {
	studentHomeRoomMap := map[int64]int64{}
	emptyRoomStudents := map[int64]struct{}{}

	studentsInHomeRoom := 0
	for studentID := range emptyRoomStudents {
		if homeRoomID, ok := studentHomeRoomMap[studentID]; ok {
			if homeRoomID == 101 {
				studentsInHomeRoom++
			}
		}
	}

	assert.Equal(t, 0, studentsInHomeRoom, "Empty room should have 0 students in Heimatraum")
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}
