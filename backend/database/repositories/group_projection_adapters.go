package repositories

import (
	"context"
	"fmt"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// unknownGroupName is the caregiver-capability blocker label for a
// supervision whose group is no longer visible (kept from the former SQL).
const unknownGroupName = "Unbekannte Gruppe"

// groupNamesByID resolves display names through the School Structure owner.
// Absent groups stay absent, mirroring the LEFT JOIN + COALESCE the callers
// used before the cutover; a query failure is returned, never swallowed.
func groupNamesByID(ctx context.Context, query schoolstructure.Query, ids []int64) (map[int64]string, error) {
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
	names := make(map[int64]string, len(unique))
	if len(unique) == 0 {
		return names, nil
	}
	groups, err := query.ListGroupsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		names[group.ID] = group.Name
	}
	return names, nil
}

func optionalGroupID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// CrossTenantQuery is the cross-tenant student read the active service consumes.
type CrossTenantQuery interface {
	FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]activeModels.CrossTenantStudent, error)
}

type groupCrossTenantRepository struct {
	CrossTenantQuery
	groups schoolstructure.Query
}

func (r groupCrossTenantRepository) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]activeModels.CrossTenantStudent, error) {
	rows, err := r.CrossTenantQuery.FindCrossTenantStudents(ctx, hostingTenantID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, optionalGroupID(row.GroupID))
	}
	names, err := groupNamesByID(ctx, r.groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for cross-tenant students: %w", err)
	}
	for index := range rows {
		rows[index].GroupName = names[optionalGroupID(rows[index].GroupID)]
	}
	return rows, nil
}

type groupSupervisorRepository struct {
	activeModels.GroupSupervisorRepository
	groups schoolstructure.Query
}

func (r groupSupervisorRepository) ListActiveSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]usersModels.BlockerSupervision, error) {
	rows, err := r.GroupSupervisorRepository.ListActiveSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.GroupID)
	}
	names, err := groupNamesByID(ctx, r.groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for supervision blockers: %w", err)
	}
	for index := range rows {
		name, found := names[rows[index].GroupID]
		if !found {
			name = unknownGroupName
		}
		rows[index].GroupName = name
	}
	return rows, nil
}

type groupVisitRepository struct {
	activeModels.VisitRepository
	groups schoolstructure.Query
}

func (r groupVisitRepository) FindActiveWithStudentDisplayByGroup(ctx context.Context, activeGroupID int64) ([]*activeModels.VisitWithStudentDisplay, error) {
	rows, err := r.VisitRepository.FindActiveWithStudentDisplayByGroup(ctx, activeGroupID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, optionalGroupID(row.GroupID))
	}
	names, err := groupNamesByID(ctx, r.groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for active group visits: %w", err)
	}
	for _, row := range rows {
		row.OGSGroupName = names[optionalGroupID(row.GroupID)]
	}
	return rows, nil
}

// activityGroupTargets is the shape the timetable services type-assert on
// the activity group repository; the decorator keeps both halves reachable.
type activityGroupTargets interface {
	activitiesModels.GroupRepository
	activitiesModels.GroupTargetRepository
}

type groupActivityGroupRepository struct {
	activityGroupTargets
	groups schoolstructure.Query
}

func (r groupActivityGroupRepository) FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activitiesModels.GroupTarget, error) {
	byGroup, err := r.activityGroupTargets.FindTargetsByGroupIDs(ctx, groupIDs)
	if err != nil || len(byGroup) == 0 {
		return byGroup, err
	}
	ids := make([]int64, 0, len(byGroup))
	for _, targets := range byGroup {
		for _, target := range targets {
			ids = append(ids, optionalGroupID(target.EducationGroupID))
		}
	}
	names, err := groupNamesByID(ctx, r.groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for template targets: %w", err)
	}
	for _, targets := range byGroup {
		for _, target := range targets {
			target.EducationGroupName = names[optionalGroupID(target.EducationGroupID)]
		}
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
	if queryErr != nil || len(rows) == 0 {
		return rows, queryErr
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.EducationGroupID.Valid {
			ids = append(ids, row.EducationGroupID.Int64)
		}
	}
	names, err := groupNamesByID(ctx, r.groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for template rows: %w", err)
	}
	for index := range rows {
		// The former SQL projected COALESCE(name, ''), so the column was
		// always non-NULL; keep that shape for the row consumers.
		rows[index].EducationGroupName.Valid = true
		rows[index].EducationGroupName.String = ""
		if rows[index].EducationGroupID.Valid {
			rows[index].EducationGroupName.String = names[rows[index].EducationGroupID.Int64]
		}
	}
	return rows, nil
}

type groupParentMessageReadRepository struct {
	usersModels.ParentMessageReadRepository
	groups schoolstructure.Query
}

func (r groupParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}

func (r groupParentMessageReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForStudent(ctx, accountID, studentID)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}

func (r groupParentMessageReadRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}

func (r groupParentMessageReadRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}

func enrichInboxGroups(ctx context.Context, query schoolstructure.Query, rows []*usersModels.InboxThread, queryErr error) ([]*usersModels.InboxThread, error) {
	if queryErr != nil || len(rows) == 0 {
		return rows, queryErr
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, optionalGroupID(row.GroupID))
	}
	names, err := groupNamesByID(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for parent message inbox: %w", err)
	}
	for _, row := range rows {
		row.GroupName = names[optionalGroupID(row.GroupID)]
	}
	return rows, nil
}

type groupStudentRepository struct {
	usersModels.StudentRepository
	groups schoolstructure.Query
}

func (r groupStudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*usersModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherIDWithGroups(ctx, teacherID)
	return r.enrichStudents(ctx, rows, err)
}

func (r groupStudentRepository) FindAllWithGroups(ctx context.Context) ([]*usersModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindAllWithGroups(ctx)
	return r.enrichStudents(ctx, rows, err)
}

// FindOverlappingWithGroups is decorated in the services composition root
// (services/group_projection.go): its calendar-date signature would pull
// internal/timezone into this package.

func (r groupStudentRepository) enrichStudents(ctx context.Context, rows []*usersModels.StudentWithGroupInfo, queryErr error) ([]*usersModels.StudentWithGroupInfo, error) {
	return EnrichStudentGroupNames(ctx, r.groups, rows, queryErr)
}

// EnrichStudentGroupNames fills StudentWithGroupInfo.GroupName from the
// School Structure owner. Exported for the one roster read whose signature
// keeps its decorator in the services composition root.
func EnrichStudentGroupNames(ctx context.Context, groups schoolstructure.Query, rows []*usersModels.StudentWithGroupInfo, queryErr error) ([]*usersModels.StudentWithGroupInfo, error) {
	if queryErr != nil || len(rows) == 0 {
		return rows, queryErr
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.Student != nil {
			ids = append(ids, optionalGroupID(row.GroupID))
		}
	}
	names, err := groupNamesByID(ctx, groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for student roster: %w", err)
	}
	for _, row := range rows {
		if row.Student != nil {
			row.GroupName = names[optionalGroupID(row.GroupID)]
		}
	}
	return rows, nil
}

func (r groupStudentRepository) FindBirthdaysOn(ctx context.Context, days []usersModels.MonthDay) ([]usersModels.BirthdayEntry, error) {
	rows, err := r.StudentRepository.FindBirthdaysOn(ctx, days)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, optionalGroupID(row.GroupID))
	}
	names, err := groupNamesByID(ctx, r.groups, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for birthdays: %w", err)
	}
	for index := range rows {
		rows[index].GroupName = names[optionalGroupID(rows[index].GroupID)]
	}
	return rows, nil
}
