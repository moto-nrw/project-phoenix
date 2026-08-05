package schedule

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type absorbGroupRepo struct {
	activeModel.GroupRepository
	openGroups []*activeModel.Group
	lockedIDs  []int64
	endedIDs   []int64
}

func (r *absorbGroupRepo) FindActiveByRoomID(_ context.Context, _ int64) ([]*activeModel.Group, error) {
	return r.openGroups, nil
}

func (r *absorbGroupRepo) FindByIDForUpdate(_ context.Context, id int64) (*activeModel.Group, error) {
	r.lockedIDs = append(r.lockedIDs, id)
	for _, group := range r.openGroups {
		if group.ID == id {
			return group, nil
		}
	}
	return nil, nil
}

func (r *absorbGroupRepo) EndSession(_ context.Context, id int64) error {
	r.endedIDs = append(r.endedIDs, id)
	return nil
}

type absorbSupervisorRepo struct {
	activeModel.GroupSupervisorRepository
	byGroup map[int64][]*activeModel.GroupSupervisor
}

func (r *absorbSupervisorRepo) FindByActiveGroupID(_ context.Context, groupID int64, _ bool) ([]*activeModel.GroupSupervisor, error) {
	return r.byGroup[groupID], nil
}

type absorbVisitRepo struct {
	activeModel.VisitRepository
	transferCounts map[int64]int
	transfers      [][2]int64
}

type absorbInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
	byGroup map[int64]*scheduleModel.ActivityInstance
}

func (r *absorbInstanceRepo) FindByActiveGroupID(_ context.Context, groupID int64) (*scheduleModel.ActivityInstance, error) {
	return r.byGroup[groupID], nil
}

func (r *absorbVisitRepo) TransferActiveVisitsBetweenGroups(_ context.Context, oldGroupID, newGroupID int64) (int, error) {
	r.transfers = append(r.transfers, [2]int64{oldGroupID, newGroupID})
	return r.transferCounts[oldGroupID], nil
}

// A started instance absorbs open sessions WITHOUT active supervisors in its
// room (their open visits move over, the orphan session ends) and leaves
// supervised parallel sessions alone (#2161, sanctioned pattern per #2139).
func TestInstanceStart_AbsorbsUnsupervisedOpenGroups(t *testing.T) {
	const newGroupID = int64(10)

	now := time.Now()
	newGroup := &activeModel.Group{StartTime: now}
	newGroup.ID = newGroupID
	unsupervised := &activeModel.Group{StartTime: now}
	unsupervised.ID = 11
	supervised := &activeModel.Group{StartTime: now}
	supervised.ID = 12
	bridged := &activeModel.Group{StartTime: now}
	bridged.ID = 13
	staleFallback := &activeModel.Group{StartTime: now.AddDate(0, 0, -1)}
	staleFallback.ID = 14

	groupRepo := &absorbGroupRepo{openGroups: []*activeModel.Group{newGroup, unsupervised, supervised, bridged, staleFallback}}
	supervisorRepo := &absorbSupervisorRepo{byGroup: map[int64][]*activeModel.GroupSupervisor{
		12: {{StaffID: 7, GroupID: 12}},
	}}
	visitRepo := &absorbVisitRepo{transferCounts: map[int64]int{11: 1}}
	instanceRepo := &absorbInstanceRepo{byGroup: map[int64]*scheduleModel.ActivityInstance{
		13: {
			Date:          timezone.TodayDate(),
			Status:        scheduleModel.InstanceStatusActive,
			ActiveGroupID: &bridged.ID,
		},
	}}

	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:    instanceRepo,
		ActiveGroupRepo: groupRepo,
		SupervisorRepo:  supervisorRepo,
		VisitRepo:       visitRepo,
		Logger:          slog.New(slog.DiscardHandler),
	}}

	err := svc.absorbUnsupervisedOpenGroups(context.Background(), 42, newGroupID)

	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12, 13}, groupRepo.lockedIDs, "only today's candidate sessions are locked")
	assert.Equal(t, []int64{11}, groupRepo.endedIDs, "only the unbridged unsupervised session is ended")
	assert.Equal(t, [][2]int64{{11, newGroupID}}, visitRepo.transfers, "open visits move through the conditional bulk update")
}
