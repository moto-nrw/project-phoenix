package timetable_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rosterQuery serves fixed instances and roster rows and records the filters.
type rosterQuery struct {
	instances       []timetable.ActivityInstance
	rows            []timetable.InstanceStudent
	instanceErr     error
	instanceFilters []timetable.ActivityInstanceFilter
	studentFilters  []timetable.InstanceStudentFilter
}

func (q *rosterQuery) ListActivityInstances(_ context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error) {
	q.instanceFilters = append(q.instanceFilters, filter)
	return q.instances, q.instanceErr
}

func (q *rosterQuery) ListInstanceStudents(_ context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error) {
	q.studentFilters = append(q.studentFilters, filter)
	return q.rows, nil
}

func TestSessionRostersMapsRosterRowsBackToSessions(t *testing.T) {
	t.Parallel()
	const studentID, sessionA, sessionB, instanceA, instanceB = int64(700), int64(21), int64(22), int64(31), int64(32)
	query := &rosterQuery{
		instances: []timetable.ActivityInstance{
			{ID: instanceA, ActiveGroupID: new(sessionA)},
			{ID: instanceB, ActiveGroupID: new(sessionB)},
			{ID: 33},
		},
		rows: []timetable.InstanceStudent{{InstanceID: instanceB, StudentID: studentID}},
	}

	sessions, err := timetable.SessionRosters{Query: query}.SessionsRosteringStudent(context.Background(), studentID, []int64{sessionA, sessionB})

	require.NoError(t, err)
	assert.Equal(t, []int64{sessionB}, sessions)
	require.Len(t, query.instanceFilters, 1)
	assert.Equal(t, []int64{sessionA, sessionB}, query.instanceFilters[0].ActiveGroupIDs)
	assert.Equal(t, timetable.InstanceStatusActive, query.instanceFilters[0].Status, "only running blocks count")
	require.Len(t, query.studentFilters, 1)
	assert.Equal(t, []int64{instanceA, instanceB}, query.studentFilters[0].InstanceIDs)
	assert.Equal(t, []int64{studentID}, query.studentFilters[0].StudentIDs)
}

func TestSessionRostersSkipsLookupsWithoutCandidates(t *testing.T) {
	t.Parallel()
	query := &rosterQuery{}
	rosters := timetable.SessionRosters{Query: query}

	sessions, err := rosters.SessionsRosteringStudent(context.Background(), 700, nil)
	require.NoError(t, err)
	assert.Empty(t, sessions)
	assert.Empty(t, query.instanceFilters, "no session, no query")

	sessions, err = rosters.SessionsRosteringStudent(context.Background(), 700, []int64{21})
	require.NoError(t, err)
	assert.Empty(t, sessions)
	assert.Empty(t, query.studentFilters, "no running instance, no roster query")
}

func TestSessionRostersReturnsLookupFailures(t *testing.T) {
	t.Parallel()
	boom := errors.New("database unavailable")

	_, err := timetable.SessionRosters{Query: &rosterQuery{instanceErr: boom}}.SessionsRosteringStudent(context.Background(), 700, []int64{21})

	require.ErrorIs(t, err, boom)
}
