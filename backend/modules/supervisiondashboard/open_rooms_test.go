package supervisiondashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRoomFakes releases a yard with two blocks, the room's own session and
// a kiosk session, and a hall with one block. The caller (staff 7) supervises
// the first block and the room's own session.
func openRoomFakes() (*fakes, *[]SessionBlocksQuery) {
	start := time.Date(2026, time.August, 19, 11, 0, 0, 0, time.UTC)
	entry := start.Add(10 * time.Minute)
	var asked []SessionBlocksQuery
	f := &fakes{
		currentStaffID: func(context.Context) (*int64, error) { return int64Ptr(7), nil },
		caller: func(context.Context) (Caller, error) {
			return Caller{AccountID: 70, TokenAdmin: true, CanReadSchedules: true}, nil
		},
		released: func(context.Context) ([]ReleasedRoom, error) {
			return []ReleasedRoom{{ID: 1, Name: "Schulhof"}, {ID: 2, Name: "Halle"}}, nil
		},
		inRooms: func(context.Context, []int64) ([]RunningSession, error) {
			return []RunningSession{
				{ActiveGroupID: 12, RoomID: 1, ActivityName: "GT 2", StartTime: start.Add(time.Minute), SupervisorStaffIDs: []int64{8}},
				{ActiveGroupID: 11, RoomID: 1, ActivityName: "GT 1", StartTime: start, SupervisorStaffIDs: []int64{7, 8}},
				{ActiveGroupID: 13, RoomID: 1, ActivityName: "Schulhof Freispiel", IndependentStays: true, StartTime: start.Add(2 * time.Minute), SupervisorStaffIDs: []int64{7}},
				{ActiveGroupID: 14, RoomID: 1, ActivityName: "Kiosk", StartTime: start.Add(3 * time.Minute)},
				{ActiveGroupID: 21, RoomID: 2, ActivityName: "Tanzen", StartTime: start, SupervisorStaffIDs: []int64{9}},
			}, nil
		},
		openVisits: func(context.Context, []int64) ([]VisitRecord, error) {
			return []VisitRecord{
				{StudentID: 101, ActiveGroupID: 11, EntryTime: entry, FirstName: "Anna"},
				{StudentID: 102, ActiveGroupID: 11, EntryTime: entry, FirstName: "Ben"},
				{StudentID: 103, ActiveGroupID: 13, EntryTime: entry, FirstName: "Cem"},
				{StudentID: 104, ActiveGroupID: 14, EntryTime: entry, FirstName: "Dana"},
				{StudentID: 105, ActiveGroupID: 21, EntryTime: entry, FirstName: "Eli"},
			}, nil
		},
	}
	f.sessionBlocks = func(_ context.Context, query SessionBlocksQuery) ([]SessionBlock, error) {
		asked = append(asked, query)
		return []SessionBlock{
			{ActiveGroupID: 11, InstanceID: 511, Title: "Garten-AG", StartTime: "13:00", EndTime: "14:00", CanOperate: true},
			{ActiveGroupID: 12, InstanceID: 512, Title: "Fußball", StartTime: "13:00", EndTime: "14:30", IsAssigned: true, CanOperate: true},
			{ActiveGroupID: 21, InstanceID: 521, Title: "Tanzen", StartTime: "13:00", EndTime: "14:00"},
			// An answer the projection did not ask for is ignored.
			{ActiveGroupID: 13, InstanceID: 513, Title: "Schulhof", StartTime: "13:00", EndTime: "18:00", CanOperate: true},
		}, nil
	}
	return f, &asked
}

func openRoomByName(t *testing.T, projection *Projection, name string) OpenRoom {
	t.Helper()
	for _, room := range projection.OpenRooms {
		if room.Name == name {
			return room
		}
	}
	t.Fatalf("open room %q missing", name)
	return OpenRoom{}
}

// A released room lists its sessions with what each section needs (#3281):
// blocks carry their instance and the caller's rights, the room's own session
// and a kiosk session carry none, and the counts add up to the room's.
func TestOpenRoomSessionsProjectBlocksAndCounts(t *testing.T) {
	t.Parallel()

	f, asked := openRoomFakes()

	projection, err := newService(f).Dashboard(context.Background(), 0)

	require.NoError(t, err)
	require.Len(t, *asked, 1, "one block read for every released room together")
	query := (*asked)[0]
	assert.Equal(t, int64(70), query.AccountID)
	assert.True(t, query.TokenAdmin)
	assert.Equal(t, projection.BusinessDay, query.Date)
	assert.Equal(t, map[int64][]int64{11: {7, 8}, 12: {8}, 14: nil, 21: {9}}, query.Supervisors,
		"the room's own session is not asked about")

	yard := openRoomByName(t, projection, "Schulhof")
	assert.Equal(t, 4, yard.StudentCount)
	assert.True(t, yard.IsUserSupervising)
	assert.Equal(t, []OpenRoomSession{
		{ActiveGroupID: 11, Title: "Garten-AG", IsUserSupervising: true, CanAssign: true, StudentCount: 2,
			Block: &OpenRoomBlock{InstanceID: 511, StartTime: "13:00", EndTime: "14:00", CanOperate: true}},
		{ActiveGroupID: 12, Title: "Fußball", StudentCount: 0,
			Block: &OpenRoomBlock{InstanceID: 512, StartTime: "13:00", EndTime: "14:30", IsUserAssigned: true, CanOperate: true}},
		{ActiveGroupID: 13, Independent: true, IsUserSupervising: true, CanAssign: true, StudentCount: 1},
		{ActiveGroupID: 14, Title: "Kiosk", StudentCount: 1},
	}, yard.Sessions, "sessions in start order, the block's title wins over the activity's")

	hall := openRoomByName(t, projection, "Halle")
	require.Len(t, hall.Sessions, 1)
	assert.False(t, hall.IsUserSupervising)
	assert.False(t, hall.Sessions[0].CanAssign)
	assert.False(t, hall.Sessions[0].Block.CanOperate)
	assert.Equal(t, 1, hall.Sessions[0].StudentCount)
}

