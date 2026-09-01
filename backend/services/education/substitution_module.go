// The substitution module exposes typed operations and narrow projections;
// storage-specific models stay behind its interface.
package education

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type GroupStore interface {
	FindByTeacher(ctx context.Context, teacherID int64) ([]*educationModels.Group, error)
	FindByIDForUpdate(ctx context.Context, id any) (*educationModels.Group, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*educationModels.Group, error)
}

type GroupHandoverStore interface {
	Create(ctx context.Context, handover *educationModels.GroupSubstitution) error
	Delete(ctx context.Context, id any) error
	FindByID(ctx context.Context, id any) (*educationModels.GroupSubstitution, error)
	FindByIDForUpdate(ctx context.Context, id any) (*educationModels.GroupSubstitution, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*educationModels.GroupSubstitution, error)
	ListWithRelations(ctx context.Context, options *base.QueryOptions) ([]*educationModels.GroupSubstitution, error)
}

type StaffLockStore interface {
	FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Staff, error)
}

type ActiveSupervisorCreator interface {
	CreateGroupSupervisor(context.Context, *activeModels.GroupSupervisor) error
}

type SubstitutionDependencies struct {
	Groups                  GroupStore
	Substitutions           GroupHandoverStore
	Teachers                userModels.TeacherRepository
	Staff                   StaffLockStore
	Actors                  ActorResolver
	Audit                   auditModels.SubstitutionChangeCreator
	DB                      *bun.DB
	Broadcaster             realtime.Broadcaster
	Logger                  *slog.Logger
	Now                     func() time.Time
	CanSeeAll               func(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error)
	Schedule                ScheduleSubstitutionAdapter
	ActiveGroups            activeModels.GroupRepository
	ActiveSupervisors       activeModels.GroupSupervisorRepository
	ActiveSupervisorCreator ActiveSupervisorCreator
}

type substitutionModule struct {
	deps SubstitutionDependencies
	tx   *tenant.TransactionRunner
}

type substitutionAccess struct {
	admin         bool
	actor         *Actor
	ownedGroupIDs []int64
}

func NewSubstitutionModule(deps SubstitutionDependencies) SubstitutionModule {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &substitutionModule{deps: deps, tx: tenant.NewTransactionRunner()}
}

func (s *substitutionModule) Overview(ctx context.Context, caller SubstitutionCaller, query OverviewQuery) (*OverviewResult, error) {
	scheduleRequested := query.ScheduleFrom != nil || query.ScheduleTo != nil || query.IncludeScheduleTargets
	if scheduleRequested {
		return s.scheduleOverview(ctx, caller, query)
	}
	if s.deps.Groups == nil || s.deps.Substitutions == nil || s.deps.Teachers == nil {
		return nil, ErrInvalidTarget
	}
	return s.groupOverview(ctx, caller, query)
}

func (s *substitutionModule) scheduleOverview(ctx context.Context, caller SubstitutionCaller, query OverviewQuery) (*OverviewResult, error) {
	if query.ScheduleFrom == nil || query.ScheduleTo == nil || s.deps.Schedule == nil {
		return nil, ErrInvalidPeriod
	}
	if err := authorizeScheduleCaller(ctx, caller, "schedules:read"); err != nil {
		return nil, err
	}
	overview, err := s.deps.Schedule.Overview(
		ctx, *query.ScheduleFrom, *query.ScheduleTo, query.IncludeScheduleTargets,
		caller.HasPermission("schedules:manage"),
	)
	if err != nil {
		return nil, err
	}
	result := emptyOverview()
	result.ScheduleAppointments, result.ScheduleTargets = overview.Appointments, overview.Targets
	return result, nil
}

func (s *substitutionModule) groupOverview(ctx context.Context, caller SubstitutionCaller, query OverviewQuery) (*OverviewResult, error) {
	access, broad, visibleGroupIDs, today, err := s.groupOverviewScope(ctx, caller, query)
	if err != nil {
		return nil, err
	}
	if err := validateOverviewQuery(query, access.admin, today); err != nil {
		return nil, err
	}
	rows, err := s.listOverviewRows(ctx, caller.TenantID, query, access.admin, broad, visibleGroupIDs, today)
	if err != nil {
		return nil, err
	}
	if !broad && query.GroupID > 0 && !canViewGroup(access, rows, query.GroupID) {
		return nil, ErrNotFound
	}
	result, err := s.projectOverview(ctx, caller.TenantID, access, rows, query.IncludeTargets)
	if err != nil {
		return nil, err
	}
	result.RunningSupervisions, err = s.listRunningSupervisions(ctx, access, query, broad, result.Targets)
	return result, err
}

