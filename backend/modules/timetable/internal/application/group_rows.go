package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// The activity service, the template writes and split, the materializer,
// the operational day and the instance lifecycle still compute on the legacy
// group row. They call the owner's group capability through these reads and
// writes, which convert the rows and report failures under the operation
// names the retained repository used.

func findLegacyGroup(ctx context.Context, groups timetable.ActivityGroupQuery, id int64) (*activityContract.Group, error) {
	group, err := groups.FindGroup(ctx, id)
	if err != nil {
		return nil, legacyGroupReadError("find by id", err)
	}
	return legacyGroup(group), nil
}

func findLegacyGroupForUpdate(ctx context.Context, groups timetable.ActivityGroupQuery, id int64) (*activityContract.Group, error) {
	group, err := groups.FindGroupForUpdate(ctx, id)
	if err != nil {
		return nil, legacyGroupReadError("find by id for update", err)
	}
	return legacyGroup(group), nil
}

func findLegacyGroupByName(ctx context.Context, groups timetable.ActivityGroupQuery, name string) (*activityContract.Group, error) {
	group, err := groups.FindGroupByName(ctx, name)
	if err != nil {
		return nil, legacyGroupReadError("find group by name", err)
	}
	return legacyGroup(group), nil
}

// legacyGroupsByID returns the groups with the given IDs, archived ones
// included; no IDs read nothing.
func legacyGroupsByID(ctx context.Context, groups timetable.ActivityGroupQuery, ids []int64) ([]*activityContract.Group, error) {
	if len(ids) == 0 {
		return []*activityContract.Group{}, nil
	}
	return listLegacyGroups(ctx, groups, timetable.GroupFilter{IDs: ids}, "find groups by ids")
}

func listLegacyGroups(ctx context.Context, groups timetable.ActivityGroupQuery, filter timetable.GroupFilter, operation string) ([]*activityContract.Group, error) {
	values, err := groups.ListGroups(ctx, filter)
	if err != nil {
		return nil, legacyGroupReadError(operation, err)
	}
	return legacyGroups(values), nil
}

// createLegacyGroup creates a group through the owner's group command with
// the legacy row checks, and copies the stored row back.
func createLegacyGroup(ctx context.Context, groups timetable.ActivityGroupCommand, group *activityContract.Group) error {
	if group == nil {
		return errors.New("group cannot be nil or zero value")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	created, err := groups.CreateGroup(ctx, publicGroupInput(group))
	if err != nil {
		return fmt.Errorf("database error during create: %w", err)
	}
	*group = *legacyGroup(created)
	return nil
}

func updateLegacyGroup(ctx context.Context, groups timetable.ActivityGroupCommand, group *activityContract.Group) error {
	if group == nil {
		return errors.New("group cannot be nil")
	}
	if err := group.Validate(); err != nil {
		return err
	}
	if _, err := groups.UpdateGroup(ctx, group.ID, publicGroupInput(group)); err != nil {
		return legacyGroupReadError("update", err)
	}
	return nil
}

func deleteLegacyGroup(ctx context.Context, groups timetable.ActivityGroupCommand, id int64) error {
	if err := groups.DeleteGroup(ctx, id); err != nil {
		return fmt.Errorf("database error during delete: %w", err)
	}
	return nil
}

// legacyGroupTargets returns the dynamic targets of the groups.
func legacyGroupTargets(ctx context.Context, groups timetable.ActivityGroupQuery, groupIDs []int64) (map[int64][]*activityContract.GroupTarget, error) {
	values, err := groups.ListGroupTargets(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]*activityContract.GroupTarget, len(values))
	for groupID, targets := range values {
		for _, target := range targets {
			result[groupID] = append(result[groupID], legacyGroupTarget(target))
		}
	}
	return result, nil
}

// replaceLegacyGroupTargets replaces the dynamic targets of a group; every
// target must be valid and all of them must have the same type.
func replaceLegacyGroupTargets(ctx context.Context, groups timetable.ActivityGroupCommand, groupID int64, targets []*activityContract.GroupTarget) error {
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
	return groups.ReplaceGroupTargets(ctx, groupID, inputs)
}

