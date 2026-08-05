package schedule

import (
	"context"
	"log/slog"
	"testing"
	"time"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type absorbGroupRepo struct {
	activeModel.GroupRepository
	openGroups []*activeModel.Group
	endedIDs   []int64
}

func (r *absorbGroupRepo) FindActiveByRoomID(_ context.Context, _ int64) ([]*activeModel.Group, error) {
	return r.openGroups, nil
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
	byGroup map[int64][]*activeModel.Visit
	updated []*activeModel.Visit
}

func (r *absorbVisitRepo) FindByActiveGroupID(_ context.Context, groupID int64) ([]*activeModel.Visit, error) {
	return r.byGroup[groupID], nil
}

func (r *absorbVisitRepo) Update(_ context.Context, visit *activeModel.Visit) error {
	r.updated = append(r.updated, visit)
	return nil
}

// A started instance absorbs open sessions WITHOUT active supervisors in its
// room (their open visits move over, the orphan session ends) and leaves
// supervised parallel sessions alone (#2161, sanctioned pattern per #2139).
func TestInstanceStart_AbsorbsUnsupervisedOpenGroups(t *testing.T) {
	const newGroupID = int64(10)

	exitTime := time.Now()
	openVisit := &activeModel.Visit{ActiveGroupID: 11, StudentID: 1}
	openVisit.ID = 101
	exitedVisit := &activeModel.Visit{ActiveGroupID: 11, StudentID: 2, ExitTime: &exitTime}
	exitedVisit.ID = 102

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
	visitRepo := &absorbVisitRepo{byGroup: map[int64][]*activeModel.Visit{
		11: {openVisit, exitedVisit},
	}}

	svc := &instanceService{deps: InstanceServiceDependencies{
		ActiveGroupRepo: groupRepo,
		SupervisorRepo:  supervisorRepo,
		VisitRepo:       visitRepo,
		Logger:          slog.New(slog.DiscardHandler),
	}}

	err := svc.absorbUnsupervisedOpenGroups(context.Background(), 42, newGroupID)

	require.NoError(t, err)
	assert.Equal(t, []int64{11}, groupRepo.endedIDs, "only the unsupervised session is ended")
	require.Len(t, visitRepo.updated, 1, "only the open visit moves")
	assert.Equal(t, openVisit.ID, visitRepo.updated[0].ID)
	assert.Equal(t, newGroupID, visitRepo.updated[0].ActiveGroupID)
	assert.Equal(t, int64(11), exitedVisit.ActiveGroupID, "exited visits stay in the absorbed group")
}
