package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// RequestPrincipal is the owner's public request principal.
type RequestPrincipal = studentpresence.RequestPrincipal

func (s *service) attendancePrincipal(ctx context.Context) RequestPrincipal {
	if s.PrincipalReader == nil {
		panic("attendance principal reader is not configured")
	}
	return s.PrincipalReader(ctx)
}
