package application

import (
	"context"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// DecideAndInviteWith runs the decision command's unit and retry around
// decide instead of the full decision flow, for the runtime tests of the
// external test package.
func DecideAndInviteWith(deps DecisionDependencies, ctx context.Context, input enrollmentNative.DecideInput, decide func(context.Context, enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error)) (*enrollmentNative.DecideOutcome, error) {
	return (&Decisions{deps: deps}).decideAndInvite(ctx, input, decide)
}
