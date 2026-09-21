package users

import "context"

// RequestPermissions supplies the authenticated caller's grants to review
// decisions. The composition root owns the request-context representation.
type RequestPermissions func(context.Context) []string
