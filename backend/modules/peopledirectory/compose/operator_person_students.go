package compose

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
)

// OperatorPersonStudentMembership marks which of the persons are students,
// for the operator directory the legacy composition assembles. Staff
// membership belongs to School Membership and is not answered here.
func (r LegacyRepositoryRuntime) OperatorPersonStudentMembership(ctx context.Context, students usersRepo.StudentDirectoryRows, personIDs []int64) (map[int64]bool, error) {
	return usersRepo.FindOperatorPersonStudentMembership(ctx, r, students, personIDs)
}
