package identityaccess

import "context"

// RFIDCards owns school-scoped card registration and lookup. Registration
// validates the identifier; lookup also accepts existing legacy identifiers.
type RFIDCards interface {
	LookupRFIDCard(context.Context, string) (id string, active, found bool, err error)
	RegisterRFIDCard(context.Context, string) error
	ValidateRFIDTag(string) error
}
