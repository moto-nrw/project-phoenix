package repositories

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
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
	FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]presenceCompose.CrossTenantStudent, error)
}

// OpenVisitQuery is the Student Presence read behind the cross-tenant
// students: the visitors with an open visit at the hosting school.
type OpenVisitQuery interface {
	ListOpenVisitStudentIDs(context.Context, int64) ([]int64, error)
}

// NewCrossTenantQuery lists the visiting students of other schools: open
// visits from Student Presence, home-school identity and names from the
// People Directory, group names from School Structure.
func NewCrossTenantQuery(visits OpenVisitQuery, people peopledirectory.Capability, groups schoolstructure.Query) CrossTenantQuery {
	return groupCrossTenantRepository{
		CrossTenantQuery: &visitorProjection{visits: visits, students: people, persons: people},
		groups:           groups,
	}
}

type groupCrossTenantRepository struct {
	CrossTenantQuery
	groups schoolstructure.Query
}

func (r groupCrossTenantRepository) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]presenceCompose.CrossTenantStudent, error) {
	rows, err := r.CrossTenantQuery.FindCrossTenantStudents(ctx, hostingTenantID)
	if err != nil {
		return nil, err
	}
	pointers := make([]*presenceCompose.CrossTenantStudent, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *presenceCompose.CrossTenantStudent) int64 { return optionalGroupID(row.GroupID) },
		func(row *presenceCompose.CrossTenantStudent, name string) { row.GroupName = name },
		"", "cross-tenant students"); err != nil {
		return nil, err
	}
	return rows, nil
}

// NewGroupSupervisionRecords names the groups of the supervision blockers
// through the School Structure owner.
func NewGroupSupervisionRecords(records studentpresence.SupervisionRecords, groups schoolstructure.Query) studentpresence.SupervisionRecords {
	return groupSupervisorRepository{SupervisionRecords: records, groups: groups}
}

type groupSupervisorRepository struct {
	studentpresence.SupervisionRecords
	groups schoolstructure.Query
}

func (r groupSupervisorRepository) ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]studentpresence.SupervisionBlocker, error) {
	rows, err := r.SupervisionRecords.ListActiveSupervisionBlockers(ctx, staffID)
	if err != nil {
		return nil, err
	}
	pointers := make([]*studentpresence.SupervisionBlocker, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := enrichGroupNames(ctx, r.groups, pointers,
		func(row *studentpresence.SupervisionBlocker) int64 { return row.GroupID },
		func(row *studentpresence.SupervisionBlocker, name string) { row.GroupName = name },
		unknownGroupName, "supervision blockers"); err != nil {
		return nil, err
	}
	return rows, nil
}
