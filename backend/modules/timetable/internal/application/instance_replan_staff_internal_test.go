package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveStaffDirectory knows the live staff members; every other id is retired.
type liveStaffDirectory struct {
	ports.LifecycleStaff
	live    map[int64]bool
	queried [][]int64
}

func (d *liveStaffDirectory) FindByIDs(_ context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	d.queried = append(d.queried, ids)
	found := make(map[int64]*timetable.StaffMember)
	for _, id := range ids {
		if d.live[id] {
			found[id] = &timetable.StaffMember{ID: id}
		}
	}
	return found, nil
}

// A re-plan locks the people of the occurrences it deletes before the
// delete (#6583). Operational occurrences only name live people; a past
// occurrence may name a retired one, who cannot be offboarded again and is
// left out, while a live person of a past occurrence is locked too, because
// offboarding locks history as well.
func TestReplanLocksTheLivePeopleOfTheDeletedOccurrences(t *testing.T) {
	t.Parallel()
	past := &scheduleModel.ActivityInstance{Date: calendar.NewDate(2000, time.January, 3)}
	past.ID = 31
	future := &scheduleModel.ActivityInstance{Date: calendar.NewDate(2099, time.June, 9)}
	future.ID = 32
	rows := []*scheduleModel.InstanceStaff{
		{InstanceID: past.ID, StaffID: 41},
		{InstanceID: past.ID, StaffID: 42},
		{InstanceID: future.ID, StaffID: 43},
	}
	directory := &liveStaffDirectory{live: map[int64]bool{41: true}}
	s := &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{StaffRepo: directory}}

	staff, err := s.liveStaffOf(context.Background(), rows, []*scheduleModel.ActivityInstance{past, future})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{41, 43}, staff, "the live past person and the operational person; the retired one is left out")
	assert.Equal(t, [][]int64{{41, 42}}, directory.queried, "only the people of past occurrences are checked")
}