func (s *substitutionModule) groupOverviewScope(ctx context.Context, caller SubstitutionCaller, query OverviewQuery) (substitutionAccess, bool, []int64, timezone.Date, error) {
	access, err := s.resolveAccess(ctx, caller)
	if err != nil {
		return access, false, nil, timezone.Date(""), err
	}
	broad, err := s.canSeeAll(ctx, caller, access)
	today := timezone.DateFromTime(s.deps.Now())
	visibleGroupIDs := access.ownedGroupIDs
	if err == nil && !broad && query.GroupID == 0 {
		visibleGroupIDs, err = s.visibleGroupIDs(ctx, access, caller.TenantID, today)
	}
	return access, broad, visibleGroupIDs, today, err
}

func emptyOverview() *OverviewResult {
	return &OverviewResult{
		GroupHandovers: []GroupHandover{}, Groups: []GroupRef{}, Targets: []StaffRef{}, RunningSupervisions: []RunningSupervision{},
		ScheduleAppointments: []ScheduleAppointmentOverview{}, ScheduleTargets: []StaffRef{},
	}
}

func validateOverviewQuery(query OverviewQuery, admin bool, today timezone.Date) error {
	if query.GroupID < 0 || query.ActiveGroupID < 0 {
		return ErrInvalidTarget
	}
	if !admin && query.On != nil && *query.On != today {
		return ErrInvalidPeriod
	}
	return nil
}

func (s *substitutionModule) listOverviewRows(ctx context.Context, tenantID int64, query OverviewQuery, admin, broad bool, visibleGroupIDs []int64, today timezone.Date) ([]*educationModels.GroupSubstitution, error) {
	options := base.NewQueryOptions()
	filter := base.NewFilter().Equal("tenant_id", tenantID).
		Equal("target_type", educationModels.GroupSubstitutionTypeGroupHandover)
	if query.GroupID > 0 {
		filter.Equal("group_id", query.GroupID)
	} else if !broad {
		values := make([]any, len(visibleGroupIDs))
		for i, id := range visibleGroupIDs {
			values[i] = id
		}
		if len(values) == 0 {
			return []*educationModels.GroupSubstitution{}, nil
		}
		filter.In("group_id", values...)
	}
	if !admin {
		filter.LessThanOrEqual("start_date", today).GreaterThanOrEqual("end_date", today)
	} else if query.On != nil {
		filter.LessThanOrEqual("start_date", *query.On).GreaterThanOrEqual("end_date", *query.On)
	} else {
		filter.GreaterThanOrEqual("end_date", today)
	}
	options.Filter = filter
	return s.deps.Substitutions.ListWithRelations(ctx, options)
}

func (s *substitutionModule) projectOverview(ctx context.Context, tenantID int64, access substitutionAccess, rows []*educationModels.GroupSubstitution, includeTargets bool) (*OverviewResult, error) {
	result := emptyOverview()
	result.GroupHandovers = make([]GroupHandover, 0, len(rows))
	for _, row := range rows {
		if row.TenantID != tenantID || row.TargetType != educationModels.GroupSubstitutionTypeGroupHandover {
			return nil, ErrForbidden
		}
		result.GroupHandovers = append(result.GroupHandovers, project(row, access.admin || contains(access.ownedGroupIDs, row.GroupID)))
	}
	if includeTargets {
		groups, err := s.listAssignableGroups(ctx, tenantID, access)
		if err != nil {
			return nil, err
		}
		targets, err := s.listTargets(ctx, access.actor)
		if err != nil {
			return nil, err
		}
		result.Groups = groups
		result.Targets = targets
	}
	return result, nil
}

