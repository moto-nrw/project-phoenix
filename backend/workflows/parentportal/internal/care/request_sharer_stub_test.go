package care

import (
	"context"
	"errors"
)

// unconfiguredRequestSharer is the test-only RequestSharer for scenarios that
// run without the sharing ledger: an empty recipient list writes nothing, any
// other choice is refused, and every request is visible only to its
// submitter. package care_test has its own copy; types in this file are not
// importable from the external test package.
type unconfiguredRequestSharer struct{}

func (unconfiguredRequestSharer) ShareRequestInTx(
	_ context.Context, _, _ int64, _ string, _ int64, recipientProfileIDs []int64,
) error {
	if len(recipientProfileIDs) == 0 {
		return nil
	}
	return errors.New("parent: request sharing service is not configured")
}

func (unconfiguredRequestSharer) LoadRequestShareVisibility(context.Context, int64) (RequestShareVisibility, error) {
	return submitterOnlyVisibility{}, nil
}

type submitterOnlyVisibility struct{}

func (submitterOnlyVisibility) Allows(_ string, _, accountID, submittedBy int64) bool {
	return submittedBy == accountID
}
