package rfid

import "net/http"

// AssignRFIDTagHandler returns the assignStaffRFIDTag handler.
func (rs *Resource) AssignRFIDTagHandler() http.HandlerFunc { return rs.assignStaffRFIDTag }

// UnassignRFIDTagHandler returns the unassignStaffRFIDTag handler.
func (rs *Resource) UnassignRFIDTagHandler() http.HandlerFunc { return rs.unassignStaffRFIDTag }