func (s *substitutionModule) listAssignableGroups(ctx context.Context, tenantID int64, access substitutionAccess) ([]GroupRef, error) {
	options := base.NewQueryOptions()
	filter := base.NewFilter().Equal("tenant_id", tenantID)
	if !access.admin {
		if len(access.ownedGroupIDs) == 0 {
			return []GroupRef{}, nil
		}
		ids := make([]any, len(access.ownedGroupIDs))
		for index, id := range access.ownedGroupIDs {
			ids[index] = id
		}
		filter.In("id", ids...)
	}
	options.Filter = filter
	rows, err := s.deps.Groups.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	groups := make([]GroupRef, 0, len(rows))
	for _, row := range rows {
		if row.TenantID != tenantID {
			return nil, ErrForbidden
		}
		groups = append(groups, GroupRef{ID: row.ID, Name: row.Name})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func canViewGroup(access substitutionAccess, rows []*educationModels.GroupSubstitution, groupID int64) bool {
	if contains(access.ownedGroupIDs, groupID) {
		return true
	}
	for _, row := range rows {
		if access.actor != nil && row.GroupID == groupID && row.SubstituteStaffID == access.actor.StaffID {
			return true
		}
	}
	return false
}

func (s *substitutionModule) Assign(ctx context.Context, caller SubstitutionCaller, assignment Assignment) (*AssignmentResult, error) {
	if assignment.Type == TargetAdditionalSupervision && assignment.AdditionalSupervision != nil && assignment.GroupHandover == nil {
		return s.assignAdditionalSupervision(ctx, caller, assignment.AdditionalSupervision)
	}
	if assignment.Type == TargetScheduleSubstitution {
		return s.assignScheduleSubstitution(ctx, caller, assignment.ScheduleSubstitution)
	}
	if assignment.Type != TargetGroupHandover || assignment.GroupHandover == nil || assignment.AdditionalSupervision != nil {
		return nil, ErrInvalidTarget
	}
	request := assignment.GroupHandover
	access, err := s.resolveAccess(ctx, caller)
	if err != nil {
		return nil, err
	}
	start, end, err := s.period(access.admin, request.StartDate, request.EndDate)
	if err != nil {
		return nil, err
	}
	if request.GroupID <= 0 || request.TargetStaffID <= 0 {
		return nil, ErrInvalidTarget
	}
	if !access.admin && !contains(access.ownedGroupIDs, request.GroupID) {
		return nil, ErrNotFound
	}
	if access.actor != nil && access.actor.StaffID == request.TargetStaffID {
		return nil, ErrInvalidTarget
	}

	var result AssignmentResult
	err = s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		created, group, target, createErr := s.assignLocked(txCtx, caller, access, request, start, end)
		if createErr == nil {
			result = projectAssignment(created, group, target)
		}
		return createErr
	})
	if err != nil {
		return nil, err
	}
	realtimeevents.QueueGroupAccessChanged(ctx, s.deps.Broadcaster, s.deps.Logger, "substitution_assign")
	return &result, nil
}

func (s *substitutionModule) assignScheduleSubstitution(ctx context.Context, caller SubstitutionCaller, assignment *ScheduleSubstitutionAssignment) (*AssignmentResult, error) {
	if assignment == nil || s.deps.Schedule == nil {
		return nil, ErrInvalidTarget
	}
	if err := authorizeScheduleCaller(ctx, caller, "schedules:manage"); err != nil {
		return nil, err
	}
	var result *ScheduleSubstitutionResult
	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		var assignErr error
		result, assignErr = s.deps.Schedule.Assign(txCtx, *assignment, caller.AccountID)
		return assignErr
	})
	if err != nil {
		tenant.MarkRollback(ctx)
		return nil, err
	}
	notifyScheduleChange(ctx, result)
	return &AssignmentResult{ScheduleSubstitution: result}, nil
}

func authorizeScheduleCaller(ctx context.Context, caller SubstitutionCaller, required string) error {
	if caller.AccountID <= 0 || caller.TenantID <= 0 || caller.TenantID != tenant.FromContext(ctx) ||
		(caller.Scope != "" && caller.Scope != "org") {
		return ErrForbidden
	}
	if caller.HasPermission == nil || !caller.HasPermission(required) {
		return ErrForbidden
	}
	return nil
}

