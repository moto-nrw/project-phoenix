//go:build testhooks

package owners

import (
	"context"
	"sync/atomic"
	"time"

	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentChildReadsWithDelay answers the parents portal's child lookup only
// after delay, so a test can tell what a route computes before and after the
// child check (ADR 0161 step 4, #5862). A zero delay answers at once.
func ParentChildReadsWithDelay(inner parentportal.ChildReads, delay *atomic.Int64) parentportal.ChildReads {
	return delayedParentChildReads{ChildReads: inner, delay: delay}
}

type delayedParentChildReads struct {
	parentportal.ChildReads
	delay *atomic.Int64
}

func (r delayedParentChildReads) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentportal.ChildSummary, error) {
	child, err := r.ChildReads.FindForAccount(ctx, accountID, studentID)
	time.Sleep(time.Duration(r.delay.Load()))
	return child, err
}
