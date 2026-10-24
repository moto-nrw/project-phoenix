package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

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
	if _, err := EnrichGroupNames(ctx, r.groups, pointers,
		func(row *presenceCompose.CrossTenantStudent) int64 {
			return OptionalGroupID(row.GroupID)
		},
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
	groups groupNameQuery
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
	if _, err := EnrichGroupNames(ctx, r.groups, pointers,
		func(row *studentpresence.SupervisionBlocker) int64 { return row.GroupID },
		func(row *studentpresence.SupervisionBlocker, name string) { row.GroupName = name },
		schoolstructure.UnknownGroupName, "supervision blockers"); err != nil {
		return nil, err
	}
	return rows, nil
}
