package presence

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
)

const opEnsureActivityParticipantLimit = "EnsureActivityParticipantLimit"

// ensureActivityParticipantLimit refuses an admission that would put more
// children into the session than its activity's participant limit allows
// (#3632): open visits of the session + incoming <= max_participants, the
// terminal's rule. Device requests always enforce it. Web writes enforce it
// when the school turned off exceeding the limit on the web.
//
// Callers hold the target session's row lock, the lock every visit admission
// into that session takes first. Two concurrent assignments into one session
// therefore count one after the other and cannot overshoot the limit
// together.
func (s *service) ensureActivityParticipantLimit(ctx context.Context, targetGroup *ports.ActiveGroup, incoming int, lockedOccupancy *int) error {
	if incoming <= 0 || targetGroup == nil || targetGroup.GroupID == nil {
		return nil
	}
	activity, err := s.limitedActivity(ctx, *targetGroup.GroupID)
	if err != nil || activity == nil {
		return err
	}
	var currentOccupancy int
	if lockedOccupancy != nil {
		currentOccupancy = *lockedOccupancy
	} else {
		currentOccupancy, err = s.SchoolPresence.CountOpenVisitsInGroup(ctx, targetGroup.ID)
		if err != nil {
			return &ActiveError{Op: opEnsureActivityParticipantLimit, Err: errors.Join(ErrDatabaseOperation, err)}
		}
	}

	return participantLimitRefusal(activity, currentOccupancy, incoming)
}

// ensureSessionFitsNewActivity refuses switching a running session to another
// activity when its open visits exceed the new activity's limit: every child
// in the session enters that activity at once. The caller holds the session's
// row lock.
func (s *service) ensureSessionFitsNewActivity(ctx context.Context, existing, revised *ports.ActiveGroup) error {
	if revised.GroupID == nil || (existing.GroupID != nil && *existing.GroupID == *revised.GroupID) {
		return nil
	}
	activity, err := s.limitedActivity(ctx, *revised.GroupID)
	if err != nil || activity == nil {
		return err
	}
	incoming, err := s.SchoolPresence.CountOpenVisitsInGroup(ctx, existing.ID)
	if err != nil {
		return &ActiveError{Op: opEnsureActivityParticipantLimit, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	return participantLimitRefusal(activity, 0, incoming)
}

// limitedActivity loads the activity whose limit this admission must respect.
// Device requests never consult the setting that allows web overflow.
func (s *service) limitedActivity(ctx context.Context, activityID int64) (*ports.SessionActivity, error) {
	if !s.isDeviceRequest(ctx) {
		enforced, err := s.webParticipantLimitEnforced(ctx)
		if err != nil {
			return nil, &ActiveError{Op: opEnsureActivityParticipantLimit, Err: errors.Join(ErrDatabaseOperation, err)}
		}
		if !enforced {
			return nil, nil
		}
	}
	if s.ActivityGroupRepo == nil {
		return nil, &ActiveError{Op: opEnsureActivityParticipantLimit, Err: errors.Join(ErrDatabaseOperation, errors.New("activity reader is not configured"))}
	}
	activity, err := s.ActivityGroupRepo.FindByID(ctx, activityID)
	if err != nil {
		return nil, &ActiveError{Op: opEnsureActivityParticipantLimit, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if activity == nil {
		return nil, &ActiveError{Op: opEnsureActivityParticipantLimit, Err: errors.Join(ErrDatabaseOperation, errors.New("activity not found"))}
	}
	if activity.MaxParticipants <= 0 {
		return nil, nil
	}
	return activity, nil

}

func participantLimitRefusal(activity *ports.SessionActivity, currentOccupancy, incoming int) error {
	if incoming <= 0 || currentOccupancy+incoming <= activity.MaxParticipants {
		return nil
	}
	return &ActivityParticipantLimitError{
		ActivityID:       activity.ID,
		ActivityName:     activity.Name,
		CurrentOccupancy: currentOccupancy,
		MaxParticipants:  activity.MaxParticipants,
		Incoming:         incoming,
	}
}

// webParticipantLimitEnforced follows the SettingsResolver contract: a service
// composed without a resolver keeps the registry default, which lets web
// assignments exceed the limit. Every production root wires the resolver.
func (s *service) webParticipantLimitEnforced(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.WebParticipantLimitEnforced(ctx)
}

// isDeviceRequest reports a kiosk request: an authenticated device, with or
// without the device PIN.
func (s *service) isDeviceRequest(ctx context.Context) bool {
	principal := s.attendancePrincipal(ctx)
	return principal.IsIoT || principal.DeviceID > 0
}
