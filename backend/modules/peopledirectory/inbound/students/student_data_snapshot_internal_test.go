package students

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// The list snapshot moved here from api/common with the student routes
// (#2731); these are its former api/common tests on the persons and
// locations it still carries.

type failedSnapshotPeople struct {
	peopleModule.Capability
	err error
}

func (p failedSnapshotPeople) ListPersonsByID(context.Context, []int64) ([]peopleModule.Person, error) {
	return nil, p.err
}

func TestLoadStudentDataSnapshotPropagatesPersonReadFailure(t *testing.T) {
	t.Parallel()
	injected := errors.New("snapshot read unavailable")
	rs := &Resource{ResourceConfig: ResourceConfig{PeopleDirectory: failedSnapshotPeople{err: injected}}}
	snapshot, err := rs.loadStudentDataSnapshot(context.Background(), nil, []int64{123})
	require.ErrorIs(t, err, injected)
	assert.Nil(t, snapshot, "a failed read must not yield a partial snapshot")
}

func TestStudentDataSnapshotGetPerson(t *testing.T) {
	t.Parallel()

	var missing *studentDataSnapshot
	assert.Nil(t, missing.GetPerson(123))
	assert.Nil(t, (&studentDataSnapshot{}).GetPerson(123))

	snapshot := &studentDataSnapshot{Persons: map[int64]*peopleModule.Person{
		123: {ID: 123, FirstName: "Bob", LastName: "Jones"},
	}}
	assert.Nil(t, snapshot.GetPerson(999))
	person := snapshot.GetPerson(123)
	require.NotNil(t, person)
	assert.Equal(t, "Bob", person.FirstName)
	assert.Equal(t, "Jones", person.LastName)
}

func TestStudentDataSnapshotResolveLocationWithTime(t *testing.T) {
	t.Parallel()

	var missing *studentDataSnapshot
	info := missing.ResolveLocationWithTime(123, true)
	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)

	info = (&studentDataSnapshot{}).ResolveLocationWithTime(123, true)
	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)

	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)
	snapshot := &studentDataSnapshot{LocationSnapshot: &common.StudentLocationSnapshot{
		Attendances: map[int64]*studentpresence.DailyAttendanceStatus{
			123: {StudentID: 123, Status: "checked_in", CheckInTime: &checkinTime},
		},
		Visits: map[int64]*studentpresence.Visit{
			123: {StudentID: 123, ActiveGroupID: 456, EntryTime: entryTime},
		},
		Groups: map[int64]*studentpresence.SessionDetail{
			456: {
				ActivityGroupID: ptrtest.Ptr(int64(789)), RoomID: 1, StartTime: startTime,
				Room: &studentpresence.SessionRoomSummary{Name: "Science Lab"},
			},
		},
	}}

	info = snapshot.ResolveLocationWithTime(123, true)
	assert.Equal(t, "Anwesend - Science Lab", info.Location)
	require.NotNil(t, info.Since)
	assert.Equal(t, entryTime, *info.Since)

	limited := snapshot.ResolveLocationWithTime(123, false)
	assert.Equal(t, "Anwesend", limited.Location)
	assert.Nil(t, limited.Since)
}
