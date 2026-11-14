package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/application"
	internalports "github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

var _ identityaccess.AccountMFA = application.ComposedAccountMFA{}

var _ internalports.MFAGate = application.CapabilityGate{}
