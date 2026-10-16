package api

import (
	"context"
	"errors"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
)

// iotUnavailableMirrorSupervisions fails the trailing supervision read of a
// session start.
type iotUnavailableMirrorSupervisions struct{}

func (iotUnavailableMirrorSupervisions) ActiveSupervisions(context.Context, int64) ([]devicescanCompose.Supervision, error) {
	return nil, errors.New("supervision response lookup unavailable")
}
