package compose

import (
	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// The guardian service's consumer-owned ports (#3755). The audit trail of
// the guardian bank data belongs to the Audit Platform and the relationship
// role presets to the Security Runtime; this package may name neither, so the
// composition root binds both ports to their owners.

// GuardianRolePresets is the Security Runtime's role-preset policy of a
// student-guardian relationship.
type GuardianRolePresets interface {
	internalports.GuardianRolePresets
}
