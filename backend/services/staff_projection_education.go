package services

import (
	"context"
	"fmt"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
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
		if substitutions == nil {
			return nil, fmt.Errorf("group supervision resolves substitutions through Workforce")
		}
		substituted, err := substitutions(ctx, raw.GroupIDs, raw.On.String())
		if err != nil {
			return nil, fmt.Errorf("load substitutions of group supervision: %w", err)
		}
		substituteIDs := make([]int64, 0, len(substituted))
		for _, pair := range substituted {
			substituteIDs = append(substituteIDs, pair.StaffID)
		}
		liveSubstitutes, err := schoolMembershipCompose.StaffByID(ctx, membership, substituteIDs, false)
		if err != nil {
			return nil, err
		}

		// Assignments first, then substitutions, each in query order.
		result := make([]educationRepo.StaffGroupID, 0, len(raw.Assigned)+len(substituted))
		seen := make(map[educationRepo.StaffGroupID]struct{}, cap(result))
		appendPair := func(pair educationRepo.StaffGroupID) {
			if _, dup := seen[pair]; dup {
				return
			}
			seen[pair] = struct{}{}
			result = append(result, pair)
		}
		for _, pair := range raw.Assigned {
			staffID, found := staffByTeacher[pair.TeacherID]
			if !found {
				continue
			}
			appendPair(educationRepo.StaffGroupID{StaffID: staffID, GroupID: pair.GroupID})
		}
		for _, pair := range substituted {
			if _, found := liveSubstitutes[pair.StaffID]; !found {
				continue
			}
			appendPair(pair)
		}
		return result, nil
	}
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
func (r groupSubstitutions) ListWithRelations(ctx context.Context, options *userModels.QueryOptions) ([]*educationRepo.GroupSubstitution, error) {
	rows, err := educationHandovers(r.ListWithOptions(ctx, options))
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	groupIDs := make([]int64, 0, len(rows))
	staffIDs := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		if row.GroupID > 0 {
			groupIDs = append(groupIDs, row.GroupID)
		}
		staffIDs = append(staffIDs, row.SubstituteStaffID)
		if row.RegularStaffID != nil {
			staffIDs = append(staffIDs, *row.RegularStaffID)
		}
	}
	if groupIDs = uniqueIDs(groupIDs); len(groupIDs) > 0 {
		groups, err := r.groups(ctx, groupIDs)
		if err != nil {
			return nil, &userModels.DatabaseError{Op: "load substitution groups", Err: err}
		}
		for _, row := range rows {
			row.Group = groups[row.GroupID]
		}
	}
	members, err := schoolMembershipCompose.StaffByID(ctx, r.membership, staffIDs, true)
	if err != nil {
		return nil, err
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
	return rows, nil
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
type GroupSubstitutionRow = workforceLegacy.GroupSubstitution

// GroupSubstitutionRepository is the retained contract of
// education.group_substitution: the Workforce rows, and the reads with
// relations as School Structure values.
type GroupSubstitutionRepository interface {
	GroupSubstitutionRows
	// FindGroupHandovers lists the substitutions of one group as School
	// Structure values, the group service's deletion guard.
	FindGroupHandovers(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error)
	// ListWithRelations is ListWithOptions with the group and staff attached.
	ListWithRelations(ctx context.Context, options *userModels.QueryOptions) ([]*educationRepo.GroupSubstitution, error)
}

// GroupSubstitutionRows are the Workforce rows of the contract, served over
// the Workforce capability.
type GroupSubstitutionRows interface {
	Create(ctx context.Context, substitution *workforceLegacy.GroupSubstitution) error
	FindByID(ctx context.Context, id any) (*workforceLegacy.GroupSubstitution, error)
	Update(ctx context.Context, substitution *workforceLegacy.GroupSubstitution) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*workforceLegacy.GroupSubstitution, error)
	FindByIDForUpdate(ctx context.Context, id any) (*workforceLegacy.GroupSubstitution, error)
	// ListActiveSubstitutionBlockers returns current/upcoming typed group
	// handovers as caregiver-capability blocker rows.
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerSubstitution, error)
	ListWithOptions(ctx context.Context, options *userModels.QueryOptions) ([]*workforceLegacy.GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*workforceLegacy.GroupSubstitution, error)
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
	return educationHandovers(h.substitutions.ListWithOptions(ctx, handoverQueryOptions(query)))
}