func (s *substitutionModule) assignLocked(ctx context.Context, caller SubstitutionCaller, access substitutionAccess, request *GroupHandoverAssignment, start, end timezone.Date) (*educationModels.GroupSubstitution, *educationModels.Group, *userModels.ActiveCaregiver, error) {
	group, err := s.lockTenantGroup(ctx, request.GroupID, caller.TenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.recheckOwnership(ctx, access, group.ID, caller.TenantID); err != nil {
		return nil, nil, nil, err
	}
	target, err := s.findAndLockTarget(ctx, request.TargetStaffID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.rejectDuplicate(ctx, caller.TenantID, request, start, end); err != nil {
		return nil, nil, nil, err
	}
	created := &educationModels.GroupSubstitution{
		TargetType: educationModels.GroupSubstitutionTypeGroupHandover,
		GroupID:    request.GroupID, SubstituteStaffID: target.StaffID,
		StartDate: start, EndDate: end, Reason: "Gruppenübergabe",
	}
	if err := s.deps.Substitutions.Create(ctx, created); err != nil {
		if base.IsUniqueViolation(err) {
			return nil, nil, nil, ErrAlreadyAssigned
		}
		return nil, nil, nil, err
	}
	if err := s.deps.Audit.Create(ctx, auditChange(created, caller.AccountID, auditModels.SubstitutionAssigned)); err != nil {
		return nil, nil, nil, err
	}
	return created, group, target, nil
}

func (s *substitutionModule) rejectDuplicate(ctx context.Context, tenantID int64, request *GroupHandoverAssignment, start, end timezone.Date) error {
	filter := base.NewFilter().Equal("tenant_id", tenantID).
		Equal("target_type", educationModels.GroupSubstitutionTypeGroupHandover).
		Equal("group_id", request.GroupID).
		Equal("substitute_staff_id", request.TargetStaffID).
		LessThanOrEqual("start_date", end).
		GreaterThanOrEqual("end_date", start)
	options := base.NewQueryOptions()
	options.Filter = filter
	duplicates, err := s.deps.Substitutions.ListWithOptions(ctx, options)
	if err != nil {
		return err
	}
	if len(duplicates) > 0 {
		return ErrAlreadyAssigned
	}
	return nil
}

func (s *substitutionModule) End(ctx context.Context, caller SubstitutionCaller, request EndRequest) error {
	if request.Type == TargetScheduleSubstitution {
		return s.endScheduleSubstitution(ctx, caller, request.ID)
	}
	if request.Type != TargetGroupHandover || request.ID <= 0 {
		return ErrInvalidTarget
	}
	access, err := s.resolveAccess(ctx, caller)
	if err != nil {
		return err
	}
	err = s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		return s.endLocked(txCtx, caller, access, request.ID)
	})
	if err != nil {
		return err
	}
	realtimeevents.QueueGroupAccessChanged(ctx, s.deps.Broadcaster, s.deps.Logger, "substitution_end")
	return nil
}

func (s *substitutionModule) endScheduleSubstitution(ctx context.Context, caller SubstitutionCaller, id int64) error {
	if id <= 0 || s.deps.Schedule == nil {
		return ErrInvalidTarget
	}
	if err := authorizeScheduleCaller(ctx, caller, "schedules:manage"); err != nil {
		return err
	}
	var result *ScheduleSubstitutionResult
	err := s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		var endErr error
		result, endErr = s.deps.Schedule.End(txCtx, id, caller.AccountID)
		return endErr
	})
	if err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	notifyScheduleChange(ctx, result)
	return nil
}

func notifyScheduleChange(ctx context.Context, result *ScheduleSubstitutionResult) {
	if result != nil && result.AfterCommit != nil {
		result.AfterCommit(ctx)
	}
}

func (s *substitutionModule) canSeeAll(ctx context.Context, caller SubstitutionCaller, access substitutionAccess) (bool, error) {
	if access.admin {
		return true, nil
	}
	if s.deps.CanSeeAll == nil {
		return false, nil
	}
	return s.deps.CanSeeAll(ctx, caller.Scope == "school", false, access.actor != nil)
}

