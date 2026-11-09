package organizationtenancy

import (
	"context"
	"net"
)

// Kinds of a change of the Beleg retention of a Träger (#4720, ADR 0184).
const (
	BelegRetentionInstruction    = "instruction"
	BelegRetentionSuspension     = "suspension"
	BelegRetentionSuspensionLift = "suspension_lift"
)

// BelegRetentionChange is an operator's record of an instruction of a
// Träger, of a suspension or of its lift in the operator dashboard (#4720,
// ADR 0184). Kind is one of the BelegRetention* constants and BelegType the
// type it concerns; no text of the instruction belongs here, the Beleg
// retention keeps it itself.
type BelegRetentionChange struct {
	OperatorID     int64
	OrganizationID int64
	Kind           string
	BelegType      string
	ClientIP       net.IP
}

// BelegRetentionChangeAudit records the operator audit entry of a change of
// the Beleg retention of a Träger. The change calls it in its own
// administrative transaction before it writes, so no change commits without
// its entry; an unknown Träger answers OrganizationNotFoundError. It is a
// capability of its own beside Provisioning; compose.BelegRetentionAudit
// answers it for the provisioning the root composed.
type BelegRetentionChangeAudit interface {
	RecordBelegRetentionChange(ctx context.Context, change BelegRetentionChange) error
}
