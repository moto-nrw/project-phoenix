package application

import "context"

// RequestAccount reads the authenticated account of the request; 0 when the
// request carries none. The root binds it over Identity's session token
// adapter.
type RequestAccount interface {
	AccountID(context.Context) int64
}

// RequestSession reads the authenticated account of the request and the
// permissions it holds; no permissions when the request carries none. The root
// binds it over Identity's session token adapter.
type RequestSession interface {
	RequestAccount
	Permissions(context.Context) []string
}
