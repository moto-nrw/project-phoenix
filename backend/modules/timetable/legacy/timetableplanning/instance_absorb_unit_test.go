package timetableplanning

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type absorbGroupRepo struct {
	studentpresence.SessionRecords
	openGroups   []*studentpresence.LiveGroup
	lockedGroups map[int64]*studentpresence.LiveGroup
	lockedIDs    []int64
}

func (r *absorbGroupRepo) FindActiveByRoomID(_ context.Context, _ int64) ([]*studentpresence.LiveGroup, error) {
	return r.openGroups, nil
}

func (r *absorbGroupRepo) FindByIDForUpdate(_ context.Context, id int64) (*studentpresence.LiveGroup, error) {
	r.lockedIDs = append(r.lockedIDs, id)
	if group := r.lockedGroups[id]; group != nil {
		return group, nil
	}
	for _, group := range r.openGroups {
		if group.ID == id {
			return group, nil
		}
	}
	return nil, nil
}

func TestInstanceStart_DoesNotAbsorbGroupMovedAfterCandidateLookup(t *testing.T) {
	t.Parallel()

	now := time.Now()
	candidate := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	candidate.ID = 11
	locked := *candidate
	locked.RoomID = 99
	groupRepo := &absorbGroupRepo{
		openGroups:   []*studentpresence.LiveGroup{candidate},
		lockedGroups: map[int64]*studentpresence.LiveGroup{candidate.ID: &locked},
	}
	visitRepo := &absorbVisitRepo{}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:     &absorbInstanceRepo{},
		InstanceStudents: &absorbInstanceStudentRepo{},
		ActiveGroupRepo:  groupRepo,
		SupervisorRepo:   &absorbSupervisorRepo{},
		Presence:         visitRepo,
		Logger:           slog.New(slog.DiscardHandler),
	}}

	err := svc.absorbUnsupervisedOpenGroups(context.Background(), 9, 42, 10)

	require.NoError(t, err)
	assert.Equal(t, []int64{candidate.ID}, groupRepo.lockedIDs)
	assert.Empty(t, visitRepo.transfers)
	assert.Empty(t, visitRepo.endedIDs)
}

type absorbSupervisorRepo struct {
	studentpresence.SupervisionRecords
	byGroup map[int64][]*studentpresence.StaffedSupervision
}

func (r *absorbSupervisorRepo) FindByActiveGroupID(_ context.Context, groupID int64, _ bool) ([]*studentpresence.StaffedSupervision, error) {
	return r.byGroup[groupID], nil
}

type absorbVisitRepo struct {
	InstancePresence
	visits    []studentpresence.Visit
	transfers [][2]int64
	endedIDs  []int64
}

func (r *absorbVisitRepo) EndGroup(_ context.Context, id int64, _ time.Time) error {
	r.endedIDs = append(r.endedIDs, id)
	return nil
}

type absorbInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
	byGroup map[int64]*scheduleModel.ActivityInstance
}

func (r *absorbInstanceRepo) FindByActiveGroupID(_ context.Context, groupID int64) (*scheduleModel.ActivityInstance, error) {
	return r.byGroup[groupID], nil
}

type absorbActivityGroupRepo struct {
	activitiesModel.GroupRepository
	byID map[int64]*activitiesModel.Group
}

func (r *absorbActivityGroupRepo) FindByIDs(_ context.Context, ids []int64) ([]*activitiesModel.Group, error) {
	out := make([]*activitiesModel.Group, 0, len(ids))
	for _, id := range ids {
		if group := r.byID[id]; group != nil {
			out = append(out, group)
		}
	}
	return out, nil
}

func (r *absorbVisitRepo) TransferOpenVisits(_ context.Context, oldGroupID, newGroupID int64) (int64, error) {
	r.transfers = append(r.transfers, [2]int64{oldGroupID, newGroupID})
	moved := int64(0)
	for i := range r.visits {
		visit := &r.visits[i]
		if visit.ActiveGroupID == oldGroupID && visit.ExitTime == nil {
			visit.ActiveGroupID = newGroupID
			moved++
		}
	}
	return moved, nil
}

func (r *absorbVisitRepo) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	visits := make([]studentpresence.Visit, 0)
	for _, visit := range r.visits {
		if visit.ActiveGroupID == filter.ActiveGroupIDs[0] {
			visits = append(visits, visit)
		}
	}
	return visits, nil
}

type absorbInstanceStudentRepo struct {
	scheduleModel.InstanceStudentRepository
	unplannedStudentID int64
	updates            []absorbedAttendanceUpdate
	lookups            []absorbedAttendanceUpdate
	creates            []absorbedAttendanceUpdate
}

type absorbedAttendanceUpdate struct {
	instanceID int64
	studentID  int64
	checkedIn  time.Time
}

func (r *absorbInstanceStudentRepo) UpdateAttendanceFromCheckin(
	_ context.Context, instanceID, studentID int64, checkedIn time.Time,
) (bool, error) {
	r.updates = append(r.updates, absorbedAttendanceUpdate{
		instanceID: instanceID,
		studentID:  studentID,
		checkedIn:  checkedIn,
	})
	return studentID != r.unplannedStudentID, nil
}

