package education

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

const additionalSupervisorRole = "additional_supervisor"

func (s *substitutionModule) listRunningSupervisions(
	ctx context.Context,
	access substitutionAccess,
	query OverviewQuery,
	broad bool,
	targets []StaffRef,
) ([]RunningSupervision, error) {
	if s.deps.ActiveGroups == nil || s.deps.ActiveSupervisors == nil {
		return []RunningSupervision{}, nil
	}

	groups, byGroup, loaded, err := s.loadRunningSupervisionState(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RunningSupervision, 0, len(groups))
	for _, group := range groups {
		if query.ActiveGroupID > 0 && group.ID != query.ActiveGroupID {
			continue
		}
		own := actorSupervises(access.actor, byGroup[group.ID])
		if !broad && !own {
			continue
		}
		if hydrated := loaded[group.ID]; hydrated != nil {
			group = hydrated
		}
		result = append(result, projectRunningSupervision(group, byGroup[group.ID], targets, access, own))
	}
	if query.ActiveGroupID > 0 && len(result) == 0 {
		return nil, ErrNotFound
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *substitutionModule) loadRunningSupervisionState(ctx context.Context) (
	[]*activeModels.Group,
	map[int64][]*activeModels.GroupSupervisor,
	map[int64]*activeModels.Group,
	error,
) {
	groups, err := s.deps.ActiveGroups.FindActiveGroups(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	groupIDs := activeGroupIDs(groups)
	supervisors, err := s.deps.ActiveSupervisors.FindByActiveGroupIDs(ctx, groupIDs, true)
	if err != nil {
		return nil, nil, nil, err
	}
	loaded, err := s.deps.ActiveGroups.FindByIDs(ctx, groupIDs)
	return groups, supervisorsByGroup(supervisors), loaded, err
}

func activeGroupIDs(groups []*activeModels.Group) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

func supervisorsByGroup(supervisors []*activeModels.GroupSupervisor) map[int64][]*activeModels.GroupSupervisor {
	result := make(map[int64][]*activeModels.GroupSupervisor)
	for _, supervisor := range supervisors {
		result[supervisor.GroupID] = append(result[supervisor.GroupID], supervisor)
	}
	return result
}

func projectRunningSupervision(
	group *activeModels.Group,
	supervisors []*activeModels.GroupSupervisor,
	caregivers []StaffRef,
	access substitutionAccess,
	own bool,
) RunningSupervision {
	result := RunningSupervision{
		ID: group.ID, Type: TargetAdditionalSupervision,
		Supervisors: []StaffRef{}, AvailableTargets: []StaffRef{},
		IsCurrentUserSupervising: own, CanAssign: access.admin || own,
	}
	if group.ActualGroup != nil {
		result.Name = group.ActualGroup.Name
	}
	if group.Room != nil {
		result.RoomName = group.Room.Name
		if result.Name == "" {
			result.Name = group.Room.Name
		}
	}
	participantIDs := make(map[int64]struct{}, len(supervisors)+1)
	for _, supervisor := range supervisors {
		participantIDs[supervisor.StaffID] = struct{}{}
		result.Supervisors = append(result.Supervisors, supervisorRef(supervisor))
	}
	if access.actor != nil {
		participantIDs[access.actor.StaffID] = struct{}{}
	}
	for _, caregiver := range caregivers {
		if _, participating := participantIDs[caregiver.ID]; !participating {
			result.AvailableTargets = append(result.AvailableTargets, caregiver)
		}
	}
	return result
}

func supervisorRef(supervisor *activeModels.GroupSupervisor) StaffRef {
	ref := StaffRef{ID: supervisor.StaffID}
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		ref.FullName = supervisor.Staff.Person.GetFullName()
	}
	return ref
}

func actorSupervises(actor *Actor, supervisors []*activeModels.GroupSupervisor) bool {
	if actor == nil {
		return false
	}
	for _, supervisor := range supervisors {
		if supervisor.StaffID == actor.StaffID {
			return true
		}
	}
	return false
}

func (s *substitutionModule) assignAdditionalSupervision(
	ctx context.Context,
	caller SubstitutionCaller,
	request *AdditionalSupervisionAssignment,
) (*AssignmentResult, error) {
	if request.ActiveGroupID <= 0 || request.TargetStaffID <= 0 ||
		s.deps.ActiveGroups == nil || s.deps.ActiveSupervisors == nil || s.deps.ActiveSupervisorCreator == nil {
		return nil, ErrInvalidTarget
	}
	access, err := s.resolveAccess(ctx, caller)
	if err != nil {
		return nil, err
	}
	broad, err := s.canSeeAll(ctx, caller, access)
	if err != nil {
		return nil, err
	}
	if access.actor != nil && access.actor.StaffID == request.TargetStaffID {
		return nil, ErrSelfAssignment
	}

	var result AssignmentResult
	err = s.tx.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		created, target, createErr := s.assignAdditionalSupervisionLocked(txCtx, caller, access, broad, request)
		if createErr == nil {
			result = AssignmentResult{
				ID: created.ID, Type: TargetAdditionalSupervision, ActiveGroupID: created.GroupID,
				Target: StaffRef{ID: target.StaffID, FullName: target.FullName()},
			}
		}
		return createErr
	})
	if err != nil {
		return nil, err
	}
	realtime.QueueGroupAccessChanged(ctx, s.deps.Broadcaster, s.deps.Logger, "additional_supervision_assign")
	realtime.QueueActiveSupervisionChanged(ctx, s.deps.Broadcaster, s.deps.Logger, request.ActiveGroupID, "additional_supervisor_assigned")
	return &result, nil
}

