package api

import (
	"context"
	"fmt"

	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// substitutedStaffQuery answers who substitutes in the given groups on a
// day. education.group_substitution belongs to Workforce (#2688); the root
// supplies the owner's query so the group repository never joins the table.
type substitutedStaffQuery = func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error)

// SupervisionStaffResolver resolves who supervises a group on a day: the
// assigned teachers through their live teacher and staff rows, plus the
// substituting staff that are still live. Offboarded teachers and staff drop
// out, as the replaced inner joins did.
func SupervisionStaffResolver(membership schoolMembershipCompose.StaffLookup, substitutions substitutedStaffQuery) func(context.Context, educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, error) {
	return func(ctx context.Context, raw educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, error) {
		staffByTeacher, err := resolveTeacherStaff(ctx, membership, raw.Assigned)
		if err != nil {
			return nil, err
		}
		substituted, liveSubstitutes, err := liveSubstitutions(ctx, membership, substitutions, raw)
		if err != nil {
			return nil, err
		}
		return supervisionPairs(raw.Assigned, staffByTeacher, substituted, liveSubstitutes), nil
	}
}

// liveSubstitutions loads the substitutions of the groups on the day and the
// substituting staff members that are still live.
func liveSubstitutions(ctx context.Context, membership schoolMembershipCompose.StaffLookup, substitutions substitutedStaffQuery, raw educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, map[int64]schoolmembership.Staff, error) {
	if substitutions == nil {
		return nil, nil, fmt.Errorf("group supervision resolves substitutions through Workforce")
	}
	substituted, err := substitutions(ctx, raw.GroupIDs, raw.On.String())
	if err != nil {
		return nil, nil, fmt.Errorf("load substitutions of group supervision: %w", err)
	}
	substituteIDs := make([]int64, 0, len(substituted))
	for _, pair := range substituted {
		substituteIDs = append(substituteIDs, pair.StaffID)
	}
	live, err := schoolMembershipCompose.StaffByID(ctx, membership, substituteIDs, false)
	if err != nil {
		return nil, nil, err
	}
	return substituted, live, nil
}

// supervisionPairs lists the assignments first, then the substitutions, each
// in query order and without duplicates. An assignment whose teacher has no
// live staff member and a substitute who is gone drop out.
func supervisionPairs(assigned []educationRepo.TeacherGroupID, staffByTeacher map[int64]int64, substituted []educationRepo.StaffGroupID, liveSubstitutes map[int64]schoolmembership.Staff) []educationRepo.StaffGroupID {
	candidates := make([]educationRepo.StaffGroupID, 0, len(assigned)+len(substituted))
	for _, pair := range assigned {
		if staffID, found := staffByTeacher[pair.TeacherID]; found {
			candidates = append(candidates, educationRepo.StaffGroupID{StaffID: staffID, GroupID: pair.GroupID})
		}
	}
	for _, pair := range substituted {
		if _, found := liveSubstitutes[pair.StaffID]; found {
			candidates = append(candidates, pair)
		}
	}
	result := make([]educationRepo.StaffGroupID, 0, len(candidates))
	seen := make(map[educationRepo.StaffGroupID]struct{}, len(candidates))
	for _, pair := range candidates {
		if _, dup := seen[pair]; dup {
			continue
		}
		seen[pair] = struct{}{}
		result = append(result, pair)
	}
	return result
}

// WorkforceSubstitutedStaff adapts the Workforce capability to the
// supervision resolver's substitution query.
func WorkforceSubstitutedStaff(capability workforce.Capability) substitutedStaffQuery {
	if capability == nil {
		return nil
	}
	return func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error) {
		if len(groupIDs) == 0 {
			return []educationRepo.StaffGroupID{}, nil
		}
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

// resolveTeacherStaff maps every live teacher of the assignments to their
// live staff member. A teacher or staff row that is gone yields no entry.
func resolveTeacherStaff(ctx context.Context, membership schoolMembershipCompose.StaffLookup, assigned []educationRepo.TeacherGroupID) (map[int64]int64, error) {
	result := make(map[int64]int64, len(assigned))
	ids := make([]int64, 0, len(assigned))
	for _, pair := range assigned {
		ids = append(ids, pair.TeacherID)
	}
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	teachers, err := membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("load teachers of group assignments: %w", err)
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	live, err := schoolMembershipCompose.StaffByID(ctx, membership, staffIDs, false)
	if err != nil {
		return nil, err
	}
	for _, teacher := range teachers {
		if _, found := live[teacher.StaffID]; found {
			result[teacher.ID] = teacher.StaffID
		}
	}
	return result, nil
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
		return &userModels.DatabaseError{Op: "load substitution groups", Err: err}
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
	// ListActiveSubstitutionBlockers returns current/upcoming typed group
	// handovers as caregiver-capability blocker rows.
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerSubstitution, error)
	ListWithFilter(ctx context.Context, filter workforce.GroupSubstitutionFilter) ([]*workforceCompose.GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*workforceCompose.GroupSubstitution, error)
}

// EducationHandovers serves the substitution module's handover store from the
// retained group substitution repository. Workforce owns the rows; the
// binding translates them into the School Structure values (#3556).
type EducationHandovers struct {
	substitutions GroupSubstitutionRepository
}

// NewEducationHandovers binds the handover store to the group substitution
// repository.
func NewEducationHandovers(substitutions GroupSubstitutionRepository) EducationHandovers {
	return EducationHandovers{substitutions: substitutions}
}

// Create stores a handover and reports educationRepo.ErrHandoverExists for
// a duplicate.
func (h EducationHandovers) Create(ctx context.Context, handover *educationRepo.GroupSubstitution) error {
	row := workforceSubstitution(handover)
	if err := h.substitutions.Create(ctx, row); err != nil {
		if userModels.IsUniqueViolation(err) {
			return educationRepo.ErrHandoverExists
		}
		return err
	}
	stored := educationHandover(row)
	stored.Group, stored.RegularStaff, stored.SubstituteStaff = handover.Group, handover.RegularStaff, handover.SubstituteStaff
	*handover = *stored
	return nil
}

// Delete removes a handover.
func (h EducationHandovers) Delete(ctx context.Context, id any) error {
	return h.substitutions.Delete(ctx, id)
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

func workforceSubstitution(handover *educationRepo.GroupSubstitution) *workforceCompose.GroupSubstitution {
	if handover == nil {
		return nil
	}
	return &workforceCompose.GroupSubstitution{
		ID: handover.ID, TenantID: handover.TenantID, CreatedAt: handover.CreatedAt, UpdatedAt: handover.UpdatedAt,
		TargetType: handover.TargetType, GroupID: handover.GroupID, RegularStaffID: handover.RegularStaffID,
		SubstituteStaffID: handover.SubstituteStaffID, StartDate: handover.StartDate, EndDate: handover.EndDate, Reason: handover.Reason,
	}
}

// handoverFilter selects the handovers of one school in the Workforce
// capability's filter: a start on or before and an end on or after the
// bounds overlap the period.
func handoverFilter(query educationRepo.HandoverQuery) workforce.GroupSubstitutionFilter {
	filter := workforce.GroupSubstitutionFilter{
		TenantID: query.TenantID, TargetType: query.TargetType, GroupID: query.GroupID,
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
