package presence

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Group Supervisor operations
func (s *service) CreateGroupSupervisor(ctx context.Context, supervisor *ports.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrInvalidData}
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.createGroupSupervisor(txCtx, supervisor)
	})
}

func (s *service) createGroupSupervisor(ctx context.Context, supervisor *ports.GroupSupervisor) error {
	if err := s.lockStaffForSupervision(ctx, supervisor.StaffID); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: err}
	}

	// Lock the group and reject ended sessions before INSERT. The same
	// active.groups row lock serializes this write with session absorption
	// (absorbUnsupervisedOpenGroups) and ClaimActiveGroup, so a supervisor
	// cannot attach to a group that a concurrent absorption is ending.
	if err := s.validateActiveGroupOpenForUpdate(ctx, supervisor.GroupID); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: err}
	}

	// Check if staff is already supervising this group (only check active supervisors)
	supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, supervisor.GroupID, true)
	if err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	for _, s := range supervisors {
		if s.StaffID == supervisor.StaffID {
			return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrStaffAlreadySupervising}
		}
	}

	supervisor.SetTenantID(tenant.FromContext(ctx))
	if s.SupervisorRepo.Create(ctx, supervisor) != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}
	s.queueSupervisionRefresh(ctx, supervisor.GroupID, activeSupervisionReasonSupervisorsChanged)

	// Taking over a supervision that starts today means the staff member is
	// working right now — auto-open their work session so they show as
	// "Anwesend" (issue #1439). Kiosk-driven session starts already do this
	// in assignMultipleSupervisorsNonCritical; this covers the web app path.
	if supervisor.StartDate == s.todayDate() && !supervisor.SkipPresenceStamp {
		source := stampSourceApp
		if s.attendancePrincipal(ctx).IsIoT {
			source = stampSourceNFC
		}
		s.ensureStaffPresence(ctx, supervisor.StaffID, source)
	}

	return nil
}

func (s *service) UpdateGroupSupervisor(ctx context.Context, supervisor *ports.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrInvalidData}
	}
	original, err := s.supervisionForWrite(ctx, supervisor.ID, "UpdateGroupSupervisor")
	if err != nil {
		return err
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.updateGroupSupervisorLocked(txCtx, original, supervisor)
	})
}

// updateGroupSupervisorLocked locks the staff member of a running supervision
// and both sessions, then updates the supervision if it still belongs to the
// session it was read from.
func (s *service) updateGroupSupervisorLocked(ctx context.Context, original, supervisor *ports.GroupSupervisor) error {
	if supervisor.EndDate == nil || supervisor.EndDate.After(s.todayDate()) {
		if err := s.lockStaffForSupervision(ctx, supervisor.StaffID); err != nil {
			return &ActiveError{Op: "UpdateGroupSupervisor", Err: err}
		}
	}
	if err := s.lockGroupRows(ctx, original.GroupID, supervisor.GroupID); err != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrDatabaseOperation}
	}
	current, err := s.SupervisorRepo.FindByID(ctx, supervisor.ID)
	if err != nil || current == nil || current.GroupID != original.GroupID {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}
	if err := s.SupervisorRepo.Update(ctx, supervisor); err != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrDatabaseOperation}
	}
	s.queueSupervisionRefresh(ctx, original.GroupID, activeSupervisionReasonSupervisorsChanged)
	if supervisor.GroupID != original.GroupID {
		s.queueSupervisionRefresh(ctx, supervisor.GroupID, activeSupervisionReasonSupervisorsChanged)
	}
	return nil
}

// supervisionForWrite reads the supervision a write changes; a failed read
// counts as not found.
func (s *service) supervisionForWrite(ctx context.Context, id int64, op string) (*ports.GroupSupervisor, error) {
	supervision, err := s.SupervisorRepo.FindByID(ctx, id)
	if err != nil || supervision == nil {
		return nil, &ActiveError{Op: op, Err: ErrGroupSupervisorNotFound}
	}
	return supervision, nil
}

func (s *service) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	supervision, err := s.supervisionForWrite(ctx, id, "DeleteGroupSupervisor")
	if err != nil {
		return err
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.lockGroupRows(txCtx, supervision.GroupID); err != nil {
			return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrDatabaseOperation}
		}
		current, err := s.SupervisorRepo.FindByID(txCtx, id)
		if err != nil || current == nil || current.GroupID != supervision.GroupID {
			return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrGroupSupervisorNotFound}
		}
		if err := s.SupervisorRepo.Delete(txCtx, id); err != nil {
			return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrDatabaseOperation}
		}
		s.queueSupervisionRefresh(txCtx, supervision.GroupID, activeSupervisionReasonSupervisorsChanged)
		return nil
	})
}

