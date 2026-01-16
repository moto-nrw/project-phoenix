package guardians

import "net/http"

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// ListGuardiansHandler returns the list guardians handler
func (rs *Resource) ListGuardiansHandler() http.HandlerFunc { return rs.listGuardians }

// GetGuardianHandler returns the get guardian handler
func (rs *Resource) GetGuardianHandler() http.HandlerFunc { return rs.getGuardian }

// CreateGuardianHandler returns the create guardian handler
func (rs *Resource) CreateGuardianHandler() http.HandlerFunc { return rs.createGuardian }

// UpdateGuardianHandler returns the update guardian handler
func (rs *Resource) UpdateGuardianHandler() http.HandlerFunc { return rs.updateGuardian }

// DeleteGuardianHandler returns the delete guardian handler
func (rs *Resource) DeleteGuardianHandler() http.HandlerFunc { return rs.deleteGuardian }

// ListGuardiansWithoutAccountHandler returns the list guardians without account handler
func (rs *Resource) ListGuardiansWithoutAccountHandler() http.HandlerFunc {
	return rs.listGuardiansWithoutAccount
}

// ListInvitableGuardiansHandler returns the list invitable guardians handler
func (rs *Resource) ListInvitableGuardiansHandler() http.HandlerFunc {
	return rs.listInvitableGuardians
}

// SendInvitationHandler returns the send invitation handler
func (rs *Resource) SendInvitationHandler() http.HandlerFunc { return rs.sendInvitation }

// ListPendingInvitationsHandler returns the list pending invitations handler
func (rs *Resource) ListPendingInvitationsHandler() http.HandlerFunc {
	return rs.listPendingInvitations
}

// GetStudentGuardiansHandler returns the get student guardians handler
func (rs *Resource) GetStudentGuardiansHandler() http.HandlerFunc { return rs.getStudentGuardians }

// GetGuardianStudentsHandler returns the get guardian students handler
func (rs *Resource) GetGuardianStudentsHandler() http.HandlerFunc { return rs.getGuardianStudents }

// LinkGuardianToStudentHandler returns the link guardian to student handler
func (rs *Resource) LinkGuardianToStudentHandler() http.HandlerFunc { return rs.linkGuardianToStudent }

// UpdateStudentGuardianRelationshipHandler returns the update relationship handler
func (rs *Resource) UpdateStudentGuardianRelationshipHandler() http.HandlerFunc {
	return rs.updateStudentGuardianRelationship
}

// RemoveGuardianFromStudentHandler returns the remove guardian from student handler
func (rs *Resource) RemoveGuardianFromStudentHandler() http.HandlerFunc {
	return rs.removeGuardianFromStudent
}

// ValidateGuardianInvitationHandler returns the validate invitation handler
func (rs *Resource) ValidateGuardianInvitationHandler() http.HandlerFunc {
	return rs.validateGuardianInvitation
}

// AcceptGuardianInvitationHandler returns the accept invitation handler
func (rs *Resource) AcceptGuardianInvitationHandler() http.HandlerFunc {
	return rs.acceptGuardianInvitation
}
