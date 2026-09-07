package legacy

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// GroupsByID resolves the education groups a substitution points at. The
// group rows belong to School Structure; the adapter never joins them.
type GroupsByID func(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error)

// SubstitutionStaffResolver attaches RegularStaff and SubstituteStaff to the
// rows through School Membership, which owns users.staff.
type SubstitutionStaffResolver func(ctx context.Context, rows []*educationModels.GroupSubstitution) error

// groupSubstitutionRepository serves
// educationModels.GroupSubstitutionRepository from the Workforce capability.
// Group names come from the injected group lookup, the staff behind a
// substitution from the injected School Membership resolver.
type groupSubstitutionRepository struct {
	workforce workforce.Capability
	groups    GroupsByID
	staff     SubstitutionStaffResolver
}

// NewGroupSubstitutionRepository binds the adapter to the capability and to
// the two owner lookups every relation-loading read needs.
func NewGroupSubstitutionRepository(capability workforce.Capability, groups GroupsByID, staff SubstitutionStaffResolver) educationModels.GroupSubstitutionRepository {
	if capability == nil || groups == nil || staff == nil {
		panic("group substitution repository adapter: Workforce capability, group lookup and staff resolver are required")
	}
	return &groupSubstitutionRepository{workforce: capability, groups: groups, staff: staff}
}

func (r *groupSubstitutionRepository) Create(ctx context.Context, entity *educationModels.GroupSubstitution) error {
	if entity == nil {
		return errors.New("group_substitution cannot be nil or zero value")
	}
	created, err := r.workforce.CreateGroupSubstitution(ctx, substitutionToWorkforce(entity))
	if err != nil {
		return writeError("create", err)
	}
	applySubstitutionToLegacy(entity, created)
	return nil
}

func (r *groupSubstitutionRepository) FindByID(ctx context.Context, id any) (*educationModels.GroupSubstitution, error) {
	substitutionID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindGroupSubstitution(ctx, substitutionID)
	if err != nil {
		return nil, readError("find by id", err, workforce.ErrGroupSubstitutionNotFound)
	}
	return substitutionToLegacy(value), nil
}

func (r *groupSubstitutionRepository) FindByIDForUpdate(ctx context.Context, id any) (*educationModels.GroupSubstitution, error) {
	substitutionID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id for update", Err: err}
	}
	value, err := r.workforce.LockGroupSubstitution(ctx, substitutionID)
	if err != nil {
		return nil, readError("find by id for update", err, workforce.ErrGroupSubstitutionNotFound)
	}
	return substitutionToLegacy(value), nil
}

func (r *groupSubstitutionRepository) Update(ctx context.Context, entity *educationModels.GroupSubstitution) error {
	if entity == nil {
		return errors.New("group_substitution cannot be nil or zero value")
	}
	updated, err := r.workforce.UpdateGroupSubstitution(ctx, substitutionToWorkforce(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrGroupSubstitutionNotFound) {
			return rowsAffectedError("update group_substitution")
		}
		return writeError("update", err)
	}
	applySubstitutionToLegacy(entity, updated)
	return nil
}

func (r *groupSubstitutionRepository) Delete(ctx context.Context, id any) error {
	substitutionID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteGroupSubstitution(ctx, substitutionID); err != nil {
		return writeError("delete", err)
	}
	return nil
}

// List applies the legacy map filters: "active" selects rows covering today,
// "date" rows covering the day, "reason_like" a case-insensitive reason
// match, and any other key an equality on that column.
func (r *groupSubstitutionRepository) List(ctx context.Context, filters map[string]any) ([]*educationModels.GroupSubstitution, error) {
	filter := workforce.GroupSubstitutionFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		switch field {
		case "active":
			if active, ok := value.(bool); ok && active {
				filter.On = timezone.TodayDate().String()
			}
		case "date":
			if date, ok := conditionDate(value); ok {
				filter.On = date
			}
		case "reason_like":
			if text, ok := value.(string); ok {
				filter.ReasonContains = text
			}
		default:
			if err := applyGroupSubstitutionCondition(&filter, modelBase.FilterCondition{Field: field, Operator: modelBase.OpEqual, Value: value}); err != nil {
				return nil, &modelBase.DatabaseError{Op: "list", Err: err}
			}
		}
	}
	return r.list(ctx, "list", filter)
}

func (r *groupSubstitutionRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*educationModels.GroupSubstitution, error) {
	filter, err := groupSubstitutionFilterFromOptions(options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: err}
	}
	return r.list(ctx, "list with options", filter)
}

func (r *groupSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*educationModels.GroupSubstitution, error) {
	return r.list(ctx, "find by group", workforce.GroupSubstitutionFilter{GroupID: groupID})
}

