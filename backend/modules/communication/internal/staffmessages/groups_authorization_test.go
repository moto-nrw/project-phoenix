package staffmessaging

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type membershipRaceRepository struct {
	StaffMessageThreadRepository
	StaffMessageReadRepository
	StaffMessageRepository
	thread            *StaffMessageThread
	locked            bool
	participantChecks int
	appendAttempted   bool
}

func (r *membershipRaceRepository) FindByID(context.Context, int64) (*StaffMessageThread, error) {
	return r.thread, nil
}

func (r *membershipRaceRepository) IsParticipant(context.Context, int64, int64) (bool, error) {
	r.participantChecks++
	return !r.locked, nil
}

func (r *membershipRaceRepository) LockForMessageAppend(context.Context, int64) error {
	r.locked = true
	return nil
}

func (r *membershipRaceRepository) IsMessageableStaff(context.Context, int64) (bool, error) {
	return true, nil
}

func (r *membershipRaceRepository) StaffRoleKinds(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{1: roleKindAdmin}, nil
}

func (r *membershipRaceRepository) Create(context.Context, *StaffMessage) error {
	r.appendAttempted = true
	return errors.New("append must not run after membership is revoked")
}

type enabledStaffMessagingSettings struct{}

func (enabledStaffMessagingSettings) StaffMessagingEnabled(context.Context) (bool, error) {
	return true, nil
}
func (enabledStaffMessagingSettings) StaffMessagingEnabledKey() string { return "test.enabled" }
func (enabledStaffMessagingSettings) StaffMessageRetentionDays(context.Context) (int, error) {
	return 30, nil
}
func (enabledStaffMessagingSettings) StaffMessageRetentionDaysKey() string { return "test.retention" }

type staticStaffMessagingSession struct{}

func (staticStaffMessagingSession) AccountID(context.Context) int64 { return 1 }

func TestGroupMutationAndSendReauthorizeAfterThreadLock(t *testing.T) {
	t.Parallel()

	newService := func() (*Service, *membershipRaceRepository) {
		repo := &membershipRaceRepository{thread: &StaffMessageThread{ID: 1, Kind: groupKind}}
		return &Service{Config: Config{
			ThreadRepo:  repo,
			MessageRepo: repo,
			ReadRepo:    repo,
			Settings:    enabledStaffMessagingSettings{},
			Session:     staticStaffMessagingSession{},
		}}, repo
	}

	t.Run("managed mutation", func(t *testing.T) {
		svc, repo := newService()
		_, _, err := svc.managedGroup(t.Context(), 1)

		assert.ErrorIs(t, err, ErrNotParticipant)
		assert.Equal(t, 2, repo.participantChecks)
	})

	t.Run("send", func(t *testing.T) {
		svc, repo := newService()
		_, err := svc.PostMessage(t.Context(), 1, "Nachricht")

		assert.ErrorIs(t, err, ErrNotParticipant)
		assert.Equal(t, 2, repo.participantChecks)
		assert.False(t, repo.appendAttempted)
	})
}

type finalMemberRepository struct {
	StaffMessageThreadRepository
	thread *StaffMessageThread
}

func (r *finalMemberRepository) FindByID(context.Context, int64) (*StaffMessageThread, error) {
	return r.thread, nil
}

func (*finalMemberRepository) IsParticipant(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (*finalMemberRepository) LockForMessageAppend(context.Context, int64) error {
	return nil
}

func (r *finalMemberRepository) ParticipantAccountIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}

type finalMemberGroups struct {
	deleteThread func(context.Context, int64) error
}

func (finalMemberGroups) CreateStaffGroupThread(context.Context, string, int64, []int64) (*communication.StaffThreadRecord, error) {
	return nil, nil
}

