package compose

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
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
