package httpintegration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingNoticePublisher stands in for the announcement service: it records
// what the cancel path hands over and answers the gate from a field.
type recordingNoticePublisher struct {
	enabled   bool
	published []communication.CareCancellationInput
}

func (p *recordingNoticePublisher) PublishCareCancellation(_ context.Context, in communication.CareCancellationInput) (*communication.CareCancellationResult, error) {
	if !p.enabled {
		return nil, communication.ErrCareCancellationDisabled
	}
	p.published = append(p.published, in)
	return &communication.CareCancellationResult{AnnouncementID: int64(len(p.published)), RecipientCount: len(in.StudentIDs)}, nil
}

func (p *recordingNoticePublisher) CareCancellationReachFor(_ context.Context, studentIDs []int64) (*communication.CareCancellationReach, error) {
	return &communication.CareCancellationReach{Enabled: p.enabled, DefaultOn: true, FamilyCount: len(studentIDs)}, nil
}

// lifecycleGuardianNotices binds Communication's care cancellation notice to
// the lifecycle's port the way the composition root does: Communication
// validates the text, and a refused publication comes back wrapped in the
// owner's notice errors.
type lifecycleGuardianNotices struct {
	publisher communication.CareCancellationPublisher
}

func (n lifecycleGuardianNotices) ValidateNoticeText(title, message string) error {
	_, _, err := communication.ValidateCareCancellationText(title, message)
	return err
}

func (n lifecycleGuardianNotices) NoticeReach(ctx context.Context, studentIDs []int64) (compose.GuardianNoticeAudience, error) {
	reach, err := n.publisher.CareCancellationReachFor(ctx, studentIDs)
	if err != nil {
		return compose.GuardianNoticeAudience{}, err
	}
	return compose.GuardianNoticeAudience{Enabled: reach.Enabled, DefaultOn: reach.DefaultOn, FamilyCount: reach.FamilyCount}, nil
}

func (n lifecycleGuardianNotices) PublishNotice(ctx context.Context, notice compose.GuardianNoticePublication) (compose.GuardianNoticePublished, error) {
	published, err := n.publisher.PublishCareCancellation(ctx, communication.CareCancellationInput{
		StudentIDs: notice.StudentIDs, Title: notice.Title, Body: notice.Body, CreatedBy: notice.CreatedBy,
	})
	switch {
	case errors.Is(err, communication.ErrCareCancellationDisabled):
		return compose.GuardianNoticePublished{}, fmt.Errorf("%w: %w", timetable.ErrGuardianNoticeDisabled, err)
	case errors.Is(err, communication.ErrParentAnnouncementValidation):
		return compose.GuardianNoticePublished{}, fmt.Errorf("%w: %w", timetable.ErrGuardianNoticeInvalid, err)
	case err != nil:
		return compose.GuardianNoticePublished{}, err
	}
	return compose.GuardianNoticePublished{AnnouncementID: published.AnnouncementID, RecipientCount: published.RecipientCount}, nil
}

// instanceServiceWithGuardianNotices builds the lifecycle service with the
// given notice publisher, the way the composition root wires it.
func instanceServiceWithGuardianNotices(t *testing.T, s *lifecycleSetup, publisher communication.CareCancellationPublisher) *compose.InstanceLifecycleService {
	t.Helper()
	deps := lifecycleDependencies(s, nil)
	deps.GuardianNotices = lifecycleGuardianNotices{publisher: publisher}
	return newLifecycle(t, deps)
}

// seedNoticeInstance creates a planned block on the given date with two booked
// children and one frozen non-booking marker.
func seedNoticeInstance(t *testing.T, s *lifecycleSetup, date calendar.Date) (*scheduleModels.ActivityInstance, int64, int64) {
	t.Helper()
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Fußball-AG"})
	testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, s.student1, scheduleModels.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, s.student2, scheduleModels.AttendanceStatusExpected)
	unbooked := testpkg.CreateTestStudent(t, s.db, "Nie", "Gebucht", "2b")
	testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, unbooked.ID, scheduleModels.AttendanceStatusExpected,
		testpkg.InstanceStudentOpts{NotScheduled: true})
	return instance, s.student1, s.student2
}

func noticeInput(instanceID int64, actor int64, notice *timetable.GuardianNoticeInput) timetable.CancelInstanceInput {
	return timetable.CancelInstanceInput{
		InstanceID:     instanceID,
		ActorAccountID: &actor,
		GuardianNotice: notice,
	}
}

func TestCancelWithNotice_PublishesForBookedChildrenOnly(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	publisher := &recordingNoticePublisher{enabled: true}
	svc := instanceServiceWithGuardianNotices(t, s, publisher)
	instance, child1, child2 := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
	actor := testpkg.CreateTestAccount(t, s.db, "cancel-actor")

	result, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &timetable.GuardianNoticeInput{
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
	svc := instanceServiceWithGuardianNotices(t, s, publisher)
	instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
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
	svc := instanceServiceWithGuardianNotices(t, s, publisher)
	actor := testpkg.CreateTestAccount(t, s.db, "cancel-actor")

	t.Run("empty text", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &timetable.GuardianNoticeInput{Title: "x"}))
		require.ErrorIs(t, err, timetable.ErrGuardianNoticeInvalid)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status, "a refused notice must not cancel the block")
	})

	t.Run("whitespace-only text", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &timetable.GuardianNoticeInput{Title: "  ", Message: "\t"}))
		require.ErrorIs(t, err, timetable.ErrGuardianNoticeInvalid)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status)
	})

	t.Run("overlong text", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &timetable.GuardianNoticeInput{Title: strings.Repeat("x", 201), Message: "Text"}))
		require.ErrorIs(t, err, timetable.ErrGuardianNoticeInvalid)
		reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, reloaded.Status)
	})

	t.Run("past block", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(-1))
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &timetable.GuardianNoticeInput{Title: "x", Message: "y"}))
		require.ErrorIs(t, err, timetable.ErrGuardianNoticeInvalid)
	})

	t.Run("no actor", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
		_, err := svc.CancelWithNotice(s.ctx, timetable.CancelInstanceInput{
			InstanceID:     instance.ID,
			GuardianNotice: &timetable.GuardianNoticeInput{Title: "x", Message: "y"},
		})
		require.ErrorIs(t, err, timetable.ErrGuardianNoticeInvalid)
	})

	t.Run("school switched it off", func(t *testing.T) {
		instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))
		publisher.enabled = false
		defer func() { publisher.enabled = true }()
		_, err := svc.CancelWithNotice(s.ctx, noticeInput(instance.ID, actor.ID, &timetable.GuardianNoticeInput{Title: "x", Message: "y"}))
		require.ErrorIs(t, err, timetable.ErrGuardianNoticeDisabled)
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
	svc := instanceServiceWithGuardianNotices(t, s, publisher)
	instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))

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
	svc := instanceServiceWithBroadcaster(t, s, nil)
	instance, _, _ := seedNoticeInstance(t, s, calendar.TodayDate().AddDays(1))

	reach, err := svc.GuardianNoticeReachFor(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.False(t, reach.Enabled)
}
