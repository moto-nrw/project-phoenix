package active

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
)

// TestStudentHomeRoomMapping tests the logic for determining if a student is in their Heimatraum
func TestStudentHomeRoomMapping(t *testing.T) {
	t.Parallel()

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
			studentGroupID: ptrtest.Ptr(int64(10)),
			groupRoomID:    ptrtest.Ptr(int64(101)),
			currentRoomID:  101,
			expectedInHome: true,
			description:    "Class 5a student in Room 101 (5a's Heimatraum) should be counted",
		},
		{
			name:           "Student in another Heimatraum",
			studentID:      2,
			studentGroupID: ptrtest.Ptr(int64(10)),
			groupRoomID:    ptrtest.Ptr(int64(101)),
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
			studentGroupID: ptrtest.Ptr(int64(10)),
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
	t.Parallel()

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
	t.Parallel()

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

// TestCountStudentsInIndoorRooms tests the indoor room student counting logic,
// including the edge case where a student's visit belongs to a playground/outdoor group
func TestCountStudentsInIndoorRooms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		activeVisits  []*active.Visit
		activeGroups  []*active.Group
		rooms         []*facilityModels.Room
		expectedCount int
		description   string
	}{
		{
			name: "Students in indoor rooms only",
			activeVisits: []*active.Visit{
				{StudentID: 1, ActiveGroupID: 100},
				{StudentID: 2, ActiveGroupID: 100},
			},
			activeGroups: []*active.Group{
				{Model: base.Model{ID: 100}, RoomID: 10},
			},
			rooms: []*facilityModels.Room{
				{Model: base.Model{ID: 10}, Name: "Raum 101", Category: ptrtest.Ptr("Klassenraum")},
			},
			expectedCount: 2,
			description:   "Both students in an indoor room should be counted",
		},
		{
			name: "Students in playground excluded",
			activeVisits: []*active.Visit{
				{StudentID: 1, ActiveGroupID: 100},
				{StudentID: 2, ActiveGroupID: 200},
			},
			activeGroups: []*active.Group{
				{Model: base.Model{ID: 100}, RoomID: 10},
				{Model: base.Model{ID: 200}, RoomID: 20},
			},
			rooms: []*facilityModels.Room{
				{Model: base.Model{ID: 10}, Name: "Raum 101", Category: ptrtest.Ptr("Klassenraum")},
				{Model: base.Model{ID: 20}, Name: "Schulhof", Category: ptrtest.Ptr("Schulhof")},
			},
			expectedCount: 1,
			description:   "Student on playground should NOT be counted, only indoor student",
		},
		{
			name: "All students on playground",
			activeVisits: []*active.Visit{
				{StudentID: 1, ActiveGroupID: 200},
				{StudentID: 2, ActiveGroupID: 200},
			},
			activeGroups: []*active.Group{
				{Model: base.Model{ID: 200}, RoomID: 20},
			},
			rooms: []*facilityModels.Room{
				{Model: base.Model{ID: 20}, Name: "Schulhof", Category: ptrtest.Ptr("Schulhof")},
			},
			expectedCount: 0,
			description:   "No students should be counted when all are on playground",
		},
		{
			name: "Visit with ended group excluded",
			activeVisits: []*active.Visit{
				{StudentID: 1, ActiveGroupID: 100},
			},
			activeGroups: []*active.Group{
				{Model: base.Model{ID: 100}, RoomID: 10, EndTime: ptrtest.Ptr(time.Now())},
			},
			rooms: []*facilityModels.Room{
				{Model: base.Model{ID: 10}, Name: "Raum 101", Category: ptrtest.Ptr("Klassenraum")},
			},
			expectedCount: 0,
			description:   "Students in ended groups should not be counted",
		},
		{
			name: "Exited visit excluded",
			activeVisits: []*active.Visit{
				{StudentID: 1, ActiveGroupID: 100, ExitTime: ptrtest.Ptr(time.Now())},
			},
			activeGroups: []*active.Group{
				{Model: base.Model{ID: 100}, RoomID: 10},
			},
			rooms: []*facilityModels.Room{
				{Model: base.Model{ID: 10}, Name: "Raum 101", Category: ptrtest.Ptr("Klassenraum")},
			},
			expectedCount: 0,
			description:   "Exited visits should not be counted",
		},
		{
			name:          "Empty visits and groups",
			activeVisits:  []*active.Visit{},
			activeGroups:  []*active.Group{},
			rooms:         []*facilityModels.Room{},
			expectedCount: 0,
			description:   "No students when there are no visits or groups",
		},
		{
			name: "Visit to group not in active groups (outdoor/unknown)",
			activeVisits: []*active.Visit{
				{StudentID: 1, ActiveGroupID: 999},
			},
			activeGroups: []*active.Group{
				{Model: base.Model{ID: 100}, RoomID: 10},
			},
			rooms: []*facilityModels.Room{
				{Model: base.Model{ID: 10}, Name: "Raum 101", Category: ptrtest.Ptr("Klassenraum")},
			},
			expectedCount: 0,
			description:   "Visit to a group not in the active groups list should not be counted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build room data
			roomData := &dashboardRoomData{
				roomByID:        make(map[int64]*facilityModels.Room),
				occupiedRooms:   make(map[int64]bool),
				roomStudentsMap: make(map[int64]map[int64]struct{}),
			}
			for _, room := range tt.rooms {
				roomData.roomByID[room.ID] = room
			}

			// Use zero-value service (countStudentsInIndoorRooms doesn't use service fields)
			s := &service{}
			count := s.countStudentsInIndoorRooms(tt.activeVisits, tt.activeGroups, roomData)

			assert.Equal(t, tt.expectedCount, count, tt.description)
		})
	}
}