// ListHandoversWithRelations is ListHandovers with the group and the staff
// members attached.
func (h EducationHandovers) ListHandoversWithRelations(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return h.substitutions.ListWithRelations(ctx, handoverQueryOptions(query))
}

func educationHandoverResult(row *workforceLegacy.GroupSubstitution, err error) (*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	return educationHandover(row), nil
}

func educationHandovers(rows []*workforceLegacy.GroupSubstitution, err error) ([]*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	result := make([]*educationRepo.GroupSubstitution, 0, len(rows))
	for _, row := range rows {
		result = append(result, educationHandover(row))
	}
	return result, nil
}

func educationHandover(row *workforceLegacy.GroupSubstitution) *educationRepo.GroupSubstitution {
	if row == nil {
		return nil
	}
	return &educationRepo.GroupSubstitution{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		TargetType: row.TargetType, GroupID: row.GroupID, RegularStaffID: row.RegularStaffID,
		SubstituteStaffID: row.SubstituteStaffID, StartDate: row.StartDate, EndDate: row.EndDate, Reason: row.Reason,
	}
}

func workforceSubstitution(handover *educationRepo.GroupSubstitution) *workforceLegacy.GroupSubstitution {
	if handover == nil {
		return nil
	}
	return &workforceLegacy.GroupSubstitution{
		ID: handover.ID, TenantID: handover.TenantID, CreatedAt: handover.CreatedAt, UpdatedAt: handover.UpdatedAt,
		TargetType: handover.TargetType, GroupID: handover.GroupID, RegularStaffID: handover.RegularStaffID,
		SubstituteStaffID: handover.SubstituteStaffID, StartDate: handover.StartDate, EndDate: handover.EndDate, Reason: handover.Reason,
	}
}

func handoverQueryOptions(query educationRepo.HandoverQuery) *userModels.QueryOptions {
	filter := userModels.NewQueryFilter().Equal("tenant_id", query.TenantID)
	if query.TargetType != "" {
		filter.Equal("target_type", query.TargetType)
	}
	if query.GroupID > 0 {
		filter.Equal("group_id", query.GroupID)
	}
	if query.GroupIDs != nil {
		values := make([]any, len(query.GroupIDs))
		for i, id := range query.GroupIDs {
			values[i] = id
		}
		filter.In("group_id", values...)
	}
	if query.SubstituteStaffID > 0 {
		filter.Equal("substitute_staff_id", query.SubstituteStaffID)
	}
	if query.StartsOnOrBefore != nil {
		filter.LessThanOrEqual("start_date", *query.StartsOnOrBefore)
	}
	if query.EndsOnOrAfter != nil {
		filter.GreaterThanOrEqual("end_date", *query.EndsOnOrAfter)
	}
	options := userModels.NewQueryOptions()
	options.Filter = filter
	return options
}

// NewGroupSubstitutionRepository serves the retained group substitution
// contract over Workforce (#2688): the rows are Workforce's, the groups
// resolve through the group store and the staff through School Membership.
func NewGroupSubstitutionRepository(groups EducationGroupRepository, membership schoolMembershipCompose.StaffLookup, workTime workforce.Capability) GroupSubstitutionRepository {
	return newGroupSubstitutions(
		workforceLegacy.NewGroupSubstitutionRepository(workTime, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return educationRepo.NewGroupNames(groups).GroupNamesByID(ctx, ids)
		}),
		groups.FindByIDs,
		membership,
	)
}
