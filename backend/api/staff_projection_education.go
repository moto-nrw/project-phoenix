package api

import (
	"context"
	"errors"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
)

// The owner reads behind School Structure's rule of who supervises a group
// on a day (#2667, #2688): the teacher and staff rows come from School
// Membership, the substitutions from Workforce. The rule itself is School
// Structure's; these bindings only translate.

// supervisionTeacherStaff maps every given live teacher to its staff member.
func supervisionTeacherStaff(membership schoolMembershipCompose.StaffLookup) func(context.Context, []int64) (map[int64]int64, error) {
	return func(ctx context.Context, teacherIDs []int64) (map[int64]int64, error) {
		teachers, err := membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: teacherIDs})
		if err != nil {
			return nil, err
		}
		result := make(map[int64]int64, len(teachers))
		for _, teacher := range teachers {
			result[teacher.ID] = teacher.StaffID
		}
		return result, nil
	}
}

// supervisionLiveStaff reports the given staff members that are still live.
func supervisionLiveStaff(membership schoolMembershipCompose.StaffLookup) func(context.Context, []int64) (map[int64]bool, error) {
	return func(ctx context.Context, staffIDs []int64) (map[int64]bool, error) {
		live, err := schoolMembershipCompose.StaffByID(ctx, membership, staffIDs, false)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]bool, len(live))
		for id := range live {
			result[id] = true
		}
		return result, nil
	}
}

// supervisionSubstitutes lists who substitutes in the groups on the day.
func supervisionSubstitutes(capability workforce.Capability) func(context.Context, []int64, string) ([]educationRepo.StaffGroupID, error) {
	return func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error) {
		rows, err := capability.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{GroupIDs: groupIDs, On: on})
		if err != nil {
			return nil, err
		}
		pairs := make([]educationRepo.StaffGroupID, 0, len(rows))
		for _, row := range rows {
			pairs = append(pairs, educationRepo.StaffGroupID{StaffID: row.SubstituteStaffID, GroupID: row.GroupID})
		}
		return pairs, nil
	}
}

// groupSubstitutions serves the retained group substitution contract over
// the Workforce rows. The reads with relations attach the group through
// School Structure and the staff members through School Membership; the
// Workforce adapter names neither (#3556).
type groupSubstitutions struct {
	GroupSubstitutionRows
	groups     func(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
	membership schoolMembershipCompose.StaffLookup
}

func newGroupSubstitutions(rows GroupSubstitutionRows, groups func(context.Context, []int64) (map[int64]*educationRepo.Group, error), membership schoolMembershipCompose.StaffLookup) groupSubstitutions {
	return groupSubstitutions{GroupSubstitutionRows: rows, groups: groups, membership: membership}
}

// ListActiveSubstitutionBlockers translates the Workforce handover blockers
// into the People Directory's caregiver-capability blocker rows.
func (r groupSubstitutions) ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]peopleValues.BlockerSubstitution, error) {
	blockers, err := r.ActiveSubstitutionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]peopleValues.BlockerSubstitution, 0, len(blockers))
	for _, blocker := range blockers {
		result = append(result, peopleValues.BlockerSubstitution{
			ID: blocker.ID, GroupName: blocker.GroupName, StartDate: blocker.StartDate, EndDate: blocker.EndDate,
		})
	}
	return result, nil
}

// FindGroupHandovers lists the substitutions of one group as School
// Structure values, the group service's deletion guard.
func (r groupSubstitutions) FindGroupHandovers(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error) {
	return educationHandovers(r.FindByGroup(ctx, groupID))
}

// ListWithRelations resolves the groups and then the staff members. Soft-
// deleted staff are included so historical substitutions keep resolving
// after offboarding, as the replaced WhereAllWithDeleted lookup did.
func (r groupSubstitutions) ListWithRelations(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	rows, err := educationHandovers(r.ListWithFilter(ctx, handoverFilter(query)))
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	if err := r.attachGroups(ctx, rows); err != nil {
		return nil, err
	}
	if err := r.attachStaff(ctx, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// attachGroups resolves the group of every row through School Structure.
func (r groupSubstitutions) attachGroups(ctx context.Context, rows []*educationRepo.GroupSubstitution) error {
	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.GroupID > 0 {
			groupIDs = append(groupIDs, row.GroupID)
		}
	}
	if groupIDs = uniqueIDs(groupIDs); len(groupIDs) == 0 {
		return nil
	}
	groups, err := r.groups(ctx, groupIDs)
	if err != nil {
		return &peoplePorts.DatabaseError{Op: "load substitution groups", Err: err}
	}
	for _, row := range rows {
		row.Group = groups[row.GroupID]
	}
	return nil
}

// attachStaff resolves the substituting and the regular staff member of
// every row through School Membership.
func (r groupSubstitutions) attachStaff(ctx context.Context, rows []*educationRepo.GroupSubstitution) error {
	staffIDs := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		staffIDs = append(staffIDs, row.SubstituteStaffID)
		if row.RegularStaffID != nil {
			staffIDs = append(staffIDs, *row.RegularStaffID)
		}
	}
	members, err := schoolMembershipCompose.StaffByID(ctx, r.membership, staffIDs, true)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if member, found := members[row.SubstituteStaffID]; found {
			row.SubstituteStaff = substitutionStaff(member)
		}
		if row.RegularStaffID == nil {
			continue
		}
		if member, found := members[*row.RegularStaffID]; found {
			row.RegularStaff = substitutionStaff(member)
		}
	}
	return nil
}

