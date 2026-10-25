package groups

import (
	"context"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	externals := map[int64]StaffRef{}
	if len(groups) > 0 {
		if targets, externals, err = s.withExternalTargets(ctx, targets); err != nil {
			return nil, err
		}
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
		result = append(result, projectRunningSupervision(group.ID, loaded[group.ID], byGroup[group.ID], targets, externals, access, own))
	}
	if query.ActiveGroupID > 0 && len(result) == 0 {
		return nil, ErrNotFound
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// withExternalTargets appends the assignable external caregivers to the
// caregiver targets: an external caregiver (#3823) with an account or an
// open invitation (#4573) can help with a running supervision, while group
// handovers keep offering caregivers only. It also returns every external
// caregiver by staff ID, so one already supervising stays labelled.
func (s *substitutionModule) withExternalTargets(ctx context.Context, caregivers []StaffRef) ([]StaffRef, map[int64]StaffRef, error) {
	if s.deps.ExternalCaregivers == nil {
		return caregivers, map[int64]StaffRef{}, nil
	}
	members, err := s.deps.ExternalCaregivers.ListExternalCaregivers(ctx)
	if err != nil {
		return nil, nil, err
	}
	result := make([]StaffRef, 0, len(caregivers)+len(members))
	result = append(result, caregivers...)
	externals := make(map[int64]StaffRef, len(members))
	for _, member := range members {
		ref := StaffRef{
			ID: member.StaffID, FullName: member.FullName, IsExternal: true,
			ExternalAccountStatus: member.AccountStatus, ExternalOrganization: member.Organization,
		}
		externals[member.StaffID] = ref
		result = append(result, ref)
	}
	return result, externals, nil
}

// findAndLockSupervisionTarget resolves an additional supervisor: an active
// caregiver or, when the directory is wired, an external caregiver with or
// without an account.
func (s *substitutionModule) findAndLockSupervisionTarget(ctx context.Context, staffID int64) (*domain.Caregiver, bool, error) {
	target, err := s.findAndLockTarget(ctx, staffID)
	if !errors.Is(err, ErrNotFound) || s.deps.ExternalCaregivers == nil {
		return target, false, err
	}
	externals, err := s.deps.ExternalCaregivers.ListExternalCaregivers(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, member := range externals {
		if member.StaffID == staffID {
			return member, true, nil
		}
	}
	return nil, false, ErrNotFound
}

func (s *substitutionModule) loadRunningSupervisionState(ctx context.Context) (
	[]*studentpresence.LiveGroup,
	map[int64][]*studentpresence.StaffedSupervision,
	map[int64]*studentpresence.SessionDetail,
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

func activeGroupIDs(groups []*studentpresence.LiveGroup) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

func supervisorsByGroup(supervisors []*studentpresence.StaffedSupervision) map[int64][]*studentpresence.StaffedSupervision {
	result := make(map[int64][]*studentpresence.StaffedSupervision)
	for _, supervisor := range supervisors {
		result[supervisor.GroupID] = append(result[supervisor.GroupID], supervisor)
	}
	return result
}

// projectRunningSupervision names the session after its activity template,
// else its room; a session the detail read missed stays unnamed.
func projectRunningSupervision(
	groupID int64,
	group *studentpresence.SessionDetail,
	supervisors []*studentpresence.StaffedSupervision,
	caregivers []StaffRef,
	externals map[int64]StaffRef,
	access substitutionAccess,
	own bool,
) RunningSupervision {
	result := RunningSupervision{
		ID: groupID, Type: TargetAdditionalSupervision,
		Supervisors: []StaffRef{}, AvailableTargets: []StaffRef{},
		IsCurrentUserSupervising: own, CanAssign: access.admin || own,
	}
	if group != nil && group.Activity != nil {
		result.Name = group.Activity.Name
	}
	if group != nil && group.Room != nil {
		result.RoomName = group.Room.Name
		if result.Name == "" {
			result.Name = group.Room.Name
		}
	}
	participantIDs := make(map[int64]struct{}, len(supervisors)+1)
	for _, supervisor := range supervisors {
		participantIDs[supervisor.StaffID] = struct{}{}
		external := externals[supervisor.StaffID]
		result.Supervisors = append(result.Supervisors, StaffRef{
			ID: supervisor.StaffID, FullName: supervisor.StaffName,
			IsExternal: external.IsExternal, ExternalAccountStatus: external.ExternalAccountStatus,
			ExternalOrganization: external.ExternalOrganization,
		})
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

func actorSupervises(actor *Actor, supervisors []*studentpresence.StaffedSupervision) bool {
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
	if request.HandOver {
		return s.handOverSupervision(ctx, caller, request)
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

	created, target, err := s.assignAdditionalSupervisionLocked(ctx, caller, access, broad, request)
	if err != nil {
		return nil, err
	}
	s.announceSupervisionChange(ctx, request.ActiveGroupID, "additional_supervision_assign", "additional_supervisor_assigned")
	return additionalSupervisionResult(created, target), nil
}

func additionalSupervisionResult(created *studentpresence.GroupSupervision, target *domain.Caregiver) *AssignmentResult {
	return &AssignmentResult{ID: created.ID, Type: TargetAdditionalSupervision, ActiveGroupID: created.GroupID,
		Target: StaffRef{ID: target.StaffID, FullName: target.FullName}}
}

// announceSupervisionChange wakes the open views after the supervisors of a
// running session changed: the group-access signal refreshes the sidebars and
// substitution lists, the supervision signal the live supervision views.
func (s *substitutionModule) announceSupervisionChange(ctx context.Context, activeGroupID int64, accessSource, reason string) {
	if s.deps.Broadcaster == nil {
		return
	}
	s.deps.Broadcaster.QueueGroupAccessChanged(ctx, accessSource)
	s.deps.Broadcaster.QueueActiveSupervisionChanged(ctx, activeGroupID, reason)
}

func (s *substitutionModule) assignAdditionalSupervisionLocked(
	ctx context.Context,
	caller SubstitutionCaller,
	access substitutionAccess,
	broad bool,
	request *AdditionalSupervisionAssignment,
) (*studentpresence.GroupSupervision, *domain.Caregiver, error) {
	group, supervisors, err := s.lockAssignableSupervision(ctx, caller, access, broad, request)
	if err != nil {
		return nil, nil, err
	}
	target, isExternal, err := s.findAndLockSupervisionTarget(ctx, request.TargetStaffID)
	if err != nil {
		return nil, nil, err
	}
	created := &studentpresence.GroupSupervision{
		StaffID: target.StaffID, GroupID: group.ID, Role: assignedSupervisionRole(request, access.actor, supervisors),
		StartDate: calendar.DateFromTime(s.deps.Now()).String(), SkipPresenceStamp: isExternal,
	}
	if err := s.deps.ActiveSupervisorCreator.CreateGroupSupervisor(ctx, created); err != nil {
		return nil, nil, err
	}
	if err := s.deps.Audit.RecordSubstitutionChange(ctx, additionalSupervisionAudit(created, caller.AccountID)); err != nil {
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
) (*studentpresence.LiveGroup, []*studentpresence.StaffedSupervision, error) {
	group, err := s.deps.ActiveGroups.FindByIDForUpdate(ctx, request.ActiveGroupID)
	if err != nil {
		return nil, nil, notFoundError(err)
	}
	if group == nil || group.TenantID != caller.TenantID {
		return nil, nil, ErrNotFound
	}
	if group.EndTime != nil {
		return nil, nil, ErrNotRunning
	}
	supervisors, err := s.deps.ActiveSupervisors.FindByActiveGroupID(ctx, group.ID, true)
	if err != nil {
		return nil, nil, err
	}
	if !access.admin && !actorSupervises(access.actor, supervisors) {
		if broad {
			return nil, nil, ErrForbidden
		}
		return nil, nil, ErrNotFound
	}
	for _, supervisor := range supervisors {
		if supervisor.StaffID == request.TargetStaffID {
			return nil, nil, ErrAlreadyAssigned
		}
	}
	return group, supervisors, nil
}

// assignedSupervisionRole is the role the target joins with: an added person
// is an additional supervisor, a hand-over passes on the role the caller held
// (#4763), so the session keeps its supervisor when the original one leaves.
func assignedSupervisionRole(request *AdditionalSupervisionAssignment, actor *Actor, supervisors []*studentpresence.StaffedSupervision) string {
	if !request.HandOver || actor == nil {
		return additionalSupervisorRole
	}
	for _, supervisor := range supervisors {
		if supervisor.StaffID == actor.StaffID && supervisor.Role != "" {
			return supervisor.Role
		}
	}
	return additionalSupervisorRole
}

func additionalSupervisionAudit(row *studentpresence.GroupSupervision, actorID int64) domain.SubstitutionChange {
	return domain.SubstitutionChange{
		SubstitutionID: row.ID, TargetType: string(TargetAdditionalSupervision), Action: domain.SubstitutionAssigned,
		GroupID: row.GroupID, TargetStaffID: row.StaffID, ActorAccountID: actorID,
		StartDate: calendar.Date(row.StartDate),
	}
}
