package application

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func NewPortalDeclarationPolicy() communication.PortalDeclarationPolicy {
	return portalDeclarationPolicy{}
}

type portalDeclarationPolicy struct {
	communication.DeclarationPolicy
}

func (portalDeclarationPolicy) PortalCanonicalRecord(value communication.PortalDeclarationSubmission) []byte {
	return CanonicalDeclarationRecord((*DeclarationSubmission)(&value))
}
func (portalDeclarationPolicy) PortalCanonicalContent(value communication.PortalDeclarationVersion) []byte {
	return CanonicalDeclarationContent((*DeclarationVersion)(&value))
}
