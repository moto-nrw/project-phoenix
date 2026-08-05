package schedule

import (
	"context"
	"log/slog"
	"testing"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
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

func (r *absorbVisitRepo) TransferActiveVisitsBetweenGroups(_ context.Context, oldGroupID, newGroupID int64) (int, error) {
	r.transfers = append(r.transfers, [2]int64{oldGroupID, newGroupID})
	return r.transferCounts[oldGroupID], nil
}

// A started instance absorbs open sessions WITHOUT active supervisors in its
// room (their open visits move over, the orphan session ends) and leaves
// supervised parallel sessions alone (#2161, sanctioned pattern per #2139).
func TestInstanceStart_AbsorbsUnsupervisedOpenGroups(t *testing.T) {
	const newGroupID = int64(10)

	newGroup := &activeModel.Group{}
	newGroup.ID = newGroupID
	unsupervised := &activeModel.Group{}
	unsupervised.ID = 11
	supervised := &activeModel.Group{}
	supervised.ID = 12

	groupRepo := &absorbGroupRepo{openGroups: []*activeModel.Group{newGroup, unsupervised, supervised}}
	supervisorRepo := &absorbSupervisorRepo{byGroup: map[int64][]*activeModel.GroupSupervisor{
		12: {{StaffID: 7, GroupID: 12}},
	}}
	visitRepo := &absorbVisitRepo{transferCounts: map[int64]int{11: 1}}

	svc := &instanceService{deps: InstanceServiceDependencies{
		ActiveGroupRepo: groupRepo,
		SupervisorRepo:  supervisorRepo,
		VisitRepo:       visitRepo,
		Logger:          slog.New(slog.DiscardHandler),
	}}

	err := svc.absorbUnsupervisedOpenGroups(context.Background(), 42, newGroupID)

	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12}, groupRepo.lockedIDs, "candidate sessions are locked before inspection")
	assert.Equal(t, []int64{11}, groupRepo.endedIDs, "only the unsupervised session is ended")
	assert.Equal(t, [][2]int64{{11, newGroupID}}, visitRepo.transfers, "open visits move through the conditional bulk update")
}
