package services

import "context"

// accountID returns the acting account of the request.
func (a TokenAdapter) accountID(ctx context.Context) int64 {
	return a.claims(ctx).AccountID
}