// substitutionStaff is the staff reference a substitution row carries; the
// name follows from the People Directory where a reader needs it.
func substitutionStaff(value schoolmembership.Staff) *educationRepo.SubstitutionStaff {
	return &educationRepo.SubstitutionStaff{ID: value.ID, PersonID: value.PersonID}
}

func uniqueIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

// GroupSubstitutionRow is the Workforce row the group substitution contract
// reads and writes.
type GroupSubstitutionRow = workforceCompose.GroupSubstitution

// GroupSubstitutionRepository is the retained contract of
// education.group_substitution: the Workforce rows, and the reads with
// relations as School Structure values.
type GroupSubstitutionRepository interface {
	GroupSubstitutionRows
	// FindGroupHandovers lists the substitutions of one group as School
	// Structure values, the group service's deletion guard.
	FindGroupHandovers(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error)
	// ListWithRelations is ListWithOptions with the group and staff attached.
	ListWithRelations(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error)
	// ListActiveSubstitutionBlockers returns current/upcoming typed group
	// handovers as caregiver-capability blocker rows.
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]peopleValues.BlockerSubstitution, error)
}

// GroupSubstitutionRows are the Workforce rows of the contract, served over
// the Workforce capability.
type GroupSubstitutionRows interface {
	Create(ctx context.Context, substitution *workforceCompose.GroupSubstitution) error
	FindByID(ctx context.Context, id any) (*workforceCompose.GroupSubstitution, error)
	Update(ctx context.Context, substitution *workforceCompose.GroupSubstitution) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*workforceCompose.GroupSubstitution, error)
	FindByIDForUpdate(ctx context.Context, id any) (*workforceCompose.GroupSubstitution, error)
	// ActiveSubstitutionBlockers returns current/upcoming typed group
	// handovers with their group named.
	ActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]workforceCompose.SubstitutionBlocker, error)
	ListWithFilter(ctx context.Context, filter workforce.GroupSubstitutionFilter) ([]*workforceCompose.GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*workforceCompose.GroupSubstitution, error)
}

// EducationHandovers serves the substitution module's handover store: the
// shift-plan-sync workflow writes the handovers into Workforce's rows, the
// reads come from the retained group substitution repository. The binding
// translates the rows into the School Structure values (#3556).
type EducationHandovers struct {
	substitutions GroupSubstitutionRepository
	handovers     shiftplansync.GroupHandovers
}

// NewEducationHandovers binds the handover store to the group substitution
// reads and the handover writes.
func NewEducationHandovers(substitutions GroupSubstitutionRepository, handovers shiftplansync.GroupHandovers) EducationHandovers {
	return EducationHandovers{substitutions: substitutions, handovers: handovers}
}

// Create stores a handover and reports educationRepo.ErrHandoverExists for
// a duplicate.
func (h EducationHandovers) Create(ctx context.Context, handover *educationRepo.GroupSubstitution) error {
	if handover == nil {
		return errors.New("group_substitution cannot be nil or zero value")
	}
	created, err := h.handovers.CreateHandover(ctx, workforceHandover(handover))
	if errors.Is(err, shiftplansync.ErrHandoverExists) {
		return educationRepo.ErrHandoverExists
	}
	if err != nil {
		return err
	}
	stored := educationHandoverFromWorkforce(created)
	stored.Group, stored.RegularStaff, stored.SubstituteStaff = handover.Group, handover.RegularStaff, handover.SubstituteStaff
	*handover = *stored
	return nil
}

// Delete removes a handover.
func (h EducationHandovers) Delete(ctx context.Context, id any) error {
	handoverID, err := membershipID(id)
	if err != nil {
		return err
	}
	return h.handovers.DeleteHandover(ctx, handoverID)
}

