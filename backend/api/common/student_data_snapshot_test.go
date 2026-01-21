package common_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// StudentDataSnapshot.GetPerson Tests
// =============================================================================

func TestStudentDataSnapshot_GetPerson_NilSnapshot(t *testing.T) {
	var snapshot *common.StudentDataSnapshot = nil

	person := snapshot.GetPerson(123)

	assert.Nil(t, person)
}

func TestStudentDataSnapshot_GetPerson_NilPersonsMap(t *testing.T) {
	snapshot := &common.StudentDataSnapshot{
		Persons: nil,
		Groups:  make(map[int64]*educationModels.Group),
	}

	person := snapshot.GetPerson(123)

	assert.Nil(t, person)
}

func TestStudentDataSnapshot_GetPerson_PersonNotFound(t *testing.T) {
	snapshot := &common.StudentDataSnapshot{
		Persons: map[int64]*userModels.Person{
			1: {FirstName: "Alice", LastName: "Smith"},
		},
		Groups: make(map[int64]*educationModels.Group),
	}

	person := snapshot.GetPerson(999) // ID not in map

	assert.Nil(t, person)
}

func TestStudentDataSnapshot_GetPerson_PersonFound(t *testing.T) {
	testPerson := &userModels.Person{
		FirstName: "Bob",
		LastName:  "Jones",
	}
	testPerson.ID = 123

	snapshot := &common.StudentDataSnapshot{
		Persons: map[int64]*userModels.Person{
			123: testPerson,
		},
		Groups: make(map[int64]*educationModels.Group),
	}

	person := snapshot.GetPerson(123)

	assert.NotNil(t, person)
	assert.Equal(t, "Bob", person.FirstName)
	assert.Equal(t, "Jones", person.LastName)
}

// =============================================================================
// StudentDataSnapshot.GetGroup Tests
// =============================================================================

func TestStudentDataSnapshot_GetGroup_NilSnapshot(t *testing.T) {
	var snapshot *common.StudentDataSnapshot = nil

	group := snapshot.GetGroup(123)

	assert.Nil(t, group)
}

func TestStudentDataSnapshot_GetGroup_NilGroupsMap(t *testing.T) {
	snapshot := &common.StudentDataSnapshot{
		Persons: make(map[int64]*userModels.Person),
		Groups:  nil,
	}

	group := snapshot.GetGroup(123)

	assert.Nil(t, group)
}

func TestStudentDataSnapshot_GetGroup_GroupNotFound(t *testing.T) {
	snapshot := &common.StudentDataSnapshot{
		Persons: make(map[int64]*userModels.Person),
		Groups: map[int64]*educationModels.Group{
			1: {Name: "Group A"},
		},
	}

	group := snapshot.GetGroup(999) // ID not in map

	assert.Nil(t, group)
}

func TestStudentDataSnapshot_GetGroup_GroupFound(t *testing.T) {
	testGroup := &educationModels.Group{
		Name: "Class 1A",
	}
	testGroup.ID = 456

	snapshot := &common.StudentDataSnapshot{
		Persons: make(map[int64]*userModels.Person),
		Groups: map[int64]*educationModels.Group{
			456: testGroup,
		},
	}

	group := snapshot.GetGroup(456)

	assert.NotNil(t, group)
	assert.Equal(t, "Class 1A", group.Name)
}

// =============================================================================
// StudentDataSnapshot.ResolveLocationWithTime Tests
// =============================================================================

func TestStudentDataSnapshot_ResolveLocationWithTime_NilSnapshot(t *testing.T) {
	var snapshot *common.StudentDataSnapshot = nil

	info := snapshot.ResolveLocationWithTime(123, true)

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)
}

func TestStudentDataSnapshot_ResolveLocationWithTime_NilLocationSnapshot(t *testing.T) {
	snapshot := &common.StudentDataSnapshot{
		Persons:          make(map[int64]*userModels.Person),
		Groups:           make(map[int64]*educationModels.Group),
		LocationSnapshot: nil,
	}

	info := snapshot.ResolveLocationWithTime(123, true)

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)
}

