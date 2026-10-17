package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"

	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

var _ identityPorts.AccountMFARecords = application.PublicAccountMFARecords{}
