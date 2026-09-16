package care

import (
	"context"
	"errors"
)

// UnconfiguredRequestSharer is the test-only RequestSharer for scenarios that
// run without the sharing ledger: an empty recipient list writes nothing, any
// other choice is refused, and every request is visible only to its
// submitter. It is exported so the external behaviour tests in this directory
// can use it too.
type UnconfiguredRequestSharer struct{}

func (UnconfiguredRequestSharer) ShareRequestInTx(
	_ context.Context, _, _ int64, _ string, _ int64, recipientProfileIDs []int64,
) error {
	if len(recipientProfileIDs) == 0 {
		return nil
	}
	return errors.New("parent: request sharing service is not configured")
}

func (UnconfiguredRequestSharer) LoadRequestShareVisibility(context.Context, int64) (RequestShareVisibility, error) {
	return submitterOnlyVisibility{}, nil
}

type submitterOnlyVisibility struct{}

func (submitterOnlyVisibility) Allows(_ string, _, accountID, submittedBy int64) bool {
	return submittedBy == accountID
}
