package active

import "net/http"

// ============================================================================
// EXPORTED HANDLER METHODS (for testing)
// ============================================================================

// Active Group Handlers
func (rs *Resource) ListActiveGroupsHandler() http.HandlerFunc { return rs.listActiveGroups }
func (rs *Resource) GetActiveGroupHandler() http.HandlerFunc   { return rs.getActiveGroup }
func (rs *Resource) CreateActiveGroupHandler() http.HandlerFunc {
	return rs.createActiveGroup
}
func (rs *Resource) UpdateActiveGroupHandler() http.HandlerFunc {
	return rs.updateActiveGroup
}
func (rs *Resource) DeleteActiveGroupHandler() http.HandlerFunc {
	return rs.deleteActiveGroup
}
func (rs *Resource) EndActiveGroupHandler() http.HandlerFunc { return rs.endActiveGroup }

// Visit Handlers
func (rs *Resource) ListVisitsHandler() http.HandlerFunc  { return rs.listVisits }
func (rs *Resource) GetVisitHandler() http.HandlerFunc    { return rs.getVisit }
func (rs *Resource) CreateVisitHandler() http.HandlerFunc { return rs.createVisit }
func (rs *Resource) UpdateVisitHandler() http.HandlerFunc { return rs.updateVisit }
func (rs *Resource) DeleteVisitHandler() http.HandlerFunc { return rs.deleteVisit }
func (rs *Resource) EndVisitHandler() http.HandlerFunc    { return rs.endVisit }
func (rs *Resource) GetStudentVisitsHandler() http.HandlerFunc {
	return rs.getStudentVisits
}
func (rs *Resource) GetStudentCurrentVisitHandler() http.HandlerFunc {
	return rs.getStudentCurrentVisit
}

// Supervisor Handlers
func (rs *Resource) ListSupervisorsHandler() http.HandlerFunc  { return rs.listSupervisors }
func (rs *Resource) GetSupervisorHandler() http.HandlerFunc    { return rs.getSupervisor }
func (rs *Resource) CreateSupervisorHandler() http.HandlerFunc { return rs.createSupervisor }
func (rs *Resource) UpdateSupervisorHandler() http.HandlerFunc { return rs.updateSupervisor }
func (rs *Resource) DeleteSupervisorHandler() http.HandlerFunc { return rs.deleteSupervisor }
func (rs *Resource) EndSupervisionHandler() http.HandlerFunc   { return rs.endSupervision }
func (rs *Resource) GetStaffSupervisionsHandler() http.HandlerFunc {
	return rs.getStaffSupervisions
}
func (rs *Resource) GetStaffActiveSupervisionsHandler() http.HandlerFunc {
	return rs.getStaffActiveSupervisions
}

// Analytics Handlers
func (rs *Resource) GetCountsHandler() http.HandlerFunc { return rs.getCounts }
func (rs *Resource) GetDashboardAnalyticsHandler() http.HandlerFunc {
	return rs.getDashboardAnalytics
}

func (rs *Resource) GetRoomUtilizationHandler() http.HandlerFunc { return rs.getRoomUtilization }
func (rs *Resource) GetStudentAttendanceHandler() http.HandlerFunc {
	return rs.getStudentAttendance
}

// Combined Group Handlers
func (rs *Resource) ListCombinedGroupsHandler() http.HandlerFunc  { return rs.listCombinedGroups }
func (rs *Resource) GetCombinedGroupHandler() http.HandlerFunc    { return rs.getCombinedGroup }
func (rs *Resource) CreateCombinedGroupHandler() http.HandlerFunc { return rs.createCombinedGroup }
func (rs *Resource) UpdateCombinedGroupHandler() http.HandlerFunc { return rs.updateCombinedGroup }
func (rs *Resource) DeleteCombinedGroupHandler() http.HandlerFunc { return rs.deleteCombinedGroup }
func (rs *Resource) EndCombinedGroupHandler() http.HandlerFunc    { return rs.endCombinedGroup }
func (rs *Resource) GetActiveCombinedGroupsHandler() http.HandlerFunc {
	return rs.getActiveCombinedGroups
}

// Group by filters Handlers
func (rs *Resource) GetActiveGroupsByRoomHandler() http.HandlerFunc {
	return rs.getActiveGroupsByRoom
}
func (rs *Resource) GetActiveGroupsByGroupHandler() http.HandlerFunc {
	return rs.getActiveGroupsByGroup
}
func (rs *Resource) GetActiveGroupVisitsHandler() http.HandlerFunc {
	return rs.getActiveGroupVisits
}
func (rs *Resource) GetActiveGroupVisitsWithDisplayHandler() http.HandlerFunc {
	return rs.getActiveGroupVisitsWithDisplay
}
func (rs *Resource) GetActiveGroupSupervisorsHandler() http.HandlerFunc {
	return rs.getActiveGroupSupervisors
}
func (rs *Resource) GetVisitsByGroupHandler() http.HandlerFunc {
	return rs.getVisitsByGroup
}
func (rs *Resource) GetSupervisorsByGroupHandler() http.HandlerFunc {
	return rs.getSupervisorsByGroup
}

// Group Mapping Handlers
func (rs *Resource) GetGroupMappingsHandler() http.HandlerFunc { return rs.getGroupMappings }
func (rs *Resource) GetCombinedGroupMappingsHandler() http.HandlerFunc {
	return rs.getCombinedGroupMappings
}
func (rs *Resource) AddGroupToCombinationHandler() http.HandlerFunc {
	return rs.addGroupToCombination
}
func (rs *Resource) RemoveGroupFromCombinationHandler() http.HandlerFunc {
	return rs.removeGroupFromCombination
}

// Unclaimed Group Handlers
func (rs *Resource) ListUnclaimedGroupsHandler() http.HandlerFunc { return rs.listUnclaimedGroups }
func (rs *Resource) ClaimGroupHandler() http.HandlerFunc          { return rs.claimGroup }

// Checkout Handler
func (rs *Resource) CheckoutStudentHandler() http.HandlerFunc { return rs.checkoutStudent }
