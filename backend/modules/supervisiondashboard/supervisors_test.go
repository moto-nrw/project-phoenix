package supervisiondashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The page head names who supervises the open session or room (#4354): every
// session tab and every session of a released room carries its supervisors'
// names, read in one bulk call for all of them.
func TestSupervisorsAreNamedForSessionsAndOpenRooms(t *testing.T) {
	t.Parallel()

	f, _ := openRoomFakes()
	f.supervised = func(context.Context) ([]Session, error) {
		return []Session{{ID: 31, Name: "Lese-AG"}, {ID: 32, Name: "Werken"}}, nil
	}
	var askedSessions [][]int64
	f.supervisorIDs = func(_ context.Context, ids []int64) (map[int64][]int64, error) {
		askedSessions = append(askedSessions, ids)
		return map[int64][]int64{31: {9, 7}}, nil
	}
	var askedStaff [][]int64
	f.staffNames = func(_ context.Context, ids []int64) (map[int64]string, error) {
		askedStaff = append(askedStaff, ids)
		// Staff 9 has no name any more; it drops out instead of showing blank.
		return map[int64]string{7: "Zora Zahn", 8: "Anna Alt"}, nil
	}

	projection, err := newService(f).Dashboard(context.Background(), 0)
	require.NoError(t, err)

	require.Len(t, askedSessions, 1)
	assert.ElementsMatch(t, []int64{31, 32}, askedSessions[0])
	require.Len(t, askedStaff, 1, "one name read for tabs and open rooms together")
	assert.ElementsMatch(t, []int64{7, 8, 9}, askedStaff[0])

	require.Len(t, projection.Groups, 2)
	assert.Equal(t, []SessionSupervisor{{StaffID: 7, Name: "Zora Zahn"}}, projection.Groups[0].Supervisors)
	assert.Nil(t, projection.Groups[1].Supervisors)

	yard := openRoomByName(t, projection, "Schulhof")
	bySession := map[int64][]SessionSupervisor{}
	for _, session := range yard.Sessions {
		bySession[session.ActiveGroupID] = session.Supervisors
	}
	// Sorted by name, not by staff id.
	assert.Equal(t, []SessionSupervisor{{StaffID: 8, Name: "Anna Alt"}, {StaffID: 7, Name: "Zora Zahn"}}, bySession[11])
	assert.Equal(t, []SessionSupervisor{{StaffID: 8, Name: "Anna Alt"}}, bySession[12])
	assert.Contains(t, bySession, int64(14))
	assert.Nil(t, bySession[14], "a kiosk session without supervisor names nobody")
}

// Without any supervisor there is nothing to name and no name read.
func TestSupervisorNamesSkipTheReadWithoutSupervisors(t *testing.T) {
	t.Parallel()

	f := &fakes{
		supervised: func(context.Context) ([]Session, error) { return []Session{{ID: 31}}, nil },
		staffNames: func(context.Context, []int64) (map[int64]string, error) {
			t.Fatal("no staff to name")
			return nil, nil
		},
	}

	projection, err := newService(f).Dashboard(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, projection.Groups, 1)
	assert.Nil(t, projection.Groups[0].Supervisors)
}

func TestSupervisorNamesFailTheRequest(t *testing.T) {
	t.Parallel()

	f := &fakes{supervised: func(context.Context) ([]Session, error) { return []Session{{ID: 31}}, nil }}
	f.supervisorIDs = func(context.Context, []int64) (map[int64][]int64, error) {
		return nil, errors.New("boom")
	}
	_, err := newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "load session supervisors")

	f.supervisorIDs = func(context.Context, []int64) (map[int64][]int64, error) {
		return map[int64][]int64{31: {7}}, nil
	}
	f.staffNames = func(context.Context, []int64) (map[int64]string, error) {
		return nil, errors.New("boom")
	}
	_, err = newService(f).Dashboard(context.Background(), 0)
	require.ErrorContains(t, err, "load supervisor names")
}
