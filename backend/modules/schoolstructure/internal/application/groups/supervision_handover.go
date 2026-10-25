package groups

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Handing over and releasing a running supervision (#4763). Both change only
// who supervises the session right now: the children stay in the room, the
// session keeps running and the planned staffing of its block is untouched.
// Only a person who supervises the session may hand it over or release it;
// an administrator who does not supervise it may add a supervisor, never
// remove one.

// codeLastSupervisor is the registered error code of a release that would
// leave the session without a supervisor.
const codeLastSupervisor = "substitutions.last_supervisor"

// The reasons name the hand-over and the release on the
// active_supervision_changed event, so the sidebars of both people reload
// their supervisions.
const (
	reasonSupervisionHandedOver = "supervision_handed_over"
	reasonSupervisionReleased   = "supervision_released"
)

// handOverSupervision adds the target to the caller's running supervision
// and ends the caller's own supervision in the same unit of work. The target
// is anyone „Betreuer hinzufügen“ could add.
func (s *substitutionModule) handOverSupervision(
	ctx context.Context,
	caller SubstitutionCaller,
	request *AdditionalSupervisionAssignment,
) (*AssignmentResult, error) {
	if s.deps.SupervisionRelease == nil {
		return nil, ErrInvalidTarget
	}
	actor, err := s.resolveSupervisingActor(ctx, caller)
	if err != nil {
		return nil, err
	}
	if actor.StaffID == request.TargetStaffID {
		return nil, ErrSelfAssignment
	}
	// Without admin access the lock requires the caller to supervise the
	// session; a caller who does not is refused as forbidden.
	created, target, err := s.assignAdditionalSupervisionLocked(ctx, caller, substitutionAccess{actor: actor}, true, request)
	if err != nil {
		return nil, err
	}
	if err := s.releaseOwnSupervision(ctx, caller, created.GroupID, actor.StaffID); err != nil {
		return nil, err
	}
	s.announceSupervisionChange(ctx, created.GroupID, "supervision_handover", reasonSupervisionHandedOver)
	return additionalSupervisionResult(created, target), nil
}

// releaseSupervision ends the caller's own supervision of the running session
// while another supervisor stays.
func (s *substitutionModule) releaseSupervision(ctx context.Context, caller SubstitutionCaller, activeGroupID int64) error {
	if activeGroupID <= 0 || s.deps.SupervisionRelease == nil {
		return ErrInvalidTarget
	}
	actor, err := s.resolveSupervisingActor(ctx, caller)
	if err != nil {
		return err
	}
	if err := s.releaseOwnSupervision(ctx, caller, activeGroupID, actor.StaffID); err != nil {
		return err
	}
	s.announceSupervisionChange(ctx, activeGroupID, "supervision_release", reasonSupervisionReleased)
	return nil
}

// resolveSupervisingActor resolves the caller's staff identity. The
// supervision itself is the authorization for a hand-over or a release, so no
// role is required beyond a staff member of the school.
func (s *substitutionModule) resolveSupervisingActor(ctx context.Context, caller SubstitutionCaller) (*Actor, error) {
	if caller.AccountID <= 0 || caller.TenantID <= 0 || (caller.Scope != "" && caller.Scope != "org") {
		return nil, ErrForbidden
	}
	actor, err := s.resolveActor(ctx, caller.AccountID)
	if err != nil {
		return nil, err
	}
	if actor == nil {
		return nil, ErrForbidden
	}
	return actor, nil
}

// releaseOwnSupervision ends the staff member's supervision through Student
// Presence, which refuses while nobody else supervises the session, and
// records the end in the responsibility trail.
func (s *substitutionModule) releaseOwnSupervision(ctx context.Context, caller SubstitutionCaller, activeGroupID, staffID int64) error {
	ended, err := s.deps.SupervisionRelease.ReleaseSupervision(ctx, activeGroupID, staffID)
	if err != nil {
		return releaseError(err)
	}
	change := domain.SubstitutionChange{
		SubstitutionID: ended.ID, TargetType: string(TargetAdditionalSupervision), Action: domain.SubstitutionEnded,
		GroupID: ended.GroupID, TargetStaffID: ended.StaffID, ActorAccountID: caller.AccountID,
		StartDate: calendar.Date(ended.StartDate),
	}
	if ended.EndDate != nil {
		end := calendar.Date(*ended.EndDate)
		change.EndDate = &end
	}
	return s.deps.Audit.RecordSubstitutionChange(ctx, change)
}

// releaseError maps Student Presence's refusals onto the substitution
// contract: an ended or missing session is not running, a caller without a
// supervision there may not release it, and the last supervisor stays.
func releaseError(err error) error {
	switch {
	case errors.Is(err, studentpresence.ErrLastSupervisor):
		return &OperationError{
			Target: ErrConflict, Code: codeLastSupervisor,
			Message: "Sie sind die einzige Aufsicht. Übergeben Sie die Aufsicht oder beenden Sie die Aktivität.", Cause: err,
		}
	case errors.Is(err, studentpresence.ErrGroupSupervisorNotFound):
		return ErrForbidden
	case errors.Is(err, studentpresence.ErrGroupNotFound):
		return ErrNotRunning
	default:
		return err
	}
}
