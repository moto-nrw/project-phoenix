package compose

import (
	"context"

	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// RosterStudents reads and locks the People Directory children the activity
// rosters and the operational day name; the root also serves the lifecycle's
// LifecycleStudents from it.
type RosterStudents interface {
	FindByID(ctx context.Context, id any) (*usersModel.Student, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*usersModel.Student, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Student, error)
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*usersModel.Student, error)
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Student, error)
}
