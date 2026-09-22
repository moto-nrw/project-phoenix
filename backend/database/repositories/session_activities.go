package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

type SessionActivityRecords interface {
	FindByID(context.Context, any) (*activities.Group, error)
	FindByIDs(context.Context, []int64) ([]*activities.Group, error)
	ListWithCategory(context.Context, *activities.GroupListQuery) ([]*activities.Group, error)
}

type SessionActivities struct{ records SessionActivityRecords }

func NewSessionActivities(records SessionActivityRecords) *SessionActivities {
	return &SessionActivities{records: records}
}

func (r *SessionActivities) FindByID(ctx context.Context, id any) (*presenceCompose.SessionActivity, error) {
	row, err := r.records.FindByID(ctx, id)
	return sessionActivity(row), err
}

func (r *SessionActivities) FindByIDs(ctx context.Context, ids []int64) ([]*presenceCompose.SessionActivity, error) {
	rows, err := r.records.FindByIDs(ctx, ids)
	return sessionActivities(rows), err
}

func (r *SessionActivities) ListSessionActivities(ctx context.Context) ([]*presenceCompose.SessionActivity, error) {
	rows, err := r.records.ListWithCategory(ctx, nil)
	return sessionActivities(rows), err
}

func sessionActivities(rows []*activities.Group) []*presenceCompose.SessionActivity {
	if rows == nil {
		return nil
	}
	result := make([]*presenceCompose.SessionActivity, len(rows))
	for i, row := range rows {
		result[i] = sessionActivity(row)
	}
	return result
}

func sessionActivity(row *activities.Group) *presenceCompose.SessionActivity {
	if row == nil {
		return nil
	}
	result := &presenceCompose.SessionActivity{
		ID:                    row.ID,
		TenantID:              row.TenantID,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		Name:                  row.Name,
		MaxParticipants:       row.MaxParticipants,
		RequiredStaff:         row.RequiredStaff,
		IsOpen:                row.IsOpen,
		CategoryID:            row.CategoryID,
		PlanningTrackID:       row.PlanningTrackID,
		PlannedRoomID:         row.PlannedRoomID,
		CreatedBy:             row.CreatedBy,
		Type:                  row.Type,
		EducationGroupID:      row.EducationGroupID,
		ListKind:              row.ListKind,
		IsTemplate:            row.IsTemplate,
		IsSystem:              row.IsSystem,
		ArchivedAt:            row.ArchivedAt,
		SeriesRootID:          row.SeriesRootID,
		CalendarPeriodID:      row.CalendarPeriodID,
		TargetGroupType:       row.TargetGroupType,
		TargetGradeLevel:      row.TargetGradeLevel,
		TargetSchoolClass:     row.TargetSchoolClass,
		SourceCareOfferingIDs: row.SourceCareOfferingIDs,
		SourceGradeLevels:     row.SourceGradeLevels,
		SourceSchoolClasses:   row.SourceSchoolClasses,
		Notes:                 row.Notes,
	}
	if row.Category != nil {
		result.Category = &presenceCompose.SessionActivityCategory{
			ID:          row.Category.ID,
			TenantID:    row.Category.TenantID,
			CreatedAt:   row.Category.CreatedAt,
			UpdatedAt:   row.Category.UpdatedAt,
			Name:        row.Category.Name,
			Description: row.Category.Description,
			Color:       row.Category.Color,
			IsSystem:    row.Category.IsSystem,
			ShiftTypeID: row.Category.ShiftTypeID,
			ArchivedAt:  row.Category.ArchivedAt,
		}
	}
	return result
}
