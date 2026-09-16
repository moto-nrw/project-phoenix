package active

import "context"

// DatabaseHandle marks a live database behind the tenant unit of work. The
// retained services only test it for presence: a nil handle is the shape unit
// tests build, where every repository is a double and nothing needs a
// transaction, while production wiring always supplies one.
type DatabaseHandle interface {
	PingContext(context.Context) error
}
