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
// supervision whose group is not visible (kept from the former SQL).
const unknownGroupName = "Unbekannte Gruppe"

// enrichGroupNames fills a display name on every row from the School
// Structure owner with one batched query. A group the owner cannot see in
// the caller's transaction yields the fallback, mirroring the former
// LEFT JOIN + COALESCE; a query failure is returned, never swallowed.
func enrichGroupNames[T any](
	ctx context.Context,
	query schoolstructure.Query,
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
	names, err := groupNamesByID(ctx, query, ids)
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
	if err != nil {
		return nil, err
	}
	pointers := make([]*activeModels.CrossTenantStudent, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *activeModels.CrossTenantStudent) int64 { return optionalGroupID(row.GroupID) },
		func(row *activeModels.CrossTenantStudent, name string) { row.GroupName = name },
		"", "cross-tenant students"); err != nil {
		return nil, err
	}
	return rows, nil
}

type groupSupervisorRepository struct {
	activeModels.GroupSupervisorRepository
	groups schoolstructure.Query
}

func (r groupSupervisorRepository) ListActiveSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]usersModels.BlockerSupervision, error) {
	rows, err := r.GroupSupervisorRepository.ListActiveSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, err
	}
	pointers := make([]*usersModels.BlockerSupervision, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *usersModels.BlockerSupervision) int64 { return row.GroupID },
		func(row *usersModels.BlockerSupervision, name string) { row.GroupName = name },
		unknownGroupName, "supervision blockers"); err != nil {
		return nil, err
	}
	return rows, nil
}

type groupVisitRepository struct {
	activeModels.VisitRepository
	groups schoolstructure.Query
}

func (r groupVisitRepository) FindActiveWithStudentDisplayByGroup(ctx context.Context, activeGroupID int64) ([]*activeModels.VisitWithStudentDisplay, error) {
	rows, err := r.VisitRepository.FindActiveWithStudentDisplayByGroup(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	return enrichGroupNames(ctx, r.groups, rows,
		func(row *activeModels.VisitWithStudentDisplay) int64 { return optionalGroupID(row.GroupID) },
		func(row *activeModels.VisitWithStudentDisplay, name string) { row.OGSGroupName = name },
		"", "active group visits")
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
	if err != nil {
		return nil, err
	}
	all := make([]*activitiesModels.GroupTarget, 0, len(byGroup))
	for _, targets := range byGroup {
		all = append(all, targets...)
	}
	if _, err := enrichGroupNames(ctx, r.groups, all,
		func(row *activitiesModels.GroupTarget) int64 { return optionalGroupID(row.EducationGroupID) },
		func(row *activitiesModels.GroupTarget, name string) { row.EducationGroupName = name },
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
	if queryErr != nil {
		return nil, queryErr
	}
	return enrichGroupNames(ctx, query, rows,
		func(row *usersModels.InboxThread) int64 { return optionalGroupID(row.GroupID) },
		func(row *usersModels.InboxThread, name string) { row.GroupName = name },
		"", "parent message inbox")
}

type groupStudentRepository struct {
	usersModels.StudentRepository
	groups schoolstructure.Query
}

func (r groupStudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*usersModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherIDWithGroups(ctx, teacherID)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

func (r groupStudentRepository) FindByTeacherIDsWithGroups(ctx context.Context, teacherIDs []int64) ([]*usersModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherIDsWithGroups(ctx, teacherIDs)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

func (r groupStudentRepository) FindAllWithGroups(ctx context.Context) ([]*usersModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindAllWithGroups(ctx)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

// FindOverlappingWithGroups is decorated in the services composition root
// (services/group_projection.go): its calendar-date signature would pull
// internal/timezone into this package.

// EnrichStudentGroupNames fills StudentWithGroupInfo.GroupName from the
// School Structure owner. Exported for the one roster read whose signature
// keeps its decorator in the services composition root.
func EnrichStudentGroupNames(ctx context.Context, groups schoolstructure.Query, rows []*usersModels.StudentWithGroupInfo, queryErr error) ([]*usersModels.StudentWithGroupInfo, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	return enrichGroupNames(ctx, groups, rows,
		func(row *usersModels.StudentWithGroupInfo) int64 {
			if row.Student == nil {
				return 0
			}
			return optionalGroupID(row.GroupID)
		},
		func(row *usersModels.StudentWithGroupInfo, name string) { row.GroupName = name },
		"", "student roster")
}

func (r groupStudentRepository) FindBirthdaysOn(ctx context.Context, days []usersModels.MonthDay) ([]usersModels.BirthdayEntry, error) {
	rows, err := r.StudentRepository.FindBirthdaysOn(ctx, days)
	if err != nil {
		return nil, err
	}
	pointers := make([]*usersModels.BirthdayEntry, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *usersModels.BirthdayEntry) int64 { return optionalGroupID(row.GroupID) },
		func(row *usersModels.BirthdayEntry, name string) { row.GroupName = name },
		"", "birthdays"); err != nil {
		return nil, err
	}
	return rows, nil
}
