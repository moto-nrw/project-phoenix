package repositories

import (
	"context"
	"errors"
	"fmt"

	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// timetableActivityGroupRepository keeps the legacy repository contract while
// routing group lifecycle and target persistence through the Timetable owner.
type timetableActivityGroupRepository struct {
	activityGroupTargets
	timetable timetable.Capability
	groups    schoolstructure.Query
}

func (r timetableActivityGroupRepository) Create(ctx context.Context, group *activitiesModels.Group) error {
	if group == nil {
		return errors.New("group cannot be nil or zero value")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateGroup(ctx, publicGroupInput(group))
	if err != nil {
		return fmt.Errorf("database error during create: %w", err)
	}
	*group = *legacyGroup(created)
	return nil
}

func (r timetableActivityGroupRepository) Update(ctx context.Context, group *activitiesModels.Group) error {
	if group == nil {
		return errors.New("group cannot be nil")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	_, err := r.timetable.UpdateGroup(ctx, group.ID, publicGroupInput(group))
	if err != nil {
		return legacyGroupReadError("update", err)
	}
	return nil
}

func (r timetableActivityGroupRepository) Delete(ctx context.Context, id any) error {
	groupID, ok := legacyGroupID(id)
	if !ok {
		return fmt.Errorf("database error during delete: invalid activity group id %T", id)
	}
	if err := r.timetable.DeleteGroup(ctx, groupID); err != nil {
		return fmt.Errorf("database error during delete: %w", err)
	}
	return nil
}

func (r timetableActivityGroupRepository) FindByID(ctx context.Context, id any) (*activitiesModels.Group, error) {
	groupID, ok := legacyGroupID(id)
	if !ok {
		return nil, fmt.Errorf("database error during find by id: invalid activity group id %T", id)
	}
	group, err := r.timetable.FindGroup(ctx, groupID)
	if err != nil {
		return nil, legacyGroupReadError("find by id", err)
	}
	return legacyGroup(group), nil
}

func (r timetableActivityGroupRepository) FindByIDForUpdate(ctx context.Context, id any) (*activitiesModels.Group, error) {
	groupID, ok := legacyGroupID(id)
	if !ok {
		return nil, fmt.Errorf("database error during find by id for update: invalid activity group id %T", id)
	}
	group, err := r.timetable.FindGroupForUpdate(ctx, groupID)
	if err != nil {
		return nil, legacyGroupReadError("find by id for update", err)
	}
	return legacyGroup(group), nil
}

func (r timetableActivityGroupRepository) FindByName(ctx context.Context, name string) (*activitiesModels.Group, error) {
	group, err := r.timetable.FindGroupByName(ctx, name)
	if err != nil {
		return nil, legacyGroupReadError("find group by name", err)
	}
	return legacyGroup(group), nil
}

func (r timetableActivityGroupRepository) FindByIDs(ctx context.Context, ids []int64) ([]*activitiesModels.Group, error) {
	if len(ids) == 0 {
		return []*activitiesModels.Group{}, nil
	}
	groups, err := r.timetable.ListGroups(ctx, timetable.GroupFilter{IDs: ids})
	if err != nil {
		return nil, legacyGroupReadError("find groups by ids", err)
	}
	return legacyGroups(groups), nil
}

func (r timetableActivityGroupRepository) ListWithCategory(ctx context.Context, query *activitiesModels.GroupListQuery) ([]*activitiesModels.Group, error) {
	filter := timetable.GroupFilter{}
	if query != nil {
		filter = timetable.GroupFilter{Name: query.Name, CategoryID: query.CategoryID, IsSystem: query.IsSystem, IDs: query.IDs}
	}
	groups, err := r.timetable.ListGroups(ctx, filter)
	if err != nil {
		return nil, legacyGroupReadError("list", err)
	}
	return legacyGroups(groups), nil
}

func (r timetableActivityGroupRepository) FindOpenGroups(ctx context.Context) ([]*activitiesModels.Group, error) {
	open, system := true, false
	return r.listLegacyGroups(ctx, timetable.GroupFilter{IsOpen: &open, IsSystem: &system, OrderByName: true}, "find open groups")
}

func (r timetableActivityGroupRepository) FindAllTemplates(ctx context.Context) ([]*activitiesModels.Group, error) {
	isTemplate := true
	return r.listLegacyGroups(ctx, timetable.GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, OrderByName: true}, "find all templates")
}

func (r timetableActivityGroupRepository) FindTemplateSeries(ctx context.Context, groupID int64) ([]*activitiesModels.Group, error) {
	isTemplate := true
	return r.listLegacyGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, ActiveOnly: true, SeriesForGroupID: &groupID, OrderByID: true,
	}, "find template series")
}

func (r timetableActivityGroupRepository) FindTemplatesBySourceOffering(ctx context.Context, offeringID int64) ([]*activitiesModels.Group, error) {
	isTemplate := true
	return r.listLegacyGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, ActiveOnly: true, SourceOfferingIDs: []int64{offeringID}, OrderByID: true,
	}, "find templates by source offering")
}

