package staffmessaging

import (
	"context"
	"errors"
	"testing"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/realtime/realtimetest"
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
	thread       *StaffMessageThread
	participants []int64
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
	return r.participants, nil
}

type finalMemberGroups struct {
	values.StaffGroupSource
	repository *finalMemberRepository
	deleted    bool
}

func (g *finalMemberGroups) RemoveStaffParticipant(context.Context, int64, int64) error {
	g.repository.participants = nil
	return nil
}

func (g *finalMemberGroups) DeleteStaffThread(context.Context, int64) error {
	g.deleted = true
	return nil
}

type recordingAfterCommitRuntime struct {
	callbacks []func()
}

func (r *recordingAfterCommitRuntime) RegisterAfterCommit(_ context.Context, callback func()) {
	r.callbacks = append(r.callbacks, callback)
}

func TestLeaveFinalGroupMemberBroadcastsAfterCommit(t *testing.T) {
	t.Parallel()

	repository := &finalMemberRepository{
		thread:       &StaffMessageThread{ID: 7, TenantID: 11, Kind: groupKind},
		participants: []int64{1},
	}
	groups := &finalMemberGroups{repository: repository}
	runtime := &recordingAfterCommitRuntime{}
	broadcaster := realtimetest.NewRecordingBroadcaster()
	service := NewService(Config{
		ThreadRepo:  repository,
		Groups:      groups,
		Session:     staticStaffMessagingSession{},
		Broadcaster: broadcaster,
	}, runtime, nil)

	require.NoError(t, service.LeaveGroup(t.Context(), 7))
	assert.True(t, groups.deleted)
	require.Len(t, runtime.callbacks, 1)
	assert.Empty(t, broadcaster.Calls(), "the event waits for the transaction commit")

	runtime.callbacks[0]()
	for _, method := range []string{"staff", "school"} {
		calls := broadcaster.CallsByMethod(method)
		require.Len(t, calls, 1)
		assert.Equal(t, int64(11), calls[0].TenantID)
		assert.Equal(t, []int64{1}, calls[0].AccountIDs)
	}
}
