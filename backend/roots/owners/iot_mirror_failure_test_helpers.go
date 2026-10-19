//go:build testhooks

package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// UnavailableMirrorSupervisions fails the trailing supervision read of a
// session start.
type UnavailableMirrorSupervisions struct{}

func (UnavailableMirrorSupervisions) ActiveSupervisions(context.Context, int64) ([]devicescan.Supervision, error) {
	return nil, errors.New("supervision response lookup unavailable")
}