func (r *absorbInstanceStudentRepo) FindByInstanceAndStudent(
	_ context.Context, instanceID, studentID int64,
) (*scheduleModel.InstanceStudent, error) {
	r.lookups = append(r.lookups, absorbedAttendanceUpdate{
		instanceID: instanceID,
		studentID:  studentID,
	})
	return nil, nil
}

func (r *absorbInstanceStudentRepo) CreateUnplannedPresentIfAbsent(
	_ context.Context, instanceID, studentID int64, checkedIn time.Time,
) (*scheduleModel.InstanceStudent, error) {
	r.creates = append(r.creates, absorbedAttendanceUpdate{
		instanceID: instanceID,
		studentID:  studentID,
		checkedIn:  checkedIn,
	})
	return &scheduleModel.InstanceStudent{}, nil
}

// A started instance absorbs open sessions WITHOUT active supervisors in its
// room (their open visits move over, the orphan session ends) and leaves
// supervised parallel sessions alone (#2161, sanctioned pattern per #2139).
func TestInstanceStart_AbsorbsUnsupervisedOpenGroups(t *testing.T) {
	t.Parallel()

	const (
		instanceID         int64 = 9
		newGroupID               = int64(10)
		studentID                = int64(21)
		unplannedStudentID       = int64(22)
	)

	now := time.Now()
	newGroup := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	newGroup.ID = newGroupID
	unsupervised := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	unsupervised.ID = 11
	supervised := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	supervised.ID = 12
	bridged := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	bridged.ID = 13
	staleFallback := &studentpresence.LiveGroup{StartTime: now.AddDate(0, 0, -1), RoomID: 42}
	staleFallback.ID = 14
	systemActivityID := int64(88)
	independent := &studentpresence.LiveGroup{StartTime: now, RoomID: 42, ActivityGroupID: &systemActivityID}
	independent.ID = 15

	groupRepo := &absorbGroupRepo{openGroups: []*studentpresence.LiveGroup{newGroup, unsupervised, supervised, bridged, staleFallback, independent}}
	supervisorRepo := &absorbSupervisorRepo{byGroup: map[int64][]*studentpresence.StaffedSupervision{
		12: {{GroupSupervision: studentpresence.GroupSupervision{StaffID: 7, GroupID: 12}}},
	}}
	entryTime := now.Add(-15 * time.Minute)
	visitRepo := &absorbVisitRepo{visits: []studentpresence.Visit{
		{
			StudentID:     studentID,
			ActiveGroupID: unsupervised.ID,
			EntryTime:     entryTime,
		},
		{
			StudentID:     unplannedStudentID,
			ActiveGroupID: unsupervised.ID,
			EntryTime:     entryTime,
		},
	}}
	instanceStudents := &absorbInstanceStudentRepo{unplannedStudentID: unplannedStudentID}
	instanceRepo := &absorbInstanceRepo{byGroup: map[int64]*scheduleModel.ActivityInstance{
		13: {
			Date:          scheduleModel.Date(timezone.TodayDate()),
			Status:        scheduleModel.InstanceStatusActive,
			ActiveGroupID: &bridged.ID,
		},
	}}
	systemActivity := &activitiesModel.Group{IsSystem: true}
	systemActivity.ID = systemActivityID

	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:      instanceRepo,
		InstanceStudents:  instanceStudents,
		ActiveGroupRepo:   groupRepo,
		ActivityGroupRepo: &absorbActivityGroupRepo{byID: map[int64]*activitiesModel.Group{systemActivityID: systemActivity}},
		SupervisorRepo:    supervisorRepo,
		Presence:          visitRepo,
		Logger:            slog.New(slog.DiscardHandler),
	}}

	err := svc.absorbUnsupervisedOpenGroups(context.Background(), instanceID, 42, newGroupID)

	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12, 13}, groupRepo.lockedIDs, "only today's candidate sessions are locked")
	assert.Equal(t, []int64{11}, visitRepo.endedIDs, "only the unbridged unsupervised session is ended")
	assert.Equal(t, [][2]int64{{11, newGroupID}}, visitRepo.transfers, "open visits move through the conditional bulk update")
	assert.Equal(t, []absorbedAttendanceUpdate{{
		instanceID: instanceID,
		studentID:  studentID,
		checkedIn:  entryTime,
	}, {
		instanceID: instanceID,
		studentID:  unplannedStudentID,
		checkedIn:  entryTime,
	}}, instanceStudents.updates, "absorbed visit is mirrored into the planned attendance row")
	assert.Equal(t, []absorbedAttendanceUpdate{{
		instanceID: instanceID,
		studentID:  unplannedStudentID,
	}}, instanceStudents.lookups, "missing planned attendance is confirmed before inserting")
	assert.Equal(t, []absorbedAttendanceUpdate{{
		instanceID: instanceID,
		studentID:  unplannedStudentID,
		checkedIn:  entryTime,
	}}, instanceStudents.creates, "unplanned absorbed student gets a present attendance row")
}

func (r *absorbInstanceStudentRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error) {
	if len(instanceIDs) > 0 && r.unplannedStudentID > 0 {
		r.lookups = append(r.lookups, absorbedAttendanceUpdate{
			instanceID: instanceIDs[0],
			studentID:  r.unplannedStudentID,
		})
	}
	return nil, nil
}