func (r timetableActivityGroupRepository) FindTemplatesBySourceOfferings(ctx context.Context, offeringIDs []int64) ([]*activitiesModels.Group, error) {
	if len(offeringIDs) == 0 {
		return []*activitiesModels.Group{}, nil
	}
	isTemplate := true
	return r.listLegacyGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, ActiveOnly: true, SourceOfferingIDs: offeringIDs, OrderByID: true,
	}, "find templates by source offerings")
}

func (r timetableActivityGroupRepository) FindTemplatesWithOfferingSource(ctx context.Context) ([]*activitiesModels.Group, error) {
	isTemplate := true
	return r.listLegacyGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, ActiveOnly: true, HasOfferingSource: true, OrderByID: true,
	}, "find templates with offering source")
}

func (r timetableActivityGroupRepository) UpdateTemplateFields(ctx context.Context, id int64, fields activitiesModels.TemplateFieldsUpdate) (int64, error) {
	return r.timetable.UpdateTemplate(ctx, id, timetable.TemplateUpdate{
		Name: fields.Name, Type: fields.Type, CategoryID: fields.CategoryID,
		PlanningTrackID: fields.PlanningTrackID, PlanningTrackIDProvided: fields.PlanningTrackIDProvided,
		RoomID: fields.RoomID, EducationGroupID: fields.EducationGroupID,
		MaxParticipants: fields.MaxParticipants, MaxParticipantsProvided: fields.MaxParticipantsProvided,
		RequiredStaff: fields.RequiredStaff, CalendarPeriodID: fields.CalendarPeriodID,
		TargetGroupType: fields.TargetGroupType, TargetGradeLevel: fields.TargetGradeLevel,
		TargetSchoolClass: fields.TargetSchoolClass, ListKind: fields.ListKind, Notes: fields.Notes,
		SourceCareOfferingIDs: fields.SourceCareOfferingIDs, SourceGradeLevels: fields.SourceGradeLevels,
		SourceSchoolClasses: fields.SourceSchoolClasses,
	})
}

func (r timetableActivityGroupRepository) ArchiveTemplate(ctx context.Context, id int64) (int64, error) {
	return r.timetable.ArchiveTemplate(ctx, id)
}

func (r timetableActivityGroupRepository) UpdateTemplateOfferingSource(ctx context.Context, id int64, offeringIDs []int64, gradeLevels []int, schoolClasses []string) error {
	return r.timetable.UpdateGroupOfferingSource(ctx, id, timetable.OfferingSourceInput{
		CareOfferingIDs: offeringIDs, GradeLevels: gradeLevels, SchoolClasses: schoolClasses,
	})
}

func (r timetableActivityGroupRepository) listLegacyGroups(ctx context.Context, filter timetable.GroupFilter, operation string) ([]*activitiesModels.Group, error) {
	groups, err := r.timetable.ListGroups(ctx, filter)
	if err != nil {
		return nil, legacyGroupReadError(operation, err)
	}
	return legacyGroups(groups), nil
}

func (r timetableActivityGroupRepository) ReplaceTargets(ctx context.Context, groupID int64, targets []*activitiesModels.GroupTarget) error {
	if groupID <= 0 {
		return errors.New("replace group targets requires a positive activity group id")
	}
	inputs := make([]timetable.GroupTargetInput, 0, len(targets))
	var targetType string
	for _, target := range targets {
		if target == nil {
			return errors.New("invalid group target: target cannot be null")
		}
		copy := *target
		if err := copy.Validate(); err != nil {
			return fmt.Errorf("invalid group target: %w", err)
		}
		if targetType != "" && targetType != copy.TargetGroupType {
			return errors.New("invalid group targets: all targets must have the same type")
		}
		targetType = copy.TargetGroupType
		inputs = append(inputs, timetable.GroupTargetInput{
			TargetGroupType: copy.TargetGroupType, TargetGradeLevel: copy.TargetGradeLevel,
			TargetSchoolClass: copy.TargetSchoolClass, EducationGroupID: copy.EducationGroupID,
		})
	}
	return r.timetable.ReplaceGroupTargets(ctx, groupID, inputs)
}

func (r timetableActivityGroupRepository) FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activitiesModels.GroupTarget, error) {
	values, err := r.timetable.ListGroupTargets(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]*activitiesModels.GroupTarget, len(values))
	all := make([]*activitiesModels.GroupTarget, 0)
	for groupID, targets := range values {
		for _, target := range targets {
			mapped := legacyGroupTarget(target)
			result[groupID] = append(result[groupID], mapped)
			all = append(all, mapped)
		}
	}
	if r.groups == nil {
		return result, nil
	}
	_, err = enrichGroupNames(ctx, r.groups, all,
		func(row *activitiesModels.GroupTarget) int64 { return optionalGroupID(row.EducationGroupID) },
		func(row *activitiesModels.GroupTarget, name string) { row.EducationGroupName = name },
		"", "template targets")
	return result, err
}

