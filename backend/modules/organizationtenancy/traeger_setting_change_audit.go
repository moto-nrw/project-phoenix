package organizationtenancy

import (
	"context"
	"net"
)

// TraegerSettingChange is an operator's change of one setting of a Träger
// from the operator dashboard (#5748). Reset is true when the Träger value
// returns to the registry default. No value belongs here: the change log of
// the Träger values records old and new value itself.
type TraegerSettingChange struct {
	OperatorID     int64
	OrganizationID int64
	SettingKey     string
	Reset          bool
	ClientIP       net.IP
}

// TraegerSettingChangeAudit records the operator audit entry of a Träger
// setting change. The change calls it after its checks and before it writes,
// so no change happens without its operator audit entry; an unknown Träger
// answers OrganizationNotFoundError.
type TraegerSettingChangeAudit interface {
	RecordTraegerSettingChange(ctx context.Context, change TraegerSettingChange) error
}
