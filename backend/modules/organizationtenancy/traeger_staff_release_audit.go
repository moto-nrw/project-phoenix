package organizationtenancy

import (
	"context"
	"net"
)

// Kinds of a change of the release „Personal beim Träger“ (#4705, ADR 0089
// section 2).
const (
	TraegerStaffReleaseGranted   = "granted"
	TraegerStaffReleaseWithdrawn = "withdrawn"
)

// TraegerStaffReleaseChange is an operator's grant or withdrawal of the
// release „Personal beim Träger“. Representation is the statement the Träger
// confirmed before the grant: "agreement" with its employee representation
// (§ 87 Abs. 1 Nr. 2 and Nr. 6 BetrVG, MAVO, MVG.EKD) or "none"; a
// withdrawal names none.
type TraegerStaffReleaseChange struct {
	OperatorID     int64
	OrganizationID int64
	Kind           string
	Representation string
	ClientIP       net.IP
}

// TraegerStaffReleaseChangeAudit records the operator audit entry of a
// release change. The change calls it in its own administrative transaction
// before it writes, so no change commits without its entry; an unknown
// Träger answers OrganizationNotFoundError. compose.TraegerStaffReleaseAudit
// answers it for the provisioning the root composed.
type TraegerStaffReleaseChangeAudit interface {
	RecordTraegerStaffReleaseChange(ctx context.Context, change TraegerStaffReleaseChange) error
}
