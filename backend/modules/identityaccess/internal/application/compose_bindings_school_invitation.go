package application

import (
	"log/slog"

	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// invitationLogger is the logger the always-composed invitation maintenance
// uses; compositions without the invitation seams fall back to the default.
func InvitationLogger(deps *identityBindingPorts.SchoolInvitationDependencies) *slog.Logger {
	if deps == nil {
		return slog.Default()
	}
	return deps.Logger
}
