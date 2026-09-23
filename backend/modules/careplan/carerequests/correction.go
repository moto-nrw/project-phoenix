package carerequests

import "context"

// Corrections requires the caller's authorized tenant transaction.
type Corrections interface {
	Correct(context.Context, int64, bool, string, string, int64) error
}
