package schedule_test

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/announcement"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingNoticePublisher stands in for the announcement service: it records
// what the cancel path hands over and answers the gate from a field.
type recordingNoticePublisher struct {
	enabled   bool
	published []announcement.CareCancellationInput
}

func (p *recordingNoticePublisher) PublishCareCancellation(_ context.Context, in announcement.CareCancellationInput) (*announcement.CareCancellationResult, error) {
	if !p.enabled {
		return nil, announcement.ErrCareCancellationDisabled
	}
	p.published = append(p.published, in)
	stored := &usersModels.ParentAnnouncement{}
	stored.ID = int64(len(p.published))
	return &announcement.CareCancellationResult{Announcement: stored, RecipientCount: len(in.StudentIDs)}, nil
}

func (p *recordingNoticePublisher) CareCancellationReachFor(_ context.Context, studentIDs []int64) (*announcement.CareCancellationReach, error) {
	return &announcement.CareCancellationReach{Enabled: p.enabled, DefaultOn: true, FamilyCount: len(studentIDs)}, nil
}

// seedNoticeInstance creates a planned block on the given date with two booked
// children and one frozen non-booking marker.
func seedNoticeInstance(t *testing.T, s *lifecycleSetup, date timezone.Date) (*scheduleModels.ActivityInstance, int64, int64) {
	t.Helper()
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Fußball-AG"})
	testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, s.student1, scheduleModels.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, s.student2, scheduleModels.AttendanceStatusExpected)
	unbooked := testpkg.CreateTestStudent(t, s.db, "Nie", "Gebucht", "2b")
	testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, unbooked.ID, scheduleModels.AttendanceStatusExpected,
		testpkg.InstanceStudentOpts{NotScheduled: true})
	return instance, s.student1, s.student2
}

func noticeInput(instanceID int64, actor int64, notice *scheduleSvc.GuardianNoticeInput) scheduleSvc.CancelInstanceInput {
	return scheduleSvc.CancelInstanceInput{
		InstanceID:     instanceID,
		ActorAccountID: &actor,
		GuardianNotice: notice,
	}
}

func TestCancelWithNotice_PublishesForBookedChildrenOnly(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	publisher := &recordingNoticePublisher{enabled: true}
	svc := instanceServiceWithBroadcaster(s, nil)
	svc.(scheduleSvc.GuardianNoticePublisherSetter).SetGuardianNoticePublisher(publisher)
	instance, child1, child2 := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
	actor := testpkg.CreateTestAccount(t, s.db, "cancel-actor")

	result, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &scheduleSvc.GuardianNoticeInput{
		Title:   "Fußball-AG entfällt",
		Message: "Die Fußball-AG morgen fällt aus.",
	}))
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCancelled, result.Instance.Status)
	require.NotNil(t, result.GuardianNotice)
	assert.Equal(t, 2, result.GuardianNotice.ChildCount)
	assert.Equal(t, 2, result.GuardianNotice.FamilyCount)

	require.Len(t, publisher.published, 1)
	sent := publisher.published[0]
	assert.ElementsMatch(t, []int64{child1, child2}, sent.StudentIDs, "the frozen non-booking marker is not a booked child")
	assert.Equal(t, actor.ID, sent.CreatedBy)
	assert.Equal(t, "Fußball-AG entfällt", sent.Title)
}

func TestCancelWithNotice_NilNoticeCancelsSilently(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	publisher := &recordingNoticePublisher{enabled: true}
	svc := instanceServiceWithBroadcaster(s, nil)
	svc.(scheduleSvc.GuardianNoticePublisherSetter).SetGuardianNoticePublisher(publisher)
	instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
	actor := testpkg.CreateTestAccount(t, s.db, "cancel-actor")

	result, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, nil))
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCancelled, result.Instance.Status)
	assert.Nil(t, result.GuardianNotice)
	assert.Empty(t, publisher.published)
}

func TestCancelWithNotice_RefusesBeforeCancellingWhenInvalid(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	publisher := &recordingNoticePublisher{enabled: true}
	svc := instanceServiceWithBroadcaster(s, nil)
	svc.(scheduleSvc.GuardianNoticePublisherSetter).SetGuardianNoticePublisher(publisher)
	actor := testpkg.CreateTestAccount(t, s.db, "cancel-actor")

	t.Run("empty text", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &scheduleSvc.GuardianNoticeInput{Title: "x"}))
		require.ErrorIs(t, err, scheduleSvc.ErrGuardianNoticeInvalid)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status, "a refused notice must not cancel the block")
	})

	t.Run("whitespace-only text", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &scheduleSvc.GuardianNoticeInput{Title: "  ", Message: "\t"}))
		require.ErrorIs(t, err, scheduleSvc.ErrGuardianNoticeInvalid)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status)
	})

	t.Run("overlong text", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &scheduleSvc.GuardianNoticeInput{Title: strings.Repeat("x", 201), Message: "Text"}))
		require.ErrorIs(t, err, scheduleSvc.ErrGuardianNoticeInvalid)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status)
	})

	t.Run("past block", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(-1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &scheduleSvc.GuardianNoticeInput{Title: "x", Message: "y"}))
		require.ErrorIs(t, err, scheduleSvc.ErrGuardianNoticeInvalid)
	})

	t.Run("no actor", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, scheduleSvc.CancelInstanceInput{
			InstanceID:     instance.ID,
			GuardianNotice: &scheduleSvc.GuardianNoticeInput{Title: "x", Message: "y"},
		})
		require.ErrorIs(t, err, scheduleSvc.ErrGuardianNoticeInvalid)
	})

	t.Run("school switched it off", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))
		publisher.enabled = false
		defer func() { publisher.enabled = true }()
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &scheduleSvc.GuardianNoticeInput{Title: "x", Message: "y"}))
		require.ErrorIs(t, err, scheduleSvc.ErrGuardianNoticeDisabled)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status)
	})
	assert.Empty(t, publisher.published)
}

func TestGuardianNoticeReachFor_CountsBookedChildren(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	publisher := &recordingNoticePublisher{enabled: true}
	svc := instanceServiceWithBroadcaster(s, nil)
	svc.(scheduleSvc.GuardianNoticePublisherSetter).SetGuardianNoticePublisher(publisher)
	instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))

	reach, err := svc.GuardianNoticeReachFor(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.True(t, reach.Enabled)
	assert.True(t, reach.DefaultOn)
	assert.Equal(t, 2, reach.ChildCount)
	assert.Equal(t, 2, reach.FamilyCount)
}

func TestGuardianNoticeReachFor_WithoutPublisherReportsDisabled(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)
	instance, _, _ := seedNoticeInstance(t, s, timezone.TodayDate().AddDays(1))

	reach, err := svc.GuardianNoticeReachFor(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.False(t, reach.Enabled)
}