func (s *substitutionModule) endLocked(ctx context.Context, caller SubstitutionCaller, access substitutionAccess, id int64) error {
	initial, err := s.deps.Substitutions.FindByID(ctx, id)
	if err != nil {
		return notFoundError(err)
	}
	if err := validateEndAccess(initial, caller.TenantID, access); err != nil {
		return err
	}
	if _, err := s.lockTenantGroup(ctx, initial.GroupID, caller.TenantID); err != nil {
		return err
	}
	if err := s.recheckOwnership(ctx, access, initial.GroupID, caller.TenantID); err != nil {
		return err
	}
	current, err := s.deps.Substitutions.FindByIDForUpdate(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			return ErrNotRunning
		}
		return err
	}
	if err := validateEndAccess(current, caller.TenantID, access); err != nil {
		return err
	}
	today := timezone.DateFromTime(s.deps.Now())
	if current.EndDate.Before(today) || (!access.admin && current.StartDate.After(today)) {
		return ErrNotRunning
	}
	if err := s.deps.Audit.Create(ctx, auditChange(current, caller.AccountID, auditModels.SubstitutionEnded)); err != nil {
		return err
	}
	return s.deps.Substitutions.Delete(ctx, id)
}

func validateEndAccess(row *educationModels.GroupSubstitution, tenantID int64, access substitutionAccess) error {
	if row.TargetType != educationModels.GroupSubstitutionTypeGroupHandover || row.TenantID != tenantID ||
		(!access.admin && !contains(access.ownedGroupIDs, row.GroupID)) {
		return ErrNotFound
	}
	return nil
}

func (s *substitutionModule) lockTenantGroup(ctx context.Context, groupID, tenantID int64) (*educationModels.Group, error) {
	group, err := s.deps.Groups.FindByIDForUpdate(ctx, groupID)
	if err != nil {
		return nil, notFoundError(err)
	}
	if group.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return group, nil
}

func (s *substitutionModule) recheckOwnership(ctx context.Context, access substitutionAccess, groupID, tenantID int64) error {
	if access.admin {
		return nil
	}
	if access.actor == nil {
		return ErrForbidden
	}
	groups, err := s.deps.Groups.FindByTeacher(ctx, access.actor.TeacherID)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.ID == groupID && group.TenantID == tenantID {
			return nil
		}
	}
	return ErrNotFound
}

func notFoundError(err error) error {
	if base.IsNoRows(err) {
		return ErrNotFound
	}
	return err
}

func (s *substitutionModule) resolveAccess(ctx context.Context, caller SubstitutionCaller) (substitutionAccess, error) {
	if caller.AccountID <= 0 || caller.TenantID <= 0 || (caller.Scope != "" && caller.Scope != "org") {
		return substitutionAccess{}, ErrForbidden
	}
	if caller.Admin {
		actor, err := s.resolveActor(ctx, caller.AccountID)
		return substitutionAccess{admin: true, actor: actor}, err
	}
	if !containsRole(caller.Roles, "user") && !containsRole(caller.Roles, "teacher") {
		return substitutionAccess{}, ErrForbidden
	}
	actor, err := s.resolveActor(ctx, caller.AccountID)
	if err != nil {
		return substitutionAccess{}, err
	}
	if actor == nil {
		return substitutionAccess{}, ErrForbidden
	}
	groups, err := s.deps.Groups.FindByTeacher(ctx, actor.TeacherID)
	if err != nil {
		return substitutionAccess{}, err
	}
	ids := make([]int64, len(groups))
	for i, group := range groups {
		if group.TenantID != caller.TenantID {
			return substitutionAccess{}, ErrForbidden
		}
		ids[i] = group.ID
	}
	return substitutionAccess{actor: actor, ownedGroupIDs: ids}, nil
}

func (s *substitutionModule) visibleGroupIDs(ctx context.Context, access substitutionAccess, tenantID int64, today timezone.Date) ([]int64, error) {
	if access.admin {
		return nil, nil
	}
	ids := append([]int64(nil), access.ownedGroupIDs...)
	filter := base.NewFilter().Equal("tenant_id", tenantID).
		Equal("target_type", educationModels.GroupSubstitutionTypeGroupHandover).
		Equal("substitute_staff_id", access.actor.StaffID).
		LessThanOrEqual("start_date", today).
		GreaterThanOrEqual("end_date", today)
	options := base.NewQueryOptions()
	options.Filter = filter
	rows, err := s.deps.Substitutions.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !contains(ids, row.GroupID) {
			ids = append(ids, row.GroupID)
		}
	}
	return ids, nil
}

