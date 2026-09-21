package presence

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Group Supervisor operations
func (s *service) CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrInvalidData}
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.createGroupSupervisor(txCtx, supervisor)
	})
}

func (s *service) createGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
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

	// Taking over a supervision that starts today means the staff member is
	// working right now — auto-open their work session so they show as
	// "Anwesend" (issue #1439). Kiosk-driven session starts already do this
	// in assignMultipleSupervisorsNonCritical; this covers the web app path.
	if supervisor.StartDate == s.todayDate() {
		source := stampSourceApp
		if s.attendancePrincipal(ctx).IsIoT {
			source = stampSourceNFC
		}
		s.ensureStaffPresence(ctx, supervisor.StaffID, source)
	}

	return nil
}

func (s *service) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
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
func (s *service) updateGroupSupervisorLocked(ctx context.Context, original, supervisor *active.GroupSupervisor) error {
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
	return nil
}

// supervisionForWrite reads the supervision a write changes; a failed read
// counts as not found.
func (s *service) supervisionForWrite(ctx context.Context, id int64, op string) (*active.GroupSupervisor, error) {
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
		return nil
	})
}

func (s *service) EndSupervision(ctx context.Context, id int64) error {
	supervision, err := s.SupervisorRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
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
		return nil
	})
}