func (r *groupSubstitutionRepository) FindActive(ctx context.Context, date timezone.Date) ([]*educationModels.GroupSubstitution, error) {
	return r.list(ctx, "find active", workforce.GroupSubstitutionFilter{On: date.String()})
}

func (r *groupSubstitutionRepository) FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*educationModels.GroupSubstitution, error) {
	return r.list(ctx, "find active by substitute", workforce.GroupSubstitutionFilter{SubstituteStaffID: substituteStaffID, On: date.String()})
}

func (r *groupSubstitutionRepository) FindOverlapping(ctx context.Context, staffID int64, startDate, endDate timezone.Date) ([]*educationModels.GroupSubstitution, error) {
	return r.list(ctx, "find overlapping", workforce.GroupSubstitutionFilter{
		StaffID: staffID, OverlapFrom: startDate.String(), OverlapTo: endDate.String(),
	})
}

func (r *groupSubstitutionRepository) DeleteActiveOrFutureByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error) {
	deleted, err := r.workforce.DeleteGroupSubstitutionsForStaff(ctx, staffID, from.String())
	if err != nil {
		return 0, writeError("delete active or future by staff id", err)
	}
	return deleted, nil
}

func (r *groupSubstitutionRepository) ListWithRelations(ctx context.Context, options *modelBase.QueryOptions) ([]*educationModels.GroupSubstitution, error) {
	rows, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	return rows, r.attachRelations(ctx, rows)
}

func (r *groupSubstitutionRepository) FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*educationModels.GroupSubstitution, error) {
	rows, err := r.FindActiveBySubstitute(ctx, substituteStaffID, date)
	if err != nil {
		return nil, err
	}
	return rows, r.attachRelations(ctx, rows)
}

// ListActiveSubstitutionBlockers returns the staff member's current or
// upcoming typed group handovers, newest start first, with the group name
// attached; a group that is gone renders as "Unbekannte Gruppe".
func (r *groupSubstitutionRepository) ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerSubstitution, error) {
	rows, err := r.list(ctx, "list active substitution blockers", workforce.GroupSubstitutionFilter{
		SubstituteStaffID: staffID, TargetType: workforce.GroupSubstitutionTypeGroupHandover,
		EndsOnOrAfter: timezone.TodayDate().String(),
	})
	if err != nil {
		return nil, err
	}
	blockers := make([]userModels.BlockerSubstitution, 0, len(rows))
	if len(rows) == 0 {
		return blockers, nil
	}
	groups, err := r.loadGroups(ctx, rows)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.TenantID != tenantID {
			continue
		}
		name := "Unbekannte Gruppe"
		if group, found := groups[row.GroupID]; found && group != nil {
			name = group.Name
		}
		blockers = append(blockers, userModels.BlockerSubstitution{
			ID: row.ID, GroupName: name, StartDate: row.StartDate.String(), EndDate: row.EndDate.String(),
		})
	}
	sort.SliceStable(blockers, func(i, j int) bool { return blockers[i].StartDate > blockers[j].StartDate })
	return blockers, nil
}

func (r *groupSubstitutionRepository) list(ctx context.Context, op string, filter workforce.GroupSubstitutionFilter) ([]*educationModels.GroupSubstitution, error) {
	values, err := r.workforce.ListGroupSubstitutions(ctx, filter)
	if err != nil {
		return nil, readError(op, err, nil)
	}
	result := make([]*educationModels.GroupSubstitution, 0, len(values))
	for _, value := range values {
		result = append(result, substitutionToLegacy(value))
	}
	return result, nil
}

// attachRelations resolves Group through School Structure and the staff
// through School Membership. It fails closed without the staff resolver:
// nameless substitutions would be wrong data rather than an obvious outage.
func (r *groupSubstitutionRepository) attachRelations(ctx context.Context, rows []*educationModels.GroupSubstitution) error {
	if len(rows) == 0 {
		return nil
	}
	groups, err := r.loadGroups(ctx, rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if group, found := groups[row.GroupID]; found {
			row.Group = group
		}
	}
	if r.staff == nil {
		return errors.New("group substitution repository resolves staff through School Membership")
	}
	return r.staff(ctx, rows)
}

func (r *groupSubstitutionRepository) loadGroups(ctx context.Context, rows []*educationModels.GroupSubstitution) (map[int64]*educationModels.Group, error) {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.GroupID > 0 {
			ids = append(ids, row.GroupID)
		}
	}
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return map[int64]*educationModels.Group{}, nil
	}
	groups, err := r.groups(ctx, ids)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "load substitution groups", Err: err}
	}
	return groups, nil
}

