package legacy

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/facilities"
)

type attendanceRoomRecords struct {
	AttendanceRoomRecords
	lock func(context.Context, int64) (*facilities.Room, error)
	list func(context.Context, map[string]interface{}) ([]*facilities.Room, error)
}

func (r attendanceRoomRecords) FindByIDForUpdate(ctx context.Context, id int64) (*facilities.Room, error) {
	return r.lock(ctx, id)
}

func (r attendanceRoomRecords) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.Room, error) {
	return r.list(ctx, filters)
}

func TestAttendanceRoomsDelegatesLockAndPreservesPartialError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lockErr := errors.New("room lock failed")
	calls := 0
	rooms := NewAttendanceRooms(attendanceRoomRecords{lock: func(got context.Context, id int64) (*facilities.Room, error) {
		calls++
		if got != ctx || id != 7 {
			t.Fatal("lock context or identity changed")
		}
		return &facilities.Room{ID: 7, TenantID: 9, Name: "Aula", IsOpenRoom: true}, lockErr
	}})
	room, err := rooms.FindByIDForUpdate(ctx, 7)
	if calls != 1 || !errors.Is(err, lockErr) {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
	if room == nil || room.ID != 7 || room.TenantID != 9 || room.Name != "Aula" || !room.IsOpenRoom {
		t.Fatalf("lost room facts: %+v", room)
	}
}

func TestAttendanceRoomListPreservesShapeAndFilters(t *testing.T) {
	t.Parallel()
	for _, rows := range [][]*facilities.Room{nil, {}, {nil, {ID: 7, IsOpenRoom: true}}} {
		lookupErr := errors.New("partial room list")
		rooms := NewAttendanceRooms(attendanceRoomRecords{list: func(_ context.Context, filters map[string]interface{}) ([]*facilities.Room, error) {
			if filters["name"] != "Aula" {
				t.Fatal("list filter changed")
			}
			return rows, lookupErr
		}})
		got, err := rooms.List(context.Background(), map[string]interface{}{"name": "Aula"})
		if !errors.Is(err, lookupErr) || (got == nil) != (rows == nil) || len(got) != len(rows) {
			t.Fatalf("list shape or error changed: %+v %v", got, err)
		}
		if len(got) > 0 && (got[0] != nil || got[1].ID != 7 || !got[1].IsOpenRoom) {
			t.Fatalf("partial room facts changed: %+v", got)
		}
	}
}
