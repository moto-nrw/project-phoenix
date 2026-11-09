//go:build testhooks

package application

import (
	"context"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// DecideAndInviteWith runs the decision command's unit and connectivity
// retry around decide instead of the full decision flow. Only test builds
// carry it: the integration tests drive the retry on the real tenant
// runtime with injected failures (#6316).
func (d *Decisions) DecideAndInviteWith(ctx context.Context, input enrollmentNative.DecideInput, decide func(context.Context, enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error)) (*enrollmentNative.DecideOutcome, error) {
	return d.decideAndInvite(ctx, input, decide)
}
