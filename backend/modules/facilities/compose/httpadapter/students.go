package httpadapter

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Students is the People Directory read port owned by this consumer: the
// children of a room snapshot and their names. The root binds it.
type Students interface {
	StudentsByIDs(ctx context.Context, ids []int64) (map[int64]peopledirectory.Student, error)
	PersonsByIDs(ctx context.Context, ids []int64) (map[int64]peopledirectory.Person, error)
}
