package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	activitiesModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
)

// enrichGroupNames fills a display name on every row from the School
// Structure owner with one batched query. A group the owner cannot see in
// the caller's transaction yields the fallback, mirroring the former
// LEFT JOIN + COALESCE; a query failure is returned, never swallowed.
func enrichGroupNames[T any](
	ctx context.Context,
	groupNames func(context.Context, []int64) (map[int64]string, error),
	rows []T,
	groupID func(T) int64,
	setName func(T, string),
	fallback string,
	op string,
) ([]T, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, groupID(row))
	}
	names, err := groupNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for %s: %w", op, err)
	}
	for index := range rows {
		name, found := names[groupID(rows[index])]
		if !found {
			name = fallback
		}
		setName(rows[index], name)
	}
	return rows, nil
}

func optionalGroupID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// activityGroupTargets is the shape the timetable services type-assert on
// the activity group repository; the decorator keeps both halves reachable.
type activityGroupTargets interface {
	ports.ActivityGroupRecords
	ports.ActivityGroupTargets
}

type groupActivityGroupRepository struct {
	activityGroupTargets
	groups func(context.Context, []int64) (map[int64]string, error)
}

func (r groupActivityGroupRepository) FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activityContract.GroupTarget, error) {
	byGroup, err := r.activityGroupTargets.FindTargetsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	all := make([]*activityContract.GroupTarget, 0, len(byGroup))
	for _, targets := range byGroup {
		all = append(all, targets...)
	}
	if _, err := enrichGroupNames(ctx, r.groups, all,
		func(row *activityContract.GroupTarget) int64 { return optionalGroupID(row.EducationGroupID) },
		func(row *activityContract.GroupTarget, name string) { row.EducationGroupName = name },
		"", "template targets"); err != nil {
		return nil, err
	}
	return byGroup, nil
}

func (r groupActivityGroupRepository) ListTemplateRows(ctx context.Context, templateID *int64) ([]activitiesModels.TemplateListRow, error) {
	rows, err := r.activityGroupTargets.ListTemplateRows(ctx, templateID)
	return r.enrichTemplateRows(ctx, rows, err)
}

func (r groupActivityGroupRepository) ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]activitiesModels.TemplateListRow, error) {
	rows, err := r.activityGroupTargets.ListTemplateRowsForTemplatePeriod(ctx, templateID, periodID)
	return r.enrichTemplateRows(ctx, rows, err)
}

func (r groupActivityGroupRepository) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activitiesModels.TemplateListRow, error) {
	rows, err := r.activityGroupTargets.ListTemplateRowsForPeriod(ctx, periodID)
	return r.enrichTemplateRows(ctx, rows, err)
}

func (r groupActivityGroupRepository) enrichTemplateRows(ctx context.Context, rows []activitiesModels.TemplateListRow, queryErr error) ([]activitiesModels.TemplateListRow, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	pointers := make([]*activitiesModels.TemplateListRow, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *activitiesModels.TemplateListRow) int64 {
			if !row.EducationGroupID.Valid {
				return 0
			}
			return row.EducationGroupID.Int64
		},
		func(row *activitiesModels.TemplateListRow, name string) {
			// The former SQL projected COALESCE(name, ''), so the column was
			// always non-NULL; keep that shape for the row consumers.
			row.EducationGroupName.Valid = true
			row.EducationGroupName.String = name
		},
		"", "template rows"); err != nil {
		return nil, err
	}
	return rows, nil
}