func TestOpenRoomSessionsAdminAssignsEverySession(t *testing.T) {
	t.Parallel()

	f, _ := openRoomFakes()
	f.caller = func(context.Context) (Caller, error) {
		return Caller{AccountID: 70, AdminScope: true, CanReadSchedules: true}, nil
	}

	projection, err := newService(f).Dashboard(context.Background(), 0)

	require.NoError(t, err)
	for _, room := range projection.OpenRooms {
		for _, session := range room.Sessions {
			assert.True(t, session.CanAssign, "session %d", session.ActiveGroupID)
		}
	}
}

func TestOpenRoomSessionsWithoutScheduleReadNameNoBlock(t *testing.T) {
	t.Parallel()

	f, asked := openRoomFakes()
	f.caller = func(context.Context) (Caller, error) { return Caller{AccountID: 70}, nil }

	projection, err := newService(f).Dashboard(context.Background(), 0)

	require.NoError(t, err)
	assert.Empty(t, *asked, "no schedules:read, no block read")
	yard := openRoomByName(t, projection, "Schulhof")
	require.Len(t, yard.Sessions, 4)
	for _, session := range yard.Sessions {
		assert.Nil(t, session.Block, "session %d", session.ActiveGroupID)
	}
	assert.Equal(t, "GT 1", yard.Sessions[0].Title, "the activity still names the session")
	assert.Equal(t, 2, yard.Sessions[0].StudentCount)
}

func TestOpenRoomSessionsSkipTheBlockReadWithoutCandidates(t *testing.T) {
	t.Parallel()

	f, asked := openRoomFakes()
	f.inRooms = func(context.Context, []int64) ([]RunningSession, error) {
		return []RunningSession{{ActiveGroupID: 13, RoomID: 1, IndependentStays: true}}, nil
	}
	f.openVisits = nil

	projection, err := newService(f).Dashboard(context.Background(), 0)

	require.NoError(t, err)
	assert.Empty(t, *asked, "only the room's own session runs: nothing to ask")
	assert.Equal(t, []OpenRoomSession{{ActiveGroupID: 13, Independent: true}}, openRoomByName(t, projection, "Schulhof").Sessions)
	assert.Equal(t, []OpenRoomSession{}, openRoomByName(t, projection, "Halle").Sessions,
		"an empty released room lists no sessions, not null")
}

func TestOpenRoomSessionsFailTheWholeProjection(t *testing.T) {
	t.Parallel()

	f, _ := openRoomFakes()
	f.sessionBlocks = func(context.Context, SessionBlocksQuery) ([]SessionBlock, error) {
		return nil, errors.New("boom")
	}

	_, err := newService(f).Dashboard(context.Background(), 0)

	require.ErrorContains(t, err, "load open-room blocks")
}

// A session section carries its activity's participant limit so the page can
// show "Anzahl / Grenze" and flag an overbooked session (#3634); a session
// without a limit carries none.
func TestOpenRoomSessionsCarryTheParticipantLimit(t *testing.T) {
	t.Parallel()

	f, _ := openRoomFakes()
	inRooms := f.inRooms
	f.inRooms = func(ctx context.Context, roomIDs []int64) ([]RunningSession, error) {
		sessions, err := inRooms(ctx, roomIDs)
		for i := range sessions {
			if sessions[i].ActiveGroupID == 11 {
				sessions[i].ParticipantLimit = intPtr(1)
			}
		}
		return sessions, err
	}

	projection, err := newService(f).Dashboard(context.Background(), 0)

	require.NoError(t, err)
	yard := openRoomByName(t, projection, "Schulhof")
	require.Equal(t, int64(11), yard.Sessions[0].ActiveGroupID)
	require.NotNil(t, yard.Sessions[0].ParticipantLimit)
	assert.Equal(t, 1, *yard.Sessions[0].ParticipantLimit)
	assert.Equal(t, 2, yard.Sessions[0].StudentCount, "two children against a limit of one")
	assert.Nil(t, yard.Sessions[1].ParticipantLimit)
}

func intPtr(v int) *int { return &v }
