package parentstore

import "github.com/moto-nrw/project-phoenix/modules/communication"

func NewPortalDeclarationPolicy() communication.PortalDeclarationPolicy {
	return portalDeclarationPolicy{}
}

type portalDeclarationPolicy struct {
	communication.DeclarationPolicy
}

func (portalDeclarationPolicy) PortalCanonicalRecord(value communication.PortalDeclarationSubmission) []byte {
	row := portalToRetainedDeclarationSubmission(value)
	return row.CanonicalRecord()
}
func (portalDeclarationPolicy) PortalCanonicalContent(value communication.PortalDeclarationVersion) []byte {
	row := portalToRetainedDeclarationVersion(value)
	return row.CanonicalContent()
}