// groupSubstitutionFilterFromOptions translates the generic query options the
// retained callers build into the typed capability filter. An unknown
// predicate is an error rather than a silently dropped condition.
func groupSubstitutionFilterFromOptions(options *modelBase.QueryOptions) (workforce.GroupSubstitutionFilter, error) {
	filter := workforce.GroupSubstitutionFilter{}
	if options == nil {
		return filter, nil
	}
	if options.Filter != nil {
		if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
			return filter, errors.New("nested group substitution filters are not supported")
		}
		for _, condition := range options.Filter.Conditions() {
			if err := applyGroupSubstitutionCondition(&filter, condition); err != nil {
				return filter, err
			}
		}
	}
	if options.Sorting != nil && len(options.Sorting.Fields) > 0 {
		return filter, errors.New("group substitution sorting is not supported")
	}
	if options.Pagination != nil && options.Pagination.PageSize > 0 {
		filter.Limit = options.Pagination.PageSize
		if options.Pagination.Page > 1 {
			filter.Offset = (options.Pagination.Page - 1) * options.Pagination.PageSize
		}
	}
	return filter, nil
}

func applyGroupSubstitutionCondition(filter *workforce.GroupSubstitutionFilter, condition modelBase.FilterCondition) error {
	switch {
	case condition.Field == "tenant_id" && condition.Operator == modelBase.OpEqual:
		id, ok := conditionInt64(condition.Value)
		if !ok {
			return errors.New("tenant_id filter must be an integer")
		}
		filter.TenantID = id
	case condition.Field == "group_id" && condition.Operator == modelBase.OpEqual:
		id, ok := conditionInt64(condition.Value)
		if !ok {
			return errors.New("group_id filter must be an integer")
		}
		filter.GroupID = id
	case condition.Field == "group_id" && condition.Operator == modelBase.OpIn:
		ids, ok := conditionInt64List(condition.Value)
		if !ok {
			return errors.New("group_id filter must be a list of integers")
		}
		filter.GroupIDs = ids
	case condition.Field == "substitute_staff_id" && condition.Operator == modelBase.OpEqual:
		id, ok := conditionInt64(condition.Value)
		if !ok {
			return errors.New("substitute_staff_id filter must be an integer")
		}
		filter.SubstituteStaffID = id
	case condition.Field == "regular_staff_id" && condition.Operator == modelBase.OpEqual:
		id, ok := conditionInt64(condition.Value)
		if !ok {
			return errors.New("regular_staff_id filter must be an integer")
		}
		filter.RegularStaffID = id
	case condition.Field == "target_type" && condition.Operator == modelBase.OpEqual:
		targetType, ok := condition.Value.(string)
		if !ok {
			return errors.New("target_type filter must be a string")
		}
		filter.TargetType = targetType
	case condition.Field == "start_date" && condition.Operator == modelBase.OpLessThanOrEqual:
		date, ok := conditionDate(condition.Value)
		if !ok {
			return errors.New("start_date filter must be a calendar date")
		}
		filter.OverlapTo = date
	case condition.Field == "end_date" && condition.Operator == modelBase.OpGreaterThanOrEqual:
		date, ok := conditionDate(condition.Value)
		if !ok {
			return errors.New("end_date filter must be a calendar date")
		}
		filter.OverlapFrom = date
	default:
		return fmt.Errorf("unsupported group substitution filter %s %s", condition.Field, condition.Operator)
	}
	return nil
}

// --- mapping ---

func substitutionToWorkforce(entity *educationModels.GroupSubstitution) workforce.GroupSubstitution {
	return workforce.GroupSubstitution{
		ID: entity.ID, TenantID: entity.TenantID, TargetType: entity.TargetType, GroupID: entity.GroupID,
		RegularStaffID: entity.RegularStaffID, SubstituteStaffID: entity.SubstituteStaffID,
		StartDate: entity.StartDate.String(), EndDate: entity.EndDate.String(), Reason: entity.Reason,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func substitutionToLegacy(value workforce.GroupSubstitution) *educationModels.GroupSubstitution {
	entity := &educationModels.GroupSubstitution{}
	applySubstitutionToLegacy(entity, value)
	return entity
}

func applySubstitutionToLegacy(entity *educationModels.GroupSubstitution, value workforce.GroupSubstitution) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.TargetType = value.TargetType
	entity.GroupID = value.GroupID
	entity.RegularStaffID = value.RegularStaffID
	entity.SubstituteStaffID = value.SubstituteStaffID
	entity.StartDate = timezone.Date(value.StartDate)
	entity.EndDate = timezone.Date(value.EndDate)
	entity.Reason = value.Reason
}