func (s *service) EndSupervision(ctx context.Context, id int64) error {
	supervision, err := s.SupervisorRepo.FindByID(ctx, id)
	if err != nil {
		if IsNoRows(err) {
			return &ActiveError{Op: "EndSupervision", Err: ErrGroupSupervisorNotFound}
		}
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("failed to verify supervision: %w", err)}
	}
	if supervision == nil {
		return &ActiveError{Op: "EndSupervision", Err: ErrGroupSupervisorNotFound}
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		group, err := s.GroupRepo.FindByIDForUpdate(txCtx, supervision.GroupID)
		if err != nil {
			return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("lock active group: %w", err)}
		}
		if group == nil {
			return &ActiveError{Op: "EndSupervision", Err: ErrActiveGroupNotFound}
		}
		current, err := s.SupervisorRepo.FindByID(txCtx, id)
		if err != nil {
			return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("recheck supervision: %w", err)}
		}
		if current == nil || current.GroupID != supervision.GroupID {
			return &ActiveError{Op: "EndSupervision", Err: ErrGroupSupervisorNotFound}
		}
		if err := s.SupervisorRepo.EndSupervision(txCtx, id); err != nil {
			return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("end supervision failed: %w", err)}
		}
		s.queueSupervisionRefresh(txCtx, supervision.GroupID, activeSupervisionReasonSupervisorsChanged)
		return nil
	})
}

// ReleaseSupervision ends the staff member's own supervision of a running
// session while another supervisor stays (#4763), and returns the ended
// supervision. It participates in the caller's unit of work and opens none:
// the session row lock it takes there serializes two supervisors releasing at
// once, so the second one sees that it is the last and is refused. Visits and
// the session stay as they are.
func (s *service) ReleaseSupervision(ctx context.Context, activeGroupID, staffID int64) (studentpresence.GroupSupervision, error) {
	ended, err := s.releaseSupervisionLocked(ctx, activeGroupID, staffID)
	if err != nil {
		return studentpresence.GroupSupervision{}, &ActiveError{Op: "ReleaseSupervision", Err: err}
	}
	return groupSupervision(ended), nil
}

// releaseSupervisionLocked ends the staff member's supervision under the
// session row lock and re-reads the ended row.
func (s *service) releaseSupervisionLocked(ctx context.Context, activeGroupID, staffID int64) (*ports.GroupSupervisor, error) {
	// Outside a unit of work the row lock would not hold, and two releases
	// could both pass the last-supervisor check.
	if err := tenant.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	if _, err := s.lockActiveGroupOpenForUpdate(ctx, activeGroupID); err != nil {
		if IsNoRows(err) {
			return nil, ErrActiveGroupNotFound
		}
		return nil, err
	}
	supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return nil, fmt.Errorf("%w: read supervisors: %w", ErrDatabaseOperation, err)
	}
	own, others := releasedSupervision(supervisors, staffID)
	if own == nil {
		return nil, ErrGroupSupervisorNotFound
	}
	if others == 0 {
		return nil, ErrLastSupervisor
	}
	if err := s.SupervisorRepo.EndSupervision(ctx, own.ID); err != nil {
		return nil, fmt.Errorf("end supervision failed: %w", err)
	}
	ended, err := s.SupervisorRepo.FindByID(ctx, own.ID)
	if err != nil {
		return nil, fmt.Errorf("reread ended supervision: %w", err)
	}
	if ended == nil {
		return nil, ErrGroupSupervisorNotFound
	}
	return ended, nil
}

// releasedSupervision finds the staff member's current supervision and counts
// the other staff members supervising the session.
func releasedSupervision(supervisors []*ports.GroupSupervisor, staffID int64) (*ports.GroupSupervisor, int) {
	var own *ports.GroupSupervisor
	others := map[int64]struct{}{}
	for _, supervisor := range supervisors {
		if supervisor.StaffID == staffID {
			own = supervisor
			continue
		}
		others[supervisor.StaffID] = struct{}{}
	}
	return own, len(others)
}
