package test

import (
	"context"
	"sync"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// SignalingGroupRepository reports when a row-lock lookup reaches its repository.
type SignalingGroupRepository struct {
	active.GroupRepository
	Entered chan struct{}
	once    sync.Once
}

func (repository *SignalingGroupRepository) FindByIDForUpdate(ctx context.Context, id int64) (*active.Group, error) {
	repository.once.Do(func() { close(repository.Entered) })
	return repository.GroupRepository.FindByIDForUpdate(ctx, id)
}