func TestStudentDataSnapshot_ResolveLocationWithTime_WithLocationSnapshot(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)

	locationSnapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: map[int64]*activeModels.Group{
			456: {
				GroupID:   789,
				RoomID:    1,
				StartTime: startTime,
				Room: &facilities.Room{
					Name: "Science Lab",
				},
			},
		},
	}

	snapshot := &common.StudentDataSnapshot{
		Persons:          make(map[int64]*userModels.Person),
		Groups:           make(map[int64]*educationModels.Group),
		LocationSnapshot: locationSnapshot,
	}

	info := snapshot.ResolveLocationWithTime(123, true)

	assert.Equal(t, "Anwesend - Science Lab", info.Location)
	assert.NotNil(t, info.Since)
	assert.Equal(t, entryTime, *info.Since)
}

func TestStudentDataSnapshot_ResolveLocationWithTime_NoFullAccess(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)

	locationSnapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	snapshot := &common.StudentDataSnapshot{
		Persons:          make(map[int64]*userModels.Person),
		Groups:           make(map[int64]*educationModels.Group),
		LocationSnapshot: locationSnapshot,
	}

	info := snapshot.ResolveLocationWithTime(123, false)

	assert.Equal(t, "Anwesend", info.Location) // Limited info without full access
	assert.Nil(t, info.Since)
}

// =============================================================================
// Complete Snapshot Tests
// =============================================================================

func TestStudentDataSnapshot_CompleteScenario(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)

	// Create persons
	person1 := &userModels.Person{FirstName: "Anna", LastName: "Miller"}
	person1.ID = 1
	person2 := &userModels.Person{FirstName: "Ben", LastName: "Wilson"}
	person2.ID = 2

	// Create education groups
	eduGroup1 := &educationModels.Group{Name: "Class 1A"}
	eduGroup1.ID = 10
	eduGroup2 := &educationModels.Group{Name: "Class 2B"}
	eduGroup2.ID = 20

	// Create location snapshot
	locationSnapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			100: {StudentID: 100, Status: "checked_in", CheckInTime: &checkinTime},
			200: {StudentID: 200, Status: "not_checked_in"},
		},
		Visits: map[int64]*activeModels.Visit{
			100: {StudentID: 100, ActiveGroupID: 500, EntryTime: entryTime},
		},
		Groups: map[int64]*activeModels.Group{
			500: {
				GroupID: 10, RoomID: 1, StartTime: startTime,
				Room: &facilities.Room{Name: "Library"},
			},
		},
	}

	snapshot := &common.StudentDataSnapshot{
		Persons: map[int64]*userModels.Person{
			1: person1,
			2: person2,
		},
		Groups: map[int64]*educationModels.Group{
			10: eduGroup1,
			20: eduGroup2,
		},
		LocationSnapshot: locationSnapshot,
	}

	// Test GetPerson
	p1 := snapshot.GetPerson(1)
	assert.NotNil(t, p1)
	assert.Equal(t, "Anna", p1.FirstName)

	p3 := snapshot.GetPerson(999)
	assert.Nil(t, p3)

	// Test GetGroup
	g1 := snapshot.GetGroup(10)
	assert.NotNil(t, g1)
	assert.Equal(t, "Class 1A", g1.Name)

	g3 := snapshot.GetGroup(999)
	assert.Nil(t, g3)

	// Test ResolveLocationWithTime
	loc100 := snapshot.ResolveLocationWithTime(100, true)
	assert.Equal(t, "Anwesend - Library", loc100.Location)
	assert.NotNil(t, loc100.Since)

	loc200 := snapshot.ResolveLocationWithTime(200, true)
	assert.Equal(t, "Abwesend", loc200.Location)
	assert.Nil(t, loc200.Since)
}
