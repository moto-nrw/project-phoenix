package application

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// guardianEnrollments keeps the optional enrollment claim out of the flows:
// a composition without it reports nothing to claim.
func GuardianEnrollments(source identityPorts.GuardianEnrollments) ports.GuardianEnrollments {
	if source == nil {
		return nil
	}
	return GuardianEnrollmentClaims{Source: source}
}