func (finalMemberGroups) RenameStaffGroupThread(context.Context, int64, string) error { return nil }
func (finalMemberGroups) AddStaffGroupParticipants(context.Context, int64, []int64) error {
	return nil
}
func (finalMemberGroups) RemoveStaffParticipant(context.Context, int64, int64) error { return nil }
func (finalMemberGroups) StaffParticipantStates(context.Context, int64) ([]*communication.StaffParticipantRecord, error) {
	return nil, nil
}
func (g finalMemberGroups) DeleteStaffThread(ctx context.Context, threadID int64) error {
	return g.deleteThread(ctx, threadID)
}

type afterCommitRuntime func(func())

func (r afterCommitRuntime) RegisterAfterCommit(_ context.Context, callback func()) {
	r(callback)
}

type finalMemberBroadcaster struct {
	staff  func(int64, []int64, realtime.Event) error
	school func(int64, []int64, realtime.Event) error
}

func (finalMemberBroadcaster) BroadcastToGroup(int64, string, realtime.Event) error { return nil }
func (finalMemberBroadcaster) BroadcastToGroups(int64, []string, realtime.Event) error {
	return nil
}
func (finalMemberBroadcaster) BroadcastToTenant(int64, realtime.Event) error { return nil }
func (finalMemberBroadcaster) BroadcastToTenantAdmins(int64, realtime.Event) error {
	return nil
}
func (b finalMemberBroadcaster) BroadcastToStaffAccounts(tenantID int64, accountIDs []int64, event realtime.Event) error {
	return b.staff(tenantID, accountIDs, event)
}
func (b finalMemberBroadcaster) BroadcastToSchoolAccounts(tenantID int64, accountIDs []int64, event realtime.Event) error {
	return b.school(tenantID, accountIDs, event)
}
func (finalMemberBroadcaster) BroadcastToAll(realtime.Event) error { return nil }
func (finalMemberBroadcaster) BroadcastParentMessage(int64, int64, realtime.Event) error {
	return nil
}
func (finalMemberBroadcaster) BroadcastToGuardian(int64, int64, realtime.Event) error {
	return nil
}

func TestLeaveFinalGroupMemberBroadcastsAfterCommit(t *testing.T) {
	t.Parallel()

	repository := &finalMemberRepository{thread: &StaffMessageThread{ID: 7, TenantID: 11, Kind: groupKind}}
	deleted := false
	groups := finalMemberGroups{deleteThread: func(context.Context, int64) error {
		deleted = true
		return nil
	}}
	var afterCommit func()
	runtime := afterCommitRuntime(func(callback func()) { afterCommit = callback })
	type broadcastCall struct {
		tenantID   int64
		accountIDs []int64
	}
	var staffCalls, schoolCalls []broadcastCall
	broadcaster := finalMemberBroadcaster{
		staff: func(tenantID int64, accountIDs []int64, _ realtime.Event) error {
			staffCalls = append(staffCalls, broadcastCall{tenantID: tenantID, accountIDs: accountIDs})
			return nil
		},
		school: func(tenantID int64, accountIDs []int64, _ realtime.Event) error {
			schoolCalls = append(schoolCalls, broadcastCall{tenantID: tenantID, accountIDs: accountIDs})
			return nil
		},
	}
	service := NewService(Config{
		ThreadRepo:  repository,
		Groups:      groups,
		Session:     staticStaffMessagingSession{},
		Broadcaster: broadcaster,
	}, runtime, nil)

	require.NoError(t, service.LeaveGroup(t.Context(), 7))
	assert.True(t, deleted)
	require.NotNil(t, afterCommit)
	assert.Empty(t, staffCalls, "the event waits for the transaction commit")
	assert.Empty(t, schoolCalls, "the event waits for the transaction commit")

	afterCommit()
	for _, calls := range [][]broadcastCall{staffCalls, schoolCalls} {
		require.Len(t, calls, 1)
		assert.Equal(t, int64(11), calls[0].tenantID)
		assert.Equal(t, []int64{1}, calls[0].accountIDs)
	}
}
