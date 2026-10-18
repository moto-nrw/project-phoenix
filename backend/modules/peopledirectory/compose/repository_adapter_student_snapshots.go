package compose

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
)

// The bound facade invokes the exact same retained reads, including their
// canonical hydration, error mapping and gate-before-row mutation lock order.
func (r studentRepositoryWithOwnerWrites) StudentSnapshotByID(ctx context.Context, id int64) (*directory.StudentSnapshot, error) {
	row, err := r.FindByID(ctx, id)
	return usersRepo.NativeStudentSnapshot(row), err
}
func (r studentRepositoryWithOwnerWrites) StudentSnapshotForUpdate(ctx context.Context, id int64) (*directory.StudentSnapshot, error) {
	row, err := r.FindByIDForUpdate(ctx, id)
	return usersRepo.NativeStudentSnapshot(row), err
}
