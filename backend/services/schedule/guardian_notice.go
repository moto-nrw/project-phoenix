package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Guardian notice on cancellation (#2601).
//
// Cancelling a block can inform the families of the booked children in the
// same request. The notice is a system-authored parent announcement; this file
// owns the schedule side: which children count, when a notice is refused, and
// that the cancellation and the notice commit together.

// ErrGuardianNoticeInvalid marks a notice request the service refuses before
// anything is written: empty text, no actor, or a block in the past.
var ErrGuardianNoticeInvalid = errors.New("guardian notice: invalid request")

// ErrGuardianNoticeDisabled surfaces the school-wide switch. Wrapped around
// announcement.ErrCareCancellationDisabled so handlers can match either.
var ErrGuardianNoticeDisabled = errors.New("guardian notice: disabled for this school")

// GuardianNoticeInput is the text the person cancelling wrote for the
// families. It is separate from the internal cancel reason on purpose: the
// reason may name a colleague's illness, the notice may not.
type GuardianNoticeInput struct {
	Title   string
	Message string
}

// GuardianNoticeResult reports what the notice reached.
type GuardianNoticeResult struct {
	AnnouncementID int64
	ChildCount     int
	FamilyCount    int
}

// GuardianNoticeReach is the preview the cancel dialog shows before sending.
type GuardianNoticeReach struct {
	Enabled     bool
	DefaultOn   bool
	ChildCount  int
	FamilyCount int
}

// CancelInstanceInput bundles a cancellation with its optional notice.
type CancelInstanceInput struct {
	InstanceID     int64
	Reason         *string
	ActorAccountID *int64
	// GuardianNotice, when set, publishes the notice in the same transaction.
	// nil cancels silently, exactly like before #2601.
	GuardianNotice *GuardianNoticeInput
}

// CancelInstanceResult is the cancelled block plus the notice outcome (nil
// when none was requested).
type CancelInstanceResult struct {
	Instance       *scheduleModel.ActivityInstance
	GuardianNotice *GuardianNoticeResult
}

// GuardianNoticePublisherSetter injects the announcement dependency after
// construction: the announcement service is wired later than the instance
// service in the factory, and widening the constructor would force every
// test that builds the service bare to supply one.
type GuardianNoticePublisherSetter interface {
	SetGuardianNoticePublisher(publisher announcement.CareCancellationPublisher)
}

// SetGuardianNoticePublisher implements GuardianNoticePublisherSetter.
func (s *instanceService) SetGuardianNoticePublisher(publisher announcement.CareCancellationPublisher) {
	s.deps.GuardianNotices = publisher
}

// GuardianNoticeReachFor resolves the dialog preview for one block: how many
// booked children it has and how many families a notice would reach. Runs
// inside the caller's tenant transaction.
func (s *instanceService) GuardianNoticeReachFor(ctx context.Context, instanceID int64) (*GuardianNoticeReach, error) {
	if s.deps.GuardianNotices == nil {
		return &GuardianNoticeReach{}, nil
	}
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	studentIDs, err := s.noticeStudentIDs(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	reach, err := s.deps.GuardianNotices.CareCancellationReachFor(ctx, studentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "guardian notice reach", Err: err}
	}
	return &GuardianNoticeReach{
		Enabled:     reach.Enabled,
		DefaultOn:   reach.DefaultOn,
		ChildCount:  len(studentIDs),
		FamilyCount: reach.FamilyCount,
	}, nil
}

// CancelWithNotice cancels a block and, when asked, informs the families in
// the same transaction. The notice is validated BEFORE the cancellation is
// written: a refused notice must not leave a silently cancelled block behind
// a 4xx response (the tenant tx commits on 4xx).
func (s *instanceService) CancelWithNotice(ctx context.Context, in CancelInstanceInput) (*CancelInstanceResult, error) {
	if in.GuardianNotice != nil {
		if err := s.validateGuardianNotice(ctx, in); err != nil {
			return nil, err
		}
	}
	instance, err := s.Cancel(ctx, in.InstanceID, in.Reason, in.ActorAccountID)
	if err != nil {
		return nil, err
	}
	result := &CancelInstanceResult{Instance: instance}
	if in.GuardianNotice == nil {
		return result, nil
	}
	notice, err := s.publishGuardianNotice(ctx, instance, *in.ActorAccountID, *in.GuardianNotice)
	if err != nil {
		// TenantTxMiddleware normally commits 4xx responses. A publication
		// refusal after Cancel must nevertheless undo the cancellation so the
		// caller never sees a rejected notice with a cancelled block.
		tenant.MarkRollback(ctx)
		return nil, err
	}
	result.GuardianNotice = notice
	return result, nil
}

func (s *instanceService) validateGuardianNotice(ctx context.Context, in CancelInstanceInput) error {
	if s.deps.GuardianNotices == nil {
		return fmt.Errorf("%w: notice publisher not wired", ErrGuardianNoticeDisabled)
	}
	if in.ActorAccountID == nil || *in.ActorAccountID <= 0 {
		return fmt.Errorf("%w: acting account is required", ErrGuardianNoticeInvalid)
	}
	if _, _, err := announcement.ValidateCareCancellationText(in.GuardianNotice.Title, in.GuardianNotice.Message); err != nil {
		return fmt.Errorf("%w: %w", ErrGuardianNoticeInvalid, err)
	}
	instance, err := s.loadForTransition(ctx, in.InstanceID)
	if err != nil {
		return err
	}
	// A block that already lies in the past is bookkeeping, not news.
	if instance.Date.Before(timezone.TodayDate()) {
		return fmt.Errorf("%w: block is in the past", ErrGuardianNoticeInvalid)
	}
	reach, err := s.deps.GuardianNotices.CareCancellationReachFor(ctx, nil)
	if err != nil {
		return &ScheduleError{Op: "guardian notice gate", Err: err}
	}
	if !reach.Enabled {
		return ErrGuardianNoticeDisabled
	}
	return nil
}

func (s *instanceService) publishGuardianNotice(ctx context.Context, instance *scheduleModel.ActivityInstance, actorAccountID int64, notice GuardianNoticeInput) (*GuardianNoticeResult, error) {
	studentIDs, err := s.noticeStudentIDs(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	result := &GuardianNoticeResult{ChildCount: len(studentIDs)}
	if len(studentIDs) == 0 {
		// Nobody was booked, so there is nobody to tell. Not an error: the
		// dialog already showed "0 Familien" and the person chose to proceed.
		s.getLogger().Info("guardian notice skipped: no booked children",
			slog.Int64("instance_id", instance.ID),
		)
		return result, nil
	}
	published, err := s.deps.GuardianNotices.PublishCareCancellation(ctx, announcement.CareCancellationInput{
		StudentIDs: studentIDs,
		Title:      notice.Title,
		Body:       notice.Message,
		CreatedBy:  actorAccountID,
	})
	if err != nil {
		if errors.Is(err, announcement.ErrCareCancellationDisabled) {
			return nil, fmt.Errorf("%w: %w", ErrGuardianNoticeDisabled, err)
		}
		if errors.Is(err, announcement.ErrParentAnnouncementValidation) {
			return nil, fmt.Errorf("%w: %w", ErrGuardianNoticeInvalid, err)
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
// attendance row that is not a frozen "was never booked that day" marker.
// A child already stamped absent (sick, excused) stays in: the family still
// learns the block fell out, which costs nothing and avoids a second rule.
func (s *instanceService) noticeStudentIDs(ctx context.Context, instanceID int64) ([]int64, error) {
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
