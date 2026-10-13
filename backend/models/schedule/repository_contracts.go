package schedule

import (
	"context"
)

// repository is the model-owned persistence contract shared by schedule
// repositories. Query shapes stay with their callers instead of coupling the
// schedule model to the generic base repository package.
type repository[T any] interface {
	Create(context.Context, T) error
	FindByID(context.Context, any) (T, error)
	Update(context.Context, T) error
	Delete(context.Context, any) error
}

type crudRepository[T any] interface {
	repository[T]
	List(context.Context, map[string]any) ([]T, error)
}
