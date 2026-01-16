package staff

import "net/http"

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// ListStaffHandler returns the listStaff handler for testing.
func (rs *Resource) ListStaffHandler() http.HandlerFunc { return rs.listStaff }

// GetStaffHandler returns the getStaff handler for testing.
func (rs *Resource) GetStaffHandler() http.HandlerFunc { return rs.getStaff }

// CreateStaffHandler returns the createStaff handler for testing.
func (rs *Resource) CreateStaffHandler() http.HandlerFunc { return rs.createStaff }

// UpdateStaffHandler returns the updateStaff handler for testing.
func (rs *Resource) UpdateStaffHandler() http.HandlerFunc { return rs.updateStaff }

// DeleteStaffHandler returns the deleteStaff handler for testing.
func (rs *Resource) DeleteStaffHandler() http.HandlerFunc { return rs.deleteStaff }

// GetStaffGroupsHandler returns the getStaffGroups handler for testing.
func (rs *Resource) GetStaffGroupsHandler() http.HandlerFunc { return rs.getStaffGroups }

// GetStaffSubstitutionsHandler returns the getStaffSubstitutions handler for testing.
func (rs *Resource) GetStaffSubstitutionsHandler() http.HandlerFunc { return rs.getStaffSubstitutions }

// GetAvailableStaffHandler returns the getAvailableStaff handler for testing.
func (rs *Resource) GetAvailableStaffHandler() http.HandlerFunc { return rs.getAvailableStaff }

// GetAvailableForSubstitutionHandler returns the getAvailableForSubstitution handler for testing.
func (rs *Resource) GetAvailableForSubstitutionHandler() http.HandlerFunc {
	return rs.getAvailableForSubstitution
}

// GetStaffByRoleHandler returns the getStaffByRole handler for testing.
func (rs *Resource) GetStaffByRoleHandler() http.HandlerFunc { return rs.getStaffByRole }

// GetPINStatusHandler returns the getPINStatus handler for testing.
func (rs *Resource) GetPINStatusHandler() http.HandlerFunc { return rs.getPINStatus }

// UpdatePINHandler returns the updatePIN handler for testing.
func (rs *Resource) UpdatePINHandler() http.HandlerFunc { return rs.updatePIN }
