package owners

import (
	"context"
	"fmt"
)

// runSteps runs the owners' deletion steps of one school in order and logs
// each deletion, as the Worker's nightly run does (services/scheduler).
func (r workerRetention) runSteps(ctx context.Context) error {
	for _, step := range r.steps {
		outcome, err := step.Run(ctx)
		if err != nil {
			return fmt.Errorf("retention step %s: %w", step.Kind, err)
		}
		if outcome.Deleted == 0 || step.LogsSubjects {
			continue
		}
		if err := r.log(ctx, step.Kind, outcome); err != nil {
			return fmt.Errorf("log retention step %s: %w", step.Kind, err)
		}
	}
	return nil
}
