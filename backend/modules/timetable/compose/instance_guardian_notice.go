package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Guardian notice on cancellation (#2601): cancelling a block can inform the
// families of the booked children in the same request. The notice is a
// system-authored parent announcement Communication publishes; the
// lifecycle owns which children count, when a notice is refused, and that
// the cancellation and the notice commit together.

// GuardianNoticeAudience is how far a care cancellation notice reaches.
type GuardianNoticeAudience struct {
	Enabled     bool
	DefaultOn   bool
	FamilyCount int
}

// GuardianNoticePublication is one notice to publish.
type GuardianNoticePublication struct {
	StudentIDs []int64
	Title      string
	Body       string
	CreatedBy  int64
}

// GuardianNoticePublished is a published notice.
type GuardianNoticePublished struct {
	AnnouncementID int64
	RecipientCount int
}

// GuardianNotices is the consumer-owned port to Communication's care
// cancellation notice. A refused publication comes back wrapped in
// timetable.ErrGuardianNoticeDisabled or timetable.ErrGuardianNoticeInvalid.
type GuardianNotices interface {
	// ValidateNoticeText checks title and message before anything is written.
	ValidateNoticeText(title, message string) error
	// NoticeReach reports the school's switch and how many families the
	// children's notice would reach.
	NoticeReach(ctx context.Context, studentIDs []int64) (GuardianNoticeAudience, error)
	PublishNotice(ctx context.Context, notice GuardianNoticePublication) (GuardianNoticePublished, error)
}

// GuardianNoticeReachFor resolves the dialog preview for one block: how
// many booked children it has and how many families a notice would reach.
func (s *InstanceLifecycleService) GuardianNoticeReachFor(ctx context.Context, instanceID int64) (*timetable.GuardianNoticeReach, error) {
	if s.deps.GuardianNotices == nil {
		return &timetable.GuardianNoticeReach{}, nil
	}
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	studentIDs, err := s.noticeStudentIDs(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	reach, err := s.deps.GuardianNotices.NoticeReach(ctx, studentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "guardian notice reach", Err: err}
	}
	return &timetable.GuardianNoticeReach{
		Enabled:     reach.Enabled,
		DefaultOn:   reach.DefaultOn,
		ChildCount:  len(studentIDs),
		FamilyCount: reach.FamilyCount,
	}, nil
}

// CancelWithNotice cancels a block and, when asked, informs the families in
// the same transaction. The notice is validated BEFORE the cancellation:
// the tenant tx commits on 4xx, so a refused notice must not leave a
// silently cancelled block behind.
func (s *InstanceLifecycleService) CancelWithNotice(ctx context.Context, in timetable.CancelInstanceInput) (*timetable.CancelInstanceResult, error) {
	if in.GuardianNotice != nil {
		if err := s.validateGuardianNotice(ctx, in); err != nil {
			return nil, err
		}
	}
	instance, err := s.cancel(ctx, in.InstanceID, in.Reason, in.ActorAccountID)
	if err != nil {
		return nil, err
	}
	result := &timetable.CancelInstanceResult{Instance: LifecycleInstanceOf(instance)}
	if in.GuardianNotice == nil {
		return result, nil
	}
	notice, err := s.publishGuardianNotice(ctx, instance, *in.ActorAccountID, *in.GuardianNotice)
	if err != nil {
		// A refusal after the cancel must undo it, although the middleware
		// commits 4xx responses.
		tenant.MarkRollback(ctx)
		return nil, err
	}
	result.GuardianNotice = notice
	return result, nil
}

func (s *InstanceLifecycleService) validateGuardianNotice(ctx context.Context, in timetable.CancelInstanceInput) error {
	if s.deps.GuardianNotices == nil {
		return fmt.Errorf("%w: notice publisher not wired", timetable.ErrGuardianNoticeDisabled)
	}
	if in.ActorAccountID == nil || *in.ActorAccountID <= 0 {
		return fmt.Errorf("%w: acting account is required", timetable.ErrGuardianNoticeInvalid)
	}
	if err := s.deps.GuardianNotices.ValidateNoticeText(in.GuardianNotice.Title, in.GuardianNotice.Message); err != nil {
		return fmt.Errorf("%w: %w", timetable.ErrGuardianNoticeInvalid, err)
	}
	instance, err := s.loadForTransition(ctx, in.InstanceID)
	if err != nil {
		return err
	}
	// A block that already lies in the past is bookkeeping, not news.
	if instance.Date.Before(timezone.TodayDate()) {
		return fmt.Errorf("%w: block is in the past", timetable.ErrGuardianNoticeInvalid)
	}
	reach, err := s.deps.GuardianNotices.NoticeReach(ctx, nil)
	if err != nil {
		return &ScheduleError{Op: "guardian notice gate", Err: err}
	}
	if !reach.Enabled {
		return timetable.ErrGuardianNoticeDisabled
	}
	return nil
}

func (s *InstanceLifecycleService) publishGuardianNotice(
	ctx context.Context, instance *scheduleModel.ActivityInstance, actorAccountID int64, notice timetable.GuardianNoticeInput,
) (*timetable.GuardianNoticeResult, error) {
	studentIDs, err := s.noticeStudentIDs(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	result := &timetable.GuardianNoticeResult{ChildCount: len(studentIDs)}
	if len(studentIDs) == 0 {
		// Nobody was booked, so there is nobody to tell; the dialog already
		// showed "0 Familien" and the person chose to proceed.
		s.getLogger().Info("guardian notice skipped: no booked children",
			slog.Int64("instance_id", instance.ID),
		)
		return result, nil
	}
	published, err := s.deps.GuardianNotices.PublishNotice(ctx, GuardianNoticePublication{
		StudentIDs: studentIDs,
		Title:      notice.Title,
		Body:       notice.Message,
		CreatedBy:  actorAccountID,
	})
	if err != nil {
		if errors.Is(err, timetable.ErrGuardianNoticeDisabled) || errors.Is(err, timetable.ErrGuardianNoticeInvalid) {
			return nil, err
		}
		return nil, &ScheduleError{Op: "publish guardian notice", Err: err}
	}
	result.AnnouncementID = published.AnnouncementID
	result.FamilyCount = published.RecipientCount
	s.getLogger().Info("guardian notice published for cancelled block",
		slog.Int64("instance_id", instance.ID),
		slog.Int64("announcement_id", published.AnnouncementID),
		slog.Int("child_count", len(studentIDs)),
		slog.Int("family_count", published.RecipientCount),
	)
	return result, nil
}

// noticeStudentIDs returns the children the block was going to serve: every
// attendance row except the frozen "never booked that day" marker. A child
// already stamped absent stays in: the family still learns the block fell
// out.
func (s *InstanceLifecycleService) noticeStudentIDs(ctx context.Context, instanceID int64) ([]int64, error) {
	rows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, &ScheduleError{Op: "guardian notice: load roster", Err: err}
	}
	seen := make(map[int64]struct{}, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.NotScheduled || row.StudentID <= 0 {
			continue
		}
		if _, dup := seen[row.StudentID]; dup {
			continue
		}
		seen[row.StudentID] = struct{}{}
		ids = append(ids, row.StudentID)
	}
	return ids, nil
}
