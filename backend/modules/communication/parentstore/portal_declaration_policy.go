package parentstore

import (
	"github.com/moto-ogs/project-phoenix/modules/communication"
	"github.com/moto-ogs/project-phoenix/modules/communication/internal/application"
)

func NewPortalDeclarationPolicy() communication.PortalDeclarationPolicy {
	return application.NewPortalDeclarationPolicy()
}
