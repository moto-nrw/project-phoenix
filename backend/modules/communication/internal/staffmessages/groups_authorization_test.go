package staffmessaging

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
