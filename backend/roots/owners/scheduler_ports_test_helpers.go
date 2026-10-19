//go:build testhooks

package owners

import (
	"context"
)

// transitionStudentStatus passes stored status values to the retained
// compare-and-set, whose status parameter is such a named string type.
func transitionStudentStatus[S ~string](transition func(context.Context, int64, S, S) (bool, error), ctx context.Context, studentID int64, expected, next string) (bool, error) {
	return transition(ctx, studentID, S(expected), S(next))
}