// FindByID reads one handover.
func (h EducationHandovers) FindByID(ctx context.Context, id any) (*educationRepo.GroupSubstitution, error) {
	return educationHandoverResult(h.substitutions.FindByID(ctx, id))
}

// FindByIDForUpdate reads one handover under a row lock.
func (h EducationHandovers) FindByIDForUpdate(ctx context.Context, id any) (*educationRepo.GroupSubstitution, error) {
	return educationHandoverResult(h.substitutions.FindByIDForUpdate(ctx, id))
}

// ListHandovers selects the handovers of one school.
func (h EducationHandovers) ListHandovers(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return educationHandovers(h.substitutions.ListWithFilter(ctx, handoverFilter(query)))
}

// ListHandoversWithRelations is ListHandovers with the group and the staff
// members attached.
func (h EducationHandovers) ListHandoversWithRelations(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return h.substitutions.ListWithRelations(ctx, query)
}

func educationHandoverResult(row *workforceCompose.GroupSubstitution, err error) (*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	return educationHandover(row), nil
}

func educationHandovers(rows []*workforceCompose.GroupSubstitution, err error) ([]*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	result := make([]*educationRepo.GroupSubstitution, 0, len(rows))
	for _, row := range rows {
		result = append(result, educationHandover(row))
	}
	return result, nil
}

func educationHandover(row *workforceCompose.GroupSubstitution) *educationRepo.GroupSubstitution {
	if row == nil {
		return nil
	}
	return &educationRepo.GroupSubstitution{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		TargetType: row.TargetType, GroupID: row.GroupID, RegularStaffID: row.RegularStaffID,
		SubstituteStaffID: row.SubstituteStaffID, StartDate: row.StartDate, EndDate: row.EndDate, Reason: row.Reason,
	}
}

// workforceHandover is the Workforce value of a School Structure handover.
func workforceHandover(handover *educationRepo.GroupSubstitution) workforce.GroupSubstitution {
	return workforce.GroupSubstitution{
		ID: handover.ID, TenantID: handover.TenantID, TargetType: handover.TargetType, GroupID: handover.GroupID,
		RegularStaffID: handover.RegularStaffID, SubstituteStaffID: handover.SubstituteStaffID,
		StartDate: handover.StartDate.String(), EndDate: handover.EndDate.String(), Reason: handover.Reason,
		CreatedAt: handover.CreatedAt, UpdatedAt: handover.UpdatedAt,
	}
}

// educationHandoverFromWorkforce is the School Structure value of a stored
// Workforce handover.
func educationHandoverFromWorkforce(value workforce.GroupSubstitution) *educationRepo.GroupSubstitution {
	return &educationRepo.GroupSubstitution{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		TargetType: value.TargetType, GroupID: value.GroupID, RegularStaffID: value.RegularStaffID,
		SubstituteStaffID: value.SubstituteStaffID, StartDate: timezone.Date(value.StartDate), EndDate: timezone.Date(value.EndDate), Reason: value.Reason,
	}
}

// handoverFilter selects the handovers of one school in the Workforce
// capability's filter: a start on or before and an end on or after the
// bounds overlap the period.
func handoverFilter(query educationRepo.HandoverQuery) workforce.GroupSubstitutionFilter {
	filter := workforce.GroupSubstitutionFilter{
		TargetType: query.TargetType, GroupID: query.GroupID,
		SubstituteStaffID: query.SubstituteStaffID,
	}
	if query.GroupIDs != nil {
		filter.GroupIDs = append([]int64{}, query.GroupIDs...)
	}
	if query.StartsOnOrBefore != nil {
		filter.OverlapTo = query.StartsOnOrBefore.String()
	}
	if query.EndsOnOrAfter != nil {
		filter.OverlapFrom = query.EndsOnOrAfter.String()
	}
	if query.TenantID != 0 {
		filter.TenantID = &query.TenantID
	}
	return filter
}

// NewGroupSubstitutionRepository serves the retained group substitution
// contract over Workforce (#2688): the rows are Workforce's, the groups
// resolve through the group store and the staff through School Membership.
func NewGroupSubstitutionRepository(groups EducationGroupRepository, membership schoolMembershipCompose.StaffLookup, workTime workforce.Capability) GroupSubstitutionRepository {
	return newGroupSubstitutions(
		workforceCompose.NewGroupSubstitutionRepository(workTime, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return educationRepo.NewGroupNames(groups).GroupNamesByID(ctx, ids)
		}),
		groups.FindByIDs,
		membership,
	)
}