func (s *substitutionModule) resolveActor(ctx context.Context, accountID int64) (*Actor, error) {
	if s.deps.Actors != nil {
		return s.deps.Actors.ResolveActor(ctx)
	}
	caregiver, err := s.deps.Teachers.FindActiveCaregiverByAccountID(ctx, accountID)
	if err != nil {
		if base.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if caregiver == nil {
		return nil, nil
	}
	return &Actor{StaffID: caregiver.StaffID, TeacherID: caregiver.TeacherID}, nil
}

func (s *substitutionModule) period(admin bool, requestedStart, requestedEnd *timezone.Date) (timezone.Date, timezone.Date, error) {
	today := timezone.DateFromTime(s.deps.Now())
	if !admin {
		if requestedStart != nil && *requestedStart != today || requestedEnd != nil && *requestedEnd != today {
			return timezone.Date(""), timezone.Date(""), ErrInvalidPeriod
		}
		return today, today, nil
	}
	if requestedStart == nil && requestedEnd == nil {
		return today, today, nil
	}
	if requestedStart == nil || requestedEnd == nil || requestedStart.IsZero() || requestedEnd.IsZero() || requestedEnd.Before(*requestedStart) || requestedStart.Before(today) {
		return timezone.Date(""), timezone.Date(""), ErrInvalidPeriod
	}
	return *requestedStart, *requestedEnd, nil
}

func (s *substitutionModule) findTarget(ctx context.Context, staffID int64) (*userModels.ActiveCaregiver, error) {
	caregivers, err := s.deps.Teachers.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, err
	}
	for _, caregiver := range caregivers {
		if caregiver.StaffID == staffID {
			return caregiver, nil
		}
	}
	return nil, ErrNotFound
}

func (s *substitutionModule) findAndLockTarget(ctx context.Context, staffID int64) (*userModels.ActiveCaregiver, error) {
	if _, err := s.deps.Staff.FindByIDForUpdate(ctx, staffID); err != nil {
		return nil, notFoundError(err)
	}
	return s.findTarget(ctx, staffID)
}

func (s *substitutionModule) listTargets(ctx context.Context, actor *Actor) ([]StaffRef, error) {
	caregivers, err := s.deps.Teachers.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StaffRef, 0, len(caregivers))
	for _, caregiver := range caregivers {
		if actor == nil || caregiver.StaffID != actor.StaffID {
			result = append(result, StaffRef{ID: caregiver.StaffID, FullName: caregiver.FullName()})
		}
	}
	return result, nil
}

func project(row *educationModels.GroupSubstitution, canEnd bool) GroupHandover {
	result := GroupHandover{ID: row.ID, Type: TargetGroupHandover, CanEnd: canEnd,
		Period: Period{StartDate: row.StartDate.String(), EndDate: row.EndDate.String()},
		Group:  GroupRef{ID: row.GroupID}, Target: StaffRef{ID: row.SubstituteStaffID}}
	if row.Group != nil {
		result.Group.Name = row.Group.Name
	}
	if row.SubstituteStaff != nil && row.SubstituteStaff.Person != nil {
		person := row.SubstituteStaff.Person
		result.Target.FullName = person.GetFullName()
	}
	return result
}

func projectAssignment(row *educationModels.GroupSubstitution, group *educationModels.Group, target *userModels.ActiveCaregiver) AssignmentResult {
	period := &Period{StartDate: row.StartDate.String(), EndDate: row.EndDate.String()}
	return AssignmentResult{
		ID: row.ID, Type: TargetGroupHandover, CanEnd: true,
		Period: period,
		Group:  &GroupRef{ID: group.ID, Name: group.Name},
		Target: StaffRef{ID: target.StaffID, FullName: target.FullName()},
	}
}

func auditChange(row *educationModels.GroupSubstitution, actorID int64, action string) *auditModels.SubstitutionChange {
	endDate := auditModels.Date(row.EndDate)
	return &auditModels.SubstitutionChange{SubstitutionID: row.ID, TargetType: string(TargetGroupHandover), Action: action,
		GroupID: row.GroupID, TargetStaffID: row.SubstituteStaffID, ActorAccountID: actorID,
		StartDate: auditModels.Date(row.StartDate), EndDate: &endDate}
}

func contains(ids []int64, id int64) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func containsRole(roles []string, role string) bool {
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}
