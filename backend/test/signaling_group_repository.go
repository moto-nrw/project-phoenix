package test

import (
	"context"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// SignalingGroupRepository reports when a row-lock lookup reaches its repository.
type SignalingGroupRepository struct {
	studentpresence.SessionRecords
	Entered chan struct{}
	once    sync.Once
}

func (repository *SignalingGroupRepository) FindByIDForUpdate(ctx context.Context, id int64) (*studentpresence.LiveGroup, error) {
	repository.once.Do(func() { close(repository.Entered) })
	return repository.SessionRecords.FindByIDForUpdate(ctx, id)
}
