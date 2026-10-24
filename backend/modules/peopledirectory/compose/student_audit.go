package compose

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"

// StudentFieldAuditLog is the Audit Platform seam behind the per-child change
// history. The composition root binds it: Audit Platform owns the trail and
// its retention, People Directory only decides which of its fields are
// tracked and how a change reads.
type StudentFieldAuditLog interface {
	application.StudentFieldAuditTrail
}
