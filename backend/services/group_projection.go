package services

import (
	"context"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// groupStudentRepository and OverlappingRosterGroupNames name the groups of
// the student roster and birthday reads through the School Structure owner, so
// no student read joins education.groups itself.
type OverlappingRosterGroupNames struct {
	userModels.StudentRepository
	Groups schoolstructure.Query
}

func (r OverlappingRosterGroupNames) FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*userModels.StudentWithGroupInfo, error) {
	return r.StudentRepository.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)
}

func (r OverlappingRosterGroupNames) FindOverlappingWithGroups(ctx context.Context, from, to, today timezone.Date) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindOverlappingWithGroups(ctx, from, to, today)
	return EnrichStudentGroupNames(ctx, r.Groups, rows, err)
}

type groupStudentRepository struct {
	userModels.StudentRepository
	groups schoolstructure.Query
}

func (r groupStudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherIDWithGroups(ctx, teacherID)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

func (r groupStudentRepository) FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

func (r groupStudentRepository) FindAllWithGroups(ctx context.Context) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindAllWithGroups(ctx)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

// EnrichStudentGroupNames fills StudentWithGroupInfo.GroupName from the
// School Structure owner.
func EnrichStudentGroupNames(ctx context.Context, groups schoolstructure.Query, rows []*userModels.StudentWithGroupInfo, queryErr error) ([]*userModels.StudentWithGroupInfo, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	return enrichGroupNames(ctx, groups, rows,
		func(row *userModels.StudentWithGroupInfo) int64 {
			if row.Student == nil {
				return 0
			}
			return optionalGroupID(row.GroupID)
		},
		func(row *userModels.StudentWithGroupInfo, name string) { row.GroupName = name },
		"", "student roster")
}

func (r groupStudentRepository) FindBirthdaysOn(ctx context.Context, days []userModels.MonthDay) ([]userModels.BirthdayEntry, error) {
	rows, err := r.StudentRepository.FindBirthdaysOn(ctx, days)
	if err != nil {
		return nil, err
	}
	pointers := make([]*userModels.BirthdayEntry, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *userModels.BirthdayEntry) int64 { return optionalGroupID(row.GroupID) },
		func(row *userModels.BirthdayEntry, name string) { row.GroupName = name },
		"", "birthdays"); err != nil {
		return nil, err
	}
	return rows, nil
}

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
