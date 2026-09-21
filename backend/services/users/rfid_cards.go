package users

import "context"

// RFIDCards supplies card existence and registration without persistence rows.
type RFIDCards interface {
	LookupRFIDCard(context.Context, string) (id string, active, found bool, err error)
	RegisterRFIDCard(context.Context, string) error
}
