package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// requestMemo binds the application memo to the slot the root resolves
// from the request context; a request without one is not memoized.
func RequestMemo(memo func(context.Context) identityBindingPorts.CallerMemo) func(context.Context) ports.CallerMemo {
	return func(ctx context.Context) ports.CallerMemo {
		if memo == nil {
			return nil
		}
		if slot := memo(ctx); slot != nil {
			return slot
		}
		return nil
	}
}
