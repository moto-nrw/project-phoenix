package groups

import "net/http"

// =============================================================================
// EXPORTED HANDLER METHODS FOR TESTING
// =============================================================================

// ListGroupsHandler returns the listGroups handler for testing
func (rs *Resource) ListGroupsHandler() http.HandlerFunc { return rs.listGroups }

// GetGroupHandler returns the getGroup handler for testing
func (rs *Resource) GetGroupHandler() http.HandlerFunc { return rs.getGroup }

// CreateGroupHandler returns the createGroup handler for testing
func (rs *Resource) CreateGroupHandler() http.HandlerFunc { return rs.createGroup }

// UpdateGroupHandler returns the updateGroup handler for testing
func (rs *Resource) UpdateGroupHandler() http.HandlerFunc { return rs.updateGroup }

// DeleteGroupHandler returns the deleteGroup handler for testing
func (rs *Resource) DeleteGroupHandler() http.HandlerFunc { return rs.deleteGroup }

// GetGroupStudentsHandler returns the getGroupStudents handler for testing
func (rs *Resource) GetGroupStudentsHandler() http.HandlerFunc { return rs.getGroupStudents }

// GetGroupSupervisorsHandler returns the getGroupSupervisors handler for testing
func (rs *Resource) GetGroupSupervisorsHandler() http.HandlerFunc { return rs.getGroupSupervisors }

// GetGroupStudentsRoomStatusHandler returns the getGroupStudentsRoomStatus handler for testing
func (rs *Resource) GetGroupStudentsRoomStatusHandler() http.HandlerFunc {
	return rs.getGroupStudentsRoomStatus
}

// GetGroupSubstitutionsHandler returns the getGroupSubstitutions handler for testing
func (rs *Resource) GetGroupSubstitutionsHandler() http.HandlerFunc { return rs.getGroupSubstitutions }

// TransferGroupHandler returns the transferGroup handler for testing
func (rs *Resource) TransferGroupHandler() http.HandlerFunc { return rs.transferGroup }

// CancelSpecificTransferHandler returns the cancelSpecificTransfer handler for testing
func (rs *Resource) CancelSpecificTransferHandler() http.HandlerFunc {
	return rs.cancelSpecificTransfer
}
