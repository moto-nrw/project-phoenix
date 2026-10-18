package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NewPortalDeclarationPolicy() communication.PortalDeclarationPolicy {
	return portalDeclarationPolicy{}
}

type portalDeclarationPolicy struct {
	communication.DeclarationPolicy
}

func (portalDeclarationPolicy) PortalCanonicalRecord(value communication.PortalDeclarationSubmission) []byte {
	return values.CanonicalDeclarationRecord((*values.DeclarationSubmission)(&value))
}
func (portalDeclarationPolicy) PortalCanonicalContent(value communication.PortalDeclarationVersion) []byte {
	return values.CanonicalDeclarationContent((*values.DeclarationVersion)(&value))
}
