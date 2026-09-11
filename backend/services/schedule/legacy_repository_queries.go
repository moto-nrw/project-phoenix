package schedule

import (
	"context"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

type legacyListRepository[T any] interface {
	List(context.Context, *modelBase.QueryOptions) ([]T, error)
}

type legacyListWithOptionsRepository[T any] interface {
	ListWithOptions(context.Context, *modelBase.QueryOptions) ([]T, error)
}

type legacyCountRepository interface {
	CountWithOptions(context.Context, *modelBase.QueryOptions) (int, error)
}

func legacyList[T any](ctx context.Context, repository any, options *modelBase.QueryOptions) ([]T, error) {
	lister, ok := repository.(legacyListRepository[T])
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", repository)
	}
	return lister.List(ctx, options)
}

func legacyListWithOptions[T any](ctx context.Context, repository any, options *modelBase.QueryOptions) ([]T, error) {
	lister, ok := repository.(legacyListWithOptionsRepository[T])
	if !ok {
		return nil, fmt.Errorf("legacy list-with-options capability is not configured for %T", repository)
	}
	return lister.ListWithOptions(ctx, options)
}

func legacyCount(ctx context.Context, repository any, options *modelBase.QueryOptions) (int, error) {
	counter, ok := repository.(legacyCountRepository)
	if !ok {
		return 0, fmt.Errorf("legacy count capability is not configured for %T", repository)
	}
	return counter.CountWithOptions(ctx, options)
}