func (s *substitutionModule) assignAdditionalSupervisionLocked(
	ctx context.Context,
	caller SubstitutionCaller,
	access substitutionAccess,
	broad bool,
	request *AdditionalSupervisionAssignment,
) (*activeModels.GroupSupervisor, *userModels.ActiveCaregiver, error) {
	group, err := s.lockAssignableSupervision(ctx, caller, access, broad, request)
	if err != nil {
		return nil, nil, err
	}
	target, err := s.findAndLockTarget(ctx, request.TargetStaffID)
	if err != nil {
		return nil, nil, err
	}
	created := &activeModels.GroupSupervisor{
		StaffID: target.StaffID, GroupID: group.ID, Role: additionalSupervisorRole,
		StartDate: timezone.DateFromTime(s.deps.Now()),
	}
	if err := s.deps.ActiveSupervisorCreator.CreateGroupSupervisor(ctx, created); err != nil {
		return nil, nil, err
	}
	if err := s.deps.Audit.Create(ctx, additionalSupervisionAudit(created, caller.AccountID)); err != nil {
		return nil, nil, err
	}
	return created, target, nil
}

func (s *substitutionModule) lockAssignableSupervision(
	ctx context.Context,
	caller SubstitutionCaller,
	access substitutionAccess,
	broad bool,
	request *AdditionalSupervisionAssignment,
) (*activeModels.Group, error) {
	group, err := s.deps.ActiveGroups.FindByIDForUpdate(ctx, request.ActiveGroupID)
	if err != nil {
		return nil, notFoundError(err)
	}
	if group == nil || group.TenantID != caller.TenantID {
		return nil, ErrNotFound
	}
	if group.EndTime != nil {
		return nil, ErrNotRunning
	}
	supervisors, err := s.deps.ActiveSupervisors.FindByActiveGroupID(ctx, group.ID, true)
	if err != nil {
		return nil, err
	}
	if !access.admin && !actorSupervises(access.actor, supervisors) {
		if broad {
			return nil, ErrForbidden
		}
		return nil, ErrNotFound
	}
	for _, supervisor := range supervisors {
		if supervisor.StaffID == request.TargetStaffID {
			return nil, ErrAlreadyAssigned
		}
	}
	return group, nil
}

func additionalSupervisionAudit(row *activeModels.GroupSupervisor, actorID int64) *auditModels.SubstitutionChange {
	return &auditModels.SubstitutionChange{
		SubstitutionID: row.ID, TargetType: string(TargetAdditionalSupervision), Action: auditModels.SubstitutionAssigned,
		GroupID: row.GroupID, TargetStaffID: row.StaffID, ActorAccountID: actorID,
		StartDate: row.StartDate,
	}
}