func (r timetableActivityGroupRepository) FindTargetStudentIDs(ctx context.Context, groupID int64) ([]int64, error) {
	byGroup, err := r.FindTargetStudentIDsByGroupIDs(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	return byGroup[groupID], nil
}

func (r timetableActivityGroupRepository) FindTargetStudentIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error) {
	return r.timetable.ListTargetStudentIDs(ctx, groupIDs)
}

func legacyGroupTarget(target timetable.GroupTarget) *activitiesModels.GroupTarget {
	result := &activitiesModels.GroupTarget{
		ActivityGroupID: target.ActivityGroupID,
		TargetGroupType: target.TargetGroupType, TargetGradeLevel: target.TargetGradeLevel,
		TargetSchoolClass: target.TargetSchoolClass, EducationGroupID: target.EducationGroupID,
		EducationGroupName: target.EducationGroupName,
	}
	result.ID = target.ID
	result.CreatedAt = target.CreatedAt
	result.UpdatedAt = target.UpdatedAt
	result.SetTenantID(target.TenantID)
	return result
}

func legacyGroups(groups []timetable.Group) []*activitiesModels.Group {
	result := make([]*activitiesModels.Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, legacyGroup(group))
	}
	return result
}

func legacyGroup(group timetable.Group) *activitiesModels.Group {
	result := &activitiesModels.Group{
		Name: group.Name, MaxParticipants: group.MaxParticipants, RequiredStaff: group.RequiredStaff,
		IsOpen: group.IsOpen, CategoryID: group.CategoryID, PlanningTrackID: group.PlanningTrackID,
		PlannedRoomID: group.PlannedRoomID, CreatedBy: group.CreatedBy, Type: group.Type,
		EducationGroupID: group.EducationGroupID, ListKind: group.ListKind, IsTemplate: group.IsTemplate,
		IsSystem: group.IsSystem, ArchivedAt: group.ArchivedAt, SeriesRootID: group.SeriesRootID,
		CalendarPeriodID: group.CalendarPeriodID, TargetGroupType: group.TargetGroupType,
		TargetGradeLevel: group.TargetGradeLevel, TargetSchoolClass: group.TargetSchoolClass,
		SourceCareOfferingIDs: group.SourceCareOfferingIDs, SourceGradeLevels: group.SourceGradeLevels,
		SourceSchoolClasses: group.SourceSchoolClasses, Notes: group.Notes,
	}
	result.ID = group.ID
	result.CreatedAt = group.CreatedAt
	result.UpdatedAt = group.UpdatedAt
	result.SetTenantID(group.TenantID)
	if group.Category != nil {
		result.Category = legacyCategory(*group.Category)
	}
	return result
}

func publicGroupInput(group *activitiesModels.Group) timetable.GroupInput {
	return timetable.GroupInput{
		Name: group.Name, MaxParticipants: group.MaxParticipants, RequiredStaff: group.RequiredStaff, IsOpen: group.IsOpen,
		CategoryID: group.CategoryID, PlanningTrackID: group.PlanningTrackID, PlannedRoomID: group.PlannedRoomID,
		CreatedBy: group.CreatedBy, Type: group.Type, EducationGroupID: group.EducationGroupID, ListKind: group.ListKind,
		IsTemplate: group.IsTemplate, IsSystem: group.IsSystem, ArchivedAt: group.ArchivedAt, SeriesRootID: group.SeriesRootID,
		CalendarPeriodID: group.CalendarPeriodID, TargetGroupType: group.TargetGroupType,
		TargetGradeLevel: group.TargetGradeLevel, TargetSchoolClass: group.TargetSchoolClass,
		SourceCareOfferingIDs: group.SourceCareOfferingIDs, SourceGradeLevels: group.SourceGradeLevels,
		SourceSchoolClasses: group.SourceSchoolClasses, Notes: group.Notes,
	}
}

func legacyCategory(category timetable.Category) *activitiesModels.Category {
	result := &activitiesModels.Category{
		Name: category.Name, Description: category.Description, Color: category.Color,
		IsSystem: category.IsSystem, ShiftTypeID: category.ShiftTypeID, ArchivedAt: category.ArchivedAt,
	}
	result.ID = category.ID
	result.CreatedAt = category.CreatedAt
	result.UpdatedAt = category.UpdatedAt
	result.SetTenantID(category.TenantID)
	return result
}

func legacyGroupID(id any) (int64, bool) {
	switch value := id.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func legacyGroupReadError(operation string, err error) error {
	if errors.Is(err, timetable.ErrGroupNotFound) || errors.Is(err, timetable.ErrInvalidGroupQuery) {
		return activitiesRepo.WrapNotFoundDatabaseError(operation)
	}
	return activitiesRepo.WrapDatabaseError(operation, err)
}