// listTargetStudentIDs lists the children the dynamic targets of a group reach.
func listTargetStudentIDs(ctx context.Context, groups timetable.ActivityGroupQuery, groupID int64) ([]int64, error) {
	byGroup, err := groups.ListTargetStudentIDs(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	return byGroup[groupID], nil
}

func uniqueSupervisorGroupIDs(values []timetable.PlannedSupervisor) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value.GroupID]; exists {
			continue
		}
		seen[value.GroupID] = struct{}{}
		result = append(result, value.GroupID)
	}
	return result
}

func legacyGroupTarget(target timetable.GroupTarget) *activityContract.GroupTarget {
	result := &activityContract.GroupTarget{
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

func legacyGroups(groups []timetable.Group) []*activityContract.Group {
	result := make([]*activityContract.Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, legacyGroup(group))
	}
	return result
}

func legacyGroup(group timetable.Group) *activityContract.Group {
	result := &activityContract.Group{
		Name: group.Name, MaxParticipants: group.MaxParticipants, RequiredStaff: group.RequiredStaff,
		IsOpen: group.IsOpen, CategoryID: group.CategoryID, PlanningTrackID: group.PlanningTrackID,
		PlannedRoomID: group.PlannedRoomID, CreatedBy: group.CreatedBy, Type: group.Type,
		EducationGroupID: group.EducationGroupID, ListKind: group.ListKind, IsTemplate: group.IsTemplate,
		IsSystem: group.IsSystem, ArchivedAt: group.ArchivedAt, SeriesRootID: group.SeriesRootID,
		CalendarPeriodID: group.CalendarPeriodID, TargetGroupType: group.TargetGroupType,
		TargetGradeLevel: group.TargetGradeLevel, TargetSchoolClass: group.TargetSchoolClass,
		SourceCareOfferingIDs: group.SourceCareOfferingIDs, SourceGradeLevels: group.SourceGradeLevels,
		SourceSchoolClasses: group.SourceSchoolClasses, Notes: group.Notes, IncludeClosingDays: group.IncludeClosingDays,
		SeriesLastDay: activityDateFromString(group.SeriesLastDay),
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

func publicGroupInput(group *activityContract.Group) timetable.GroupInput {
	return timetable.GroupInput{
		Name: group.Name, MaxParticipants: group.MaxParticipants, RequiredStaff: group.RequiredStaff, IsOpen: group.IsOpen,
		CategoryID: group.CategoryID, PlanningTrackID: group.PlanningTrackID, PlannedRoomID: group.PlannedRoomID,
		CreatedBy: group.CreatedBy, Type: group.Type, EducationGroupID: group.EducationGroupID, ListKind: group.ListKind,
		IsTemplate: group.IsTemplate, IsSystem: group.IsSystem, ArchivedAt: group.ArchivedAt, SeriesRootID: group.SeriesRootID,
		CalendarPeriodID: group.CalendarPeriodID, TargetGroupType: group.TargetGroupType,
		TargetGradeLevel: group.TargetGradeLevel, TargetSchoolClass: group.TargetSchoolClass,
		SourceCareOfferingIDs: group.SourceCareOfferingIDs, SourceGradeLevels: group.SourceGradeLevels,
		SourceSchoolClasses: group.SourceSchoolClasses, Notes: group.Notes, IncludeClosingDays: group.IncludeClosingDays,
		SeriesLastDay: activityDateString(group.SeriesLastDay),
	}
}

func activityDateFromString(value *string) *activityContract.Date {
	if value == nil {
		return nil
	}
	day := activityContract.Date(*value)
	return &day
}

func activityDateString(value *activityContract.Date) *string {
	if value == nil {
		return nil
	}
	day := value.String()
	return &day
}

func legacyCategory(category timetable.Category) *activityContract.Category {
	result := &activityContract.Category{
		Name: category.Name, Description: category.Description, Color: category.Color,
		IsSystem: category.IsSystem, ShiftTypeID: category.ShiftTypeID, ArchivedAt: category.ArchivedAt,
	}
	result.ID = category.ID
	result.CreatedAt = category.CreatedAt
	result.UpdatedAt = category.UpdatedAt
	result.SetTenantID(category.TenantID)
	return result
}

func LegacyGroupID(id any) (int64, bool) {
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
		return legacyNotFoundError(operation)
	}
	return legacyDatabaseError(operation, err)
}
