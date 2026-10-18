package parentinbox

import (
	"github.com/uptrace/bun"
)

// The projection answers in its own vocabulary (ADR 0058); Communication's
// parent store maps the rows onto its repository contract.

// GuardianLinks is the student-guardian relationship with its parents-portal
// permissions (#2756) as a row source the inbox joins in its own statement.
// The roots bind it to the guardian-link projection.
type GuardianLinks interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
}
