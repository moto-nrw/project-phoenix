package presence

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// UpdateActiveGroupSupervisors replaces all supervisors for an active group
func (s *service) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*ports.ActiveGroup, error) {
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	uniqueSupervisors := deduplicateSupervisorIDs(supervisorIDs)
	if err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.lockSupervisorsForAssignment(txCtx, supervisorIDs); err != nil {
			return err
		}
		if err := s.lockActiveGroupForSupervisorUpdate(txCtx, activeGroupID); err != nil {
			return err
		}
		return s.replaceSupervisorsInTransaction(txCtx, activeGroupID, uniqueSupervisors)
	}); err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	// Supervisor takeover/handover means these staff members are on site —
	// auto-open their work sessions so they show as "Anwesend" (issue #1439).
	// Same best-effort semantics as the session-start auto-stamp.
	source := stampSourceApp
	if s.attendancePrincipal(ctx).IsIoT {
		source = stampSourceNFC
	}
	for staffID := range uniqueSupervisors {
		s.ensureStaffPresence(ctx, staffID, source)
	}

	updatedGroup, err := s.GroupRepo.FindWithSupervisors(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	return updatedGroup, nil
}

// Lock requested staff in a stable order before locking supervision rows.
// Revalidation inside the write transaction closes the preflight/offboarding gap.
func (s *service) lockSupervisorsForAssignment(ctx context.Context, supervisorIDs []int64) error {
	ids := slices.Collect(maps.Keys(deduplicateSupervisorIDs(supervisorIDs)))
	slices.Sort(ids)
	for _, id := range ids {
		if err := s.lockStaffForSupervision(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// validateActiveGroupForSupervisorUpdate validates that the group exists and is active
func (s *service) lockActiveGroupForSupervisorUpdate(ctx context.Context, activeGroupID int64) error {
	activeGroup, err := s.GroupRepo.FindByIDForUpdate(ctx, activeGroupID)
	if err != nil || activeGroup == nil {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: ErrActiveGroupNotFound}
	}

	if !activeGroup.IsActive() {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: fmt.Errorf("cannot update supervisors for an ended session")}
	}

	return nil
}

func (s *service) lockGroupRows(ctx context.Context, groupIDs ...int64) error {
	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		if id > 0 {
			unique[id] = struct{}{}
		}
	}
	ids := slices.Collect(maps.Keys(unique))
	slices.Sort(ids)
	for _, id := range ids {
		group, err := s.GroupRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if group == nil {
			return ErrActiveGroupNotFound
		}
	}
	return nil
}

// deduplicateSupervisorIDs removes duplicate supervisor IDs
func deduplicateSupervisorIDs(supervisorIDs []int64) map[int64]bool {
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}
	return uniqueSupervisors
}

// replaceSupervisorsInTransaction replaces all supervisors for a group within a transaction
func (s *service) replaceSupervisorsInTransaction(ctx context.Context, activeGroupID int64, uniqueSupervisors map[int64]bool) error {
	currentSupervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return err
	}

	uniqueSupervisors = primarySupervisorIDs(uniqueSupervisors, currentSupervisors)
	if err := s.endAllCurrentSupervisors(ctx, currentSupervisors); err != nil {
		return err
	}

	return s.upsertSupervisors(ctx, activeGroupID, uniqueSupervisors, currentSupervisors)
}

// primarySupervisorIDs excludes non-primary roles from a primary-supervisor
// replacement. IoT check-ins pass every active supervisor ID, including
// additional supervisors, which must remain assigned to the session.
func primarySupervisorIDs(supervisorIDs map[int64]bool, currentSupervisors []*ports.GroupSupervisor) map[int64]bool {
	primaryIDs := maps.Clone(supervisorIDs)
	for _, supervisor := range currentSupervisors {
		if supervisor.Role != "supervisor" {
			delete(primaryIDs, supervisor.StaffID)
		}
	}
	return primaryIDs
}

// endAllCurrentSupervisors ends the current primary supervisors by setting end_date.
func (s *service) endAllCurrentSupervisors(ctx context.Context, supervisors []*ports.GroupSupervisor) error {
	today := timezone.TodayDate()
	for _, supervisor := range supervisors {
		if supervisor.Role != "supervisor" {
			continue
		}
		supervisor.EndDate = &today
		if err := s.SupervisorRepo.Update(ctx, supervisor); err != nil {
			return err
		}
	}
	return nil
}

// upsertSupervisors creates new supervisors or reactivates existing ones
func (s *service) upsertSupervisors(ctx context.Context, activeGroupID int64, uniqueSupervisors map[int64]bool, currentSupervisors []*ports.GroupSupervisor) error {
	now := time.Now()

	for supervisorID := range uniqueSupervisors {
		existingSuper := s.findExistingSupervisor(currentSupervisors, supervisorID)

		if existingSuper != nil {
			if err := s.reactivateSupervisor(ctx, existingSuper, now); err != nil {
				return err
			}
		} else {
			if err := s.createNewSupervisor(ctx, activeGroupID, supervisorID, now); err != nil {
				return err
			}
		}
	}

	return nil
}

// findExistingSupervisor finds a supervisor in the list by staff ID and role
func (s *service) findExistingSupervisor(supervisors []*ports.GroupSupervisor, staffID int64) *ports.GroupSupervisor {
	for _, existing := range supervisors {
		if existing.StaffID == staffID && existing.Role == "supervisor" {
			return existing
		}
	}
	return nil
}

// reactivateSupervisor reactivates an ended supervisor
func (s *service) reactivateSupervisor(ctx context.Context, supervisor *ports.GroupSupervisor, now time.Time) error {
	if supervisor.EndDate == nil {
		return nil
	}

	supervisor.EndDate = nil
	supervisor.StartDate = timezone.DateFromTime(now)
	return s.SupervisorRepo.Update(ctx, supervisor)
}

// createNewSupervisor creates a new supervisor record
func (s *service) createNewSupervisor(ctx context.Context, activeGroupID, supervisorID int64, now time.Time) error {
	supervisor := &ports.GroupSupervisor{
		StaffID:   supervisorID,
		GroupID:   activeGroupID,
		Role:      "supervisor",
		StartDate: timezone.DateFromTime(now),
	}
	supervisor.SetTenantID(tenant.FromContext(ctx))
	return s.SupervisorRepo.Create(ctx, supervisor)
}
